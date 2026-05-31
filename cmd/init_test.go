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

	if err := doInit(root, "", false, "", &out); err != nil {
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
	if err := doInit(root, "", false, "", &out); err != nil {
		t.Fatalf("first doInit: %v", err)
	}

	err := doInit(root, "", false, "", &out)
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
	if err := doInit(root, "", true, "", &out); err != nil {
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

func TestDoInitWithProjectFlag(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := doInit(root, "acme-api", false, "", &out); err != nil {
		t.Fatalf("doInit: %v", err)
	}
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if m.Project != "acme-api" {
		t.Errorf("project = %q, want acme-api", m.Project)
	}
	if m.Image != "acme-api.dew.age" {
		t.Errorf("image = %q, want acme-api.dew.age", m.Image)
	}
}

func TestDoInitInvalidProjectFlag(t *testing.T) {
	root := t.TempDir()
	err := doInit(root, "../evil", false, "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "project name") {
		t.Fatalf("expected invalid-project error, got %v", err)
	}
	// Explicit flag was invalid, so don't suggest --project.
	if strings.Contains(err.Error(), "pass --project") {
		t.Errorf("error should not suggest --project for an explicit flag: %v", err)
	}
}

func TestDoInitDerivedNameInvalidSuggestsFlag(t *testing.T) {
	// A directory whose base name isn't a valid project name (contains a space).
	root := filepath.Join(t.TempDir(), "my project")
	if err := os.MkdirAll(root, 0o750); err != nil {
		t.Fatal(err)
	}
	err := doInit(root, "", false, "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "pass --project") {
		t.Fatalf("expected a --project hint for an unusable directory name, got %v", err)
	}
}

func TestDoInitCollisionWarning(t *testing.T) {
	root := t.TempDir()
	imagesDir := t.TempDir()
	// Pre-create an image with the name this init will derive.
	project := filepath.Base(root)
	if err := os.WriteFile(filepath.Join(imagesDir, project+".dew.age"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doInit(root, "", false, imagesDir, &out); err != nil {
		t.Fatalf("doInit: %v", err)
	}
	if !strings.Contains(out.String(), "already exists") {
		t.Errorf("output = %q, want a collision warning", out.String())
	}
}
