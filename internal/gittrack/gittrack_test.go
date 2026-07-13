package gittrack

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func mustGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...) //nolint:gosec // G204: test-local git invocation under t.TempDir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestTracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "committed.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustGit(t, root, "init", "-q")
	mustGit(t, root, "add", "committed.txt")
	mustGit(t, root, "-c", "user.email=t@t", "-c", "user.name=t", "commit", "-qm", "seed")

	tracked, err := Tracked(root)
	if err != nil {
		t.Fatalf("Tracked: %v", err)
	}
	if !tracked["committed.txt"] {
		t.Error("committed.txt should be tracked")
	}
	if tracked["untracked.txt"] {
		t.Error("untracked.txt must not be tracked")
	}
}

func TestTrackedNotARepo(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	_, err := Tracked(t.TempDir())
	if err == nil || err != ErrNotARepo {
		t.Fatalf("expected ErrNotARepo, got %v", err)
	}
}
