package main

import (
	"errors"
	"os"
	"os/exec"
	"testing"
)

// ---------- in-process tests (provide coverage) ----------

func TestRun_Success(t *testing.T) {
	called := false
	runFn := func([]string) error { called = true; return nil }
	fatalFn := func(error) { t.Fatal("fatalFn should not be called on success") }

	run([]string{"specscore", "--help"}, runFn, fatalFn)
	if !called {
		t.Error("runFn was not called")
	}
}

func TestRun_Error(t *testing.T) {
	wantErr := errors.New("boom")
	var gotErr error
	runFn := func([]string) error { return wantErr }
	fatalFn := func(err error) { gotErr = err }

	run([]string{"specscore", "bad"}, runFn, fatalFn)
	if gotErr != wantErr {
		t.Errorf("fatalFn got %v, want %v", gotErr, wantErr)
	}
}

func TestMain_CallsRun(t *testing.T) {
	orig := os.Args
	defer func() { os.Args = orig }()
	os.Args = []string{"specscore", "--help"}
	main() // exercises the real main → run wiring
}

// ---------- subprocess tests (validate real binary behavior) ----------

func TestMain_HelpExitsZero(t *testing.T) {
	// Build the binary and run it with --help to cover the main() function.
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--help")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		t.Fatalf("specscore --help exited non-zero: %v", err)
	}
}

func TestMain_VersionExitsZero(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "--version")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("specscore --version exited non-zero: %v", err)
	}
	if len(out) == 0 {
		t.Error("--version produced no output")
	}
}

func TestMain_UnknownCommandExitsNonZero(t *testing.T) {
	bin := buildBinary(t)
	cmd := exec.Command(bin, "nonexistent-command-xyz")
	err := cmd.Run()
	if err == nil {
		t.Error("expected non-zero exit for unknown command")
	}
}

func buildBinary(t *testing.T) string {
	t.Helper()
	bin := t.TempDir() + "/specscore-test"
	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = "."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}
