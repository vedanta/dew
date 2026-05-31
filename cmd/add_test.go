package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/manifest"
)

func TestDoAddAppendsAndDedupes(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	mustTouch(t, root, ".env.local")

	var out bytes.Buffer
	if err := doAdd(root, []string{".env.local"}, &out); err != nil {
		t.Fatalf("doAdd: %v", err)
	}
	if got := allowList(t, root); len(got) != 1 || got[0] != ".env.local" {
		t.Fatalf("allow = %v, want [.env.local]", got)
	}
	if !strings.Contains(out.String(), "added .env.local") {
		t.Errorf("output = %q, want 'added .env.local'", out.String())
	}

	// Adding the same path again is idempotent.
	out.Reset()
	if err := doAdd(root, []string{".env.local"}, &out); err != nil {
		t.Fatalf("doAdd (dup): %v", err)
	}
	if got := allowList(t, root); len(got) != 1 {
		t.Errorf("allow = %v, want a single entry after dup add", got)
	}
	if !strings.Contains(out.String(), "already tracked") {
		t.Errorf("output = %q, want 'already tracked'", out.String())
	}
}

func TestDoAddRejectsOutsideRepo(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)

	var out bytes.Buffer
	err := doAdd(root, []string{"../escape"}, &out)
	if err == nil || !strings.Contains(err.Error(), "outside the repository") {
		t.Fatalf("expected outside-repo error, got %v", err)
	}
}

func TestDoAddWithoutManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	err := doAdd(root, []string{"x"}, &out)
	if err == nil || !strings.Contains(err.Error(), "dew init") {
		t.Fatalf("expected init hint, got %v", err)
	}
}

func TestRepoRelPath(t *testing.T) {
	root := t.TempDir()
	cases := []struct {
		name    string
		arg     string
		want    string
		wantErr bool
	}{
		{"plain file", ".env.local", ".env.local", false},
		{"nested", "certs/dev.pem", "certs/dev.pem", false},
		{"abs inside", filepath.Join(root, "a/b"), "a/b", false},
		{"parent escape", "../evil", "", true},
		{"abs outside", filepath.Join(filepath.Dir(root), "dew-outside"), "", true},
		{"repo root dot", ".", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := repoRelPath(root, c.arg)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, c.wantErr)
			}
			if !c.wantErr && got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}

func mustInit(t *testing.T, root string) {
	t.Helper()
	var discard bytes.Buffer
	if err := doInit(root, "", false, "", &discard); err != nil {
		t.Fatalf("init: %v", err)
	}
}

func mustTouch(t *testing.T, root, rel string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func allowList(t *testing.T, root string) []string {
	t.Helper()
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	return m.Allow
}
