package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/specscore/specscore-cli/pkg/exitcode"
)

// --- Fatal ---

func TestFatal_NilIsNoop(t *testing.T) {
	var called bool
	old := osExit
	osExit = func(code int) { called = true }
	t.Cleanup(func() { osExit = old })

	Fatal(nil)
	if called {
		t.Error("Fatal(nil) should not call osExit")
	}
}

func TestFatal_TypedExitCode(t *testing.T) {
	var gotCode int
	old := osExit
	osExit = func(code int) { gotCode = code }
	t.Cleanup(func() { osExit = old })

	// Redirect stderr to capture the error message.
	origStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr })

	Fatal(exitcode.NotFoundErrorf("missing thing"))
	_ = w.Close()
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if gotCode != exitcode.NotFound {
		t.Errorf("exit code = %d, want %d (NotFound)", gotCode, exitcode.NotFound)
	}
	if !strings.Contains(buf.String(), "missing thing") {
		t.Errorf("stderr = %q, want to contain 'missing thing'", buf.String())
	}
}

func TestFatal_GenericError(t *testing.T) {
	var gotCode int
	old := osExit
	osExit = func(code int) { gotCode = code }
	t.Cleanup(func() { osExit = old })

	origStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr; _ = w.Close() })

	Fatal(exitcode.InvalidArgsErrorf("bad args"))
	if gotCode != exitcode.InvalidArgs {
		t.Errorf("exit code = %d, want %d (InvalidArgs)", gotCode, exitcode.InvalidArgs)
	}
}

func TestFatal_PlainError(t *testing.T) {
	var gotCode int
	old := osExit
	osExit = func(code int) { gotCode = code }
	t.Cleanup(func() { osExit = old })

	origStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w
	t.Cleanup(func() { os.Stderr = origStderr; _ = w.Close() })

	Fatal(os.ErrNotExist)
	if gotCode != 1 {
		t.Errorf("exit code = %d, want 1 for plain error", gotCode)
	}
}

// --- Run ---

func TestRun_HelpOutput(t *testing.T) {
	// Run with no args should print help and return nil.
	err := Run([]string{"specscore"})
	if err != nil {
		t.Errorf("Run([specscore]) = %v, want nil", err)
	}
}

func TestRun_VersionFlag(t *testing.T) {
	err := Run([]string{"specscore", "--version"})
	if err != nil {
		t.Errorf("Run([--version]) = %v, want nil", err)
	}
}

func TestRun_VersionSubcommand(t *testing.T) {
	err := Run([]string{"specscore", "version"})
	if err != nil {
		t.Errorf("Run([version]) = %v, want nil", err)
	}
}

func TestRun_UnknownCommand(t *testing.T) {
	err := Run([]string{"specscore", "nonexistent-command"})
	if err == nil {
		t.Error("Run([nonexistent-command]) = nil, want error")
	}
}

// --- versionCommand ---

func TestVersionCommand_Output(t *testing.T) {
	cmd := versionCommand()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.Run(cmd, nil)
	if !strings.Contains(out.String(), "specscore") {
		t.Errorf("output = %q, want to contain 'specscore'", out.String())
	}
}
