package gitremote

import (
	"os/exec"
	"testing"
)

func TestOriginURL(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "t@example.com")
	runGit(t, dir, "config", "user.name", "T")
	runGit(t, dir, "remote", "add", "origin", "https://github.com/specscore/specscore-cli.git")

	got, err := OriginURL(dir)
	if err != nil {
		t.Fatalf("OriginURL returned error: %v", err)
	}
	want := "https://github.com/specscore/specscore-cli.git"
	if got != want {
		t.Errorf("OriginURL = %q; want %q", got, want)
	}
}

func TestOriginURL_NoGitRepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	dir := t.TempDir()
	if _, err := OriginURL(dir); err == nil {
		t.Fatal("OriginURL on non-git dir returned nil error; want error")
	}
}

func TestOriginURL_NoOriginRemote(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available on PATH")
	}

	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")

	if _, err := OriginURL(dir); err == nil {
		t.Fatal("OriginURL on repo without origin returned nil error; want error")
	}
}
