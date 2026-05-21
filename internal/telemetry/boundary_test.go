package telemetry_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// forbiddenImports are the vendor telemetry SDK import paths that MUST only
// appear inside internal/telemetry/ per cli/telemetry#req:vendor-sdk-import-
// confinement. Adding a new forbidden import here is the canonical way to
// extend the privacy audit surface when integrating a new telemetry vendor.
var forbiddenImports = map[string]string{
	"github.com/posthog/posthog-go":     "PostHog Go SDK — use internal/telemetry/usage.go",
	"github.com/getsentry/sentry-go":    "Sentry Go SDK — use internal/telemetry/errors.go",
	"github.com/segmentio/analytics-go": "Segment SDK — not used in MVP; if added, route through internal/telemetry",
}

// allowedInsidePath is the directory prefix (relative to the repo root) where
// forbidden imports MAY appear. Any path with this prefix is exempt; every
// other Go file in the repo is checked.
const allowedInsidePath = "internal/telemetry/"

// TestVendorSDKImportConfinement walks every .go file in the repository (except
// the allowed-inside path) and fails the build if any file imports a member of
// forbiddenImports. Implements cli/telemetry#ac:vendor-sdk-import-confinement-
// enforced via the `_test.go` form permitted by REQ:vendor-sdk-import-
// confinement.
//
// The test runs as part of `go test ./...` and produces a clear stderr line
// naming the offending file, the forbidden import, and the rule that forbids
// it.
func TestVendorSDKImportConfinement(t *testing.T) {
	repoRoot := findRepoRoot(t)
	fset := token.NewFileSet()
	walkErr := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip vendor, hidden dirs, and the allowed path.
			name := d.Name()
			if name == "vendor" || strings.HasPrefix(name, ".") || name == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, relErr := filepath.Rel(repoRoot, path)
		if relErr != nil {
			return relErr
		}
		// Files under internal/telemetry/ MAY import vendor SDKs.
		if strings.HasPrefix(filepath.ToSlash(rel), allowedInsidePath) {
			return nil
		}
		// Parse imports only — faster than parsing the whole file.
		file, parseErr := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if parseErr != nil {
			// A file we can't parse is not our concern here; let the regular
			// build catch it.
			return nil
		}
		for _, imp := range file.Imports {
			if imp.Path == nil {
				continue
			}
			// imp.Path.Value is the quoted import path, e.g. `"fmt"`.
			value := strings.Trim(imp.Path.Value, `"`)
			if reason, forbidden := forbiddenImports[value]; forbidden {
				t.Errorf(
					"vendor-sdk-import-confinement: %s imports %q; "+
						"only files under %s may import telemetry vendor SDKs (%s)",
					rel, value, allowedInsidePath, reason,
				)
			}
		}
		return nil
	})
	if walkErr != nil {
		t.Fatalf("walking repo root: %v", walkErr)
	}
}

// TestForbiddenImportsDetectsSyntheticViolation feeds a synthetic Go source
// containing a forbidden import into the same parser-based check used by
// TestVendorSDKImportConfinement and asserts the violation is detected. This
// guards against bit-rot in the detection logic itself: if the production
// check stops working (e.g. parser changes silently), this fixture-driven test
// will fail.
func TestForbiddenImportsDetectsSyntheticViolation(t *testing.T) {
	const syntheticSource = `package main

import (
	"fmt"
	"github.com/posthog/posthog-go"
)

func main() {
	fmt.Println(posthog.NewClient(""))
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "synthetic.go", syntheticSource, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing synthetic source: %v", err)
	}
	var found bool
	for _, imp := range file.Imports {
		value := strings.Trim(imp.Path.Value, `"`)
		if _, forbidden := forbiddenImports[value]; forbidden {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("synthetic violation NOT detected — forbiddenImports check is broken")
	}
}

// findRepoRoot walks up from the test's working directory looking for go.mod.
// Returns the directory containing go.mod or fails the test.
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("abs cwd: %v", err)
	}
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not find go.mod walking up from %s", dir)
		}
		dir = parent
	}
}

