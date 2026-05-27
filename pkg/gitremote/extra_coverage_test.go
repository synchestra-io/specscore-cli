package gitremote

import (
	"os/exec"
	"testing"
)

// ---------------------------------------------------------------------------
// gitremote.go — line 69: HeadSHA success path with a real commit
// The existing TestHeadSHA uses `git commit` without commit.gpgsign=false
// which can fail in CI. This test explicitly disables gpg signing.
// ---------------------------------------------------------------------------

func TestHeadSHA_SuccessPath(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "T")
	runGit(t, dir, "config", "commit.gpgsign", "false")
	runGit(t, dir, "commit", "--allow-empty", "-q", "-m", "initial")

	sha, err := HeadSHA(dir)
	if err != nil {
		t.Fatalf("HeadSHA returned error: %v", err)
	}
	if len(sha) != 40 {
		t.Errorf("HeadSHA length = %d; want 40 hex chars (got %q)", len(sha), sha)
	}
}
