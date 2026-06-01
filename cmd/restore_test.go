package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/identity"
)

func TestPackRestoreRoundTrip(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=roundtrip")

	// Simulate a fresh clone: the local file is gone.
	if err := os.Remove(filepath.Join(root, ".env.local")); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doRestore(root, p, false, false, &out); err != nil {
		t.Fatalf("doRestore: %v", err)
	}
	assertRepoContent(t, root, ".env.local", "TOKEN=roundtrip")
	if !strings.Contains(out.String(), "1 written") {
		t.Errorf("output = %q, want '1 written'", out.String())
	}
}

func TestDoRestoreConflictRequiresForce(t *testing.T) {
	root, p := packedFixture(t, "FROM=image")

	// Local change diverges from the image.
	writeRepoContent(t, root, ".env.local", "FROM=local")

	var out bytes.Buffer
	err := doRestore(root, p, false, false, &out)
	if err == nil || !strings.Contains(err.Error(), "differ") {
		t.Fatalf("expected conflict error, got %v", err)
	}
	// Non-destructive: local change preserved.
	assertRepoContent(t, root, ".env.local", "FROM=local")
	if !strings.Contains(out.String(), "conflict") {
		t.Errorf("output = %q, want a conflict line", out.String())
	}

	// --force overwrites with the image content.
	var out2 bytes.Buffer
	if err := doRestore(root, p, true, false, &out2); err != nil {
		t.Fatalf("doRestore --force: %v", err)
	}
	assertRepoContent(t, root, ".env.local", "FROM=image")
}

func TestDoRestoreDryRun(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	// Diverge the local file so dry-run has a conflict to report.
	writeRepoContent(t, root, ".env.local", "LOCAL")

	var out bytes.Buffer
	if err := doRestore(root, p, false, true, &out); err != nil {
		t.Fatalf("dry-run should not error on conflicts, got %v", err)
	}
	if !strings.Contains(out.String(), "Dry run") || !strings.Contains(out.String(), "conflict") {
		t.Errorf("dry-run output = %q, want a conflict preview", out.String())
	}
	// The working tree must be untouched.
	assertRepoContent(t, root, ".env.local", "LOCAL")
}

func TestDoRestoreNoImage(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)

	var out bytes.Buffer
	err := doRestore(root, p, false, false, &out)
	if err == nil || !strings.Contains(err.Error(), "no image") {
		t.Fatalf("expected no-image error, got %v", err)
	}
}

// packedFixture builds a repo with one tracked file packed into an image, and
// returns the repo root and identity paths.
func packedFixture(t *testing.T, content string) (string, identity.Paths) {
	t.Helper()
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)
	writeRepoContent(t, root, ".env.local", content)

	var discard bytes.Buffer
	if err := doAdd(root, []string{".env.local"}, &discard); err != nil {
		t.Fatal(err)
	}
	if err := doPack(root, p, false, false, &discard); err != nil {
		t.Fatal(err)
	}
	return root, p
}

func writeRepoContent(t *testing.T, root, rel, content string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertRepoContent(t *testing.T, root, rel, want string) {
	t.Helper()
	got, err := os.ReadFile(filepath.Join(root, rel)) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", rel, got, want)
	}
}
