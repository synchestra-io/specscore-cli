package main

import (
	"os"
	"os/exec"
	"testing"
)

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
