package lifecycle

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
)

// Testable indirections for OS operations. Tests inject failures via these.
var (
	osCreateTemp = os.CreateTemp
	ioCopy       = io.Copy
	osChmod      = os.Chmod
	osRename     = os.Rename
)

// statusLineRe matches a header line of the form `**Status:** <value>`,
// allowing leading horizontal whitespace before the `**` marker and tolerating
// trailing whitespace after the value. The matcher operates on a single line
// (caller has already split on newlines).
//
// Capture groups:
//
//	[1] indent (any leading horizontal whitespace before "**Status:**")
//	[2] value (the status text, without surrounding whitespace)
//	[3] trailing (any trailing horizontal whitespace AND optional CR)
//
// Note the explicit `\r?` in the trailing group: when a file uses CRLF line
// endings, Rewrite MUST preserve the carriage return byte (REQ:
// status-line-rewrite). The line-splitter below also preserves the original
// terminator on each line, so Rewrite reassembles the file byte-for-byte
// outside of the single mutated value.
var statusLineRe = regexp.MustCompile(`^([ \t]*)\*\*Status:\*\*[ \t]+([^\r\n]*?)([ \t]*\r?)$`)

// ErrStatusLineNotFound is returned by Validate/Rewrite when the artifact
// does not contain a recognizable `**Status:**` line.
var ErrStatusLineNotFound = errors.New("lifecycle: artifact has no **Status:** line")

// Validate reads artifactPath, extracts its current Status, and checks that
// the (kind, current, to) transition is legal. It does NOT mutate the file.
//
// It returns the from status on success. On failure it returns one of:
//
//   - an os error if the file cannot be opened or read
//   - ErrStatusLineNotFound if the file has no recognizable **Status:** line
//   - an *InvalidTransitionError if the transition is illegal in kind's matrix
//
// Validate is the primitive that the CLI verb runs FIRST, before any
// mutation; it is the single check that guarantees REQ:
// state-machine-strictness.
func Validate(kind Kind, artifactPath string, to Status) (Status, error) {
	from, err := readStatus(artifactPath)
	if err != nil {
		return "", err
	}
	if err := Transition(kind, from, to); err != nil {
		return from, err
	}
	return from, nil
}

// Rewrite mutates the artifact's `**Status:**` line in place, replacing only
// the value text. Every other byte of the file (line ordering, indentation,
// line endings, trailing whitespace) is preserved (REQ: status-line-rewrite).
//
// The returned string is the ORIGINAL line content (including any line
// terminator that was attached to it), suitable for passing to Rollback to
// undo the mutation. The caller is responsible for retaining this value
// until index sync is confirmed successful.
//
// If the file has no `**Status:**` line, Rewrite returns ErrStatusLineNotFound
// and the file is left untouched.
func Rewrite(artifactPath string, newStatus Status) (string, error) {
	original, err := os.ReadFile(artifactPath)
	if err != nil {
		return "", err
	}
	lines := splitKeepTerminators(original)

	idx := findStatusLineIndex(lines)
	if idx < 0 {
		return "", ErrStatusLineNotFound
	}

	originalLine := lines[idx]
	body, terminator := splitTerminator(originalLine)
	m := statusLineRe.FindStringSubmatch(body)
	if m == nil {
		// Shouldn't happen because findStatusLineIndex matched, but be defensive.
		return "", ErrStatusLineNotFound
	}
	indent := m[1]
	trailing := m[3]
	newBody := fmt.Sprintf("%s**Status:** %s%s", indent, string(newStatus), trailing)
	lines[idx] = newBody + terminator

	if err := writeFileAtomic(artifactPath, joinLines(lines)); err != nil {
		return "", err
	}
	return originalLine, nil
}

