package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/manifest"
)

func TestDoInitCreatesValidManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer

	if err := doInit(root, false, &out); err != nil {
		t.Fatalf("doInit: %v", err)
	}

	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatalf("load created manifest: %v", err)
	}
	if m.Project != filepath.Base(root) {
		t.Errorf("project = %q, want %q", m.Project, filepath.Base(root))
	}
	if err := m.Validate(); err != nil {
		t.Errorf("created manifest does not validate: %v", err)
	}
	if !strings.Contains(out.String(), "Created") {
		t.Errorf("output = %q, want it to mention Created", out.String())
	}
}

func TestDoInitRefusesOverwrite(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := doInit(root, false, &out); err != nil {
		t.Fatalf("first doInit: %v", err)
	}

	err := doInit(root, false, &out)
	if err == nil {
		t.Fatal("expected error on second init, got nil")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Errorf("error = %q, want it to mention 'already exists'", err.Error())
	}
}

func TestDoInitFromGitignoreSeeds(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(".env.local\nnode_modules/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, rel := range []string{".env.local", "node_modules/dep.js"} {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("x"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	var out bytes.Buffer
	if err := doInit(root, true, &out); err != nil {
		t.Fatalf("doInit: %v", err)
	}

	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Allow) != 1 || m.Allow[0] != ".env.local" {
		t.Errorf("allow = %v, want [.env.local] (node_modules is noise)", m.Allow)
	}
	if !strings.Contains(out.String(), "Seeded 1 candidate") {
		t.Errorf("output = %q, want a seeded count", out.String())
	}
}
