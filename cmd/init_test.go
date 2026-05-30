package cmd

import (
	"bytes"
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

func TestDoInitFromGitignoreNote(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := doInit(root, true, &out); err != nil {
		t.Fatalf("doInit: %v", err)
	}
	if !strings.Contains(out.String(), "from-gitignore") {
		t.Errorf("output = %q, want a --from-gitignore note", out.String())
	}
}