// Rollback restores the artifact's `**Status:**` line to its
// pre-Rewrite content, identified by the originalStatusLine returned from
// the prior Rewrite call.
//
// Rollback locates the file's current `**Status:**` line (which is now the
// MUTATED value), replaces that single line with originalStatusLine, and
// writes the file back. After Rollback returns nil, the file content is
// byte-identical to its pre-Rewrite state.
//
// If the file has been mutated externally between Rewrite and Rollback such
// that no `**Status:**` line remains, Rollback returns ErrStatusLineNotFound.
// Concurrent modification is outside the contract (REQ: no-coordination in
// the lifecycle-transitions Meta spec).
func Rollback(artifactPath string, originalStatusLine string) error {
	current, err := os.ReadFile(artifactPath)
	if err != nil {
		return err
	}
	lines := splitKeepTerminators(current)
	idx := findStatusLineIndex(lines)
	if idx < 0 {
		return ErrStatusLineNotFound
	}
	lines[idx] = originalStatusLine
	return writeFileAtomic(artifactPath, joinLines(lines))
}

// readStatus opens the file and returns the value text of the first
// `**Status:**` line. Returns ErrStatusLineNotFound if no such line is
// present.
func readStatus(artifactPath string) (Status, error) {
	f, err := os.Open(artifactPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if m := statusLineRe.FindStringSubmatch(line); m != nil {
			return Status(strings.TrimSpace(m[2])), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", ErrStatusLineNotFound
}

// findStatusLineIndex returns the index in lines of the first line whose
// body (terminator stripped) matches statusLineRe, or -1 if no such line
// exists.
func findStatusLineIndex(lines []string) int {
	for i, ln := range lines {
		body, _ := splitTerminator(ln)
		if statusLineRe.MatchString(body) {
			return i
		}
	}
	return -1
}

// splitKeepTerminators splits content on '\n' boundaries, retaining the
// terminator on each line so that joinLines round-trips byte-for-byte.
//
// Trailing data without a final newline is captured as a final element with
// no terminator. An empty input returns an empty slice (joinLines will then
// produce empty output).
func splitKeepTerminators(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	var lines []string
	start := 0
	for i := 0; i < len(content); i++ {
		if content[i] == '\n' {
			lines = append(lines, string(content[start:i+1]))
			start = i + 1
		}
	}
	if start < len(content) {
		lines = append(lines, string(content[start:]))
	}
	return lines
}

// splitTerminator splits a line into (body, terminator). The terminator is
// "" for the (possibly only) line with no trailing newline; otherwise it is
// "\n" or "\r\n".
func splitTerminator(line string) (string, string) {
	if strings.HasSuffix(line, "\r\n") {
		return line[:len(line)-2], "\r\n"
	}
	if strings.HasSuffix(line, "\n") {
		return line[:len(line)-1], "\n"
	}
	return line, ""
}

// joinLines concatenates lines (each of which retains its own terminator)
// into a single byte slice.
func joinLines(lines []string) []byte {
	var buf bytes.Buffer
	for _, ln := range lines {
		buf.WriteString(ln)
	}
	return buf.Bytes()
}

// writeFileAtomic writes content to dst via a same-directory temp file +
// rename, so a partial write cannot leave a half-rewritten artifact on
// disk. File mode is preserved from the existing dst.
func writeFileAtomic(dst string, content []byte) error {
	stat, err := os.Stat(dst)
	if err != nil {
		return err
	}
	tmp, err := osCreateTemp(dirOf(dst), ".lifecycle-rewrite-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := func() {
		_ = os.Remove(tmpName)
	}
	if _, err := ioCopy(tmp, bytes.NewReader(content)); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		cleanup()
		return err
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return err
	}
	if err := osChmod(tmpName, stat.Mode().Perm()); err != nil {
		cleanup()
		return err
	}
	if err := osRename(tmpName, dst); err != nil {
		cleanup()
		return err
	}
	return nil
}

// dirOf returns the directory portion of path. Uses a tiny local
// implementation rather than importing path/filepath to keep this file
// dependency-light and consistent in style with the rest of the package.
func dirOf(p string) string {
	for i := len(p) - 1; i >= 0; i-- {
		if p[i] == '/' || p[i] == '\\' {
			if i == 0 {
				return p[:1]
			}
			return p[:i]
		}
	}
	return "."
}
