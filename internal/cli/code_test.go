package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/specscore/specscore-cli/pkg/exitcode"
)

func runCode(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := codeCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestCodeDeps_InvalidType(t *testing.T) {
	_, _, err := runCode(t, "deps", "--type=banana")
	if err == nil {
		t.Fatal("expected error for invalid --type, got nil")
	}
	if got := exitCodeOf(err); got != exitcode.InvalidArgs {
		t.Fatalf("exit code = %d, want %d (InvalidArgs)", got, exitcode.InvalidArgs)
	}
	if !strings.Contains(err.Error(), "invalid --type") {
		t.Errorf("error should mention 'invalid --type', got: %q", err.Error())
	}
}

func TestCodeDeps_NoFiles(t *testing.T) {
	// Use a pattern that won't match anything in a temp dir.
	tmp := t.TempDir()
	withCwd(t, tmp)
	out, _, err := runCode(t, "deps", "--path=nonexistent_pattern_xyz_*.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "" {
		t.Errorf("expected empty output for no matching files, got: %q", out)
	}
}

func TestCodeDeps_WithAnnotations(t *testing.T) {
	tmp := t.TempDir()
	// Create a Go file with a specscore: annotation.
	goFile := filepath.Join(tmp, "main.go")
	content := `package main

// specscore:feature/auth
func main() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}
	withCwd(t, tmp)
	out, _, err := runCode(t, "deps", "--path=main.go")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "spec/features/auth") {
		t.Errorf("expected output to contain 'spec/features/auth', got: %q", out)
	}
}

func TestCodeDeps_TypeFilter(t *testing.T) {
	tmp := t.TempDir()
	goFile := filepath.Join(tmp, "svc.go")
	content := `package svc

// specscore:feature/payments
// specscore:plan/rollout
func process() {}
`
	if err := os.WriteFile(goFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write go file: %v", err)
	}
	withCwd(t, tmp)

	// Filter to only features.
	out, _, err := runCode(t, "deps", "--path=svc.go", "--type=feature")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(out, "spec/features/payments") {
		t.Errorf("expected feature ref in output, got: %q", out)
	}
	if strings.Contains(out, "spec/plans/rollout") {
		t.Errorf("plan ref should be filtered out, got: %q", out)
	}
}
