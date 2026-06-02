package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/identity"
)

func emptyIdentityPaths(t *testing.T) identity.Paths {
	t.Helper()
	return identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
}

func removeRepoFile(t *testing.T, root, rel string) {
	t.Helper()
	if err := os.Remove(filepath.Join(root, rel)); err != nil {
		t.Fatal(err)
	}
}

func TestDoStatusFreshRepo(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t) // identity present
	mustInit(t, root)

	var out bytes.Buffer
	if err := doStatus(root, p, "", &out); err != nil {
		t.Fatalf("doStatus: %v", err)
	}
	s := out.String()
	for _, want := range []string{"Identity:", "Present", "Manifest:", "Valid", "Image:", "Missing", "Sync:"} {
		if !strings.Contains(s, want) {
			t.Errorf("status missing %q:\n%s", want, s)
		}
	}
}

func TestDoStatusHealthyAfterPack(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc") // identity + manifest + image + file present

	var out bytes.Buffer
	if err := doStatus(root, p, "", &out); err != nil {
		t.Fatalf("doStatus: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "Image:") || !strings.Contains(s, "Present") {
		t.Errorf("expected image present:\n%s", s)
	}
	if !strings.Contains(s, "Healthy") {
		t.Errorf("expected healthy hydration:\n%s", s)
	}
}

func TestDoStatusIncompleteWhenFileMissing(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	// Simulate a fresh clone: the tracked file is gone but the image exists.
	removeRepoFile(t, root, ".env.local")

	var out bytes.Buffer
	if err := doStatus(root, p, "", &out); err != nil {
		t.Fatalf("doStatus: %v", err)
	}
	if !strings.Contains(out.String(), "Incomplete") || !strings.Contains(out.String(), "dew restore") {
		t.Errorf("expected incomplete + restore hint:\n%s", out.String())
	}
}

func TestDoStatusShowsConfiguredRemote(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)

	var out bytes.Buffer
	if err := doStatus(root, p, "nas:/vol1/dew", &out); err != nil {
		t.Fatalf("doStatus: %v", err)
	}
	if !strings.Contains(out.String(), "nas:/vol1/dew") {
		t.Errorf("expected configured remote in status:\n%s", out.String())
	}
}

func TestDoStatusNoIdentityNoManifest(t *testing.T) {
	root := t.TempDir()
	p := emptyIdentityPaths(t) // not generated

	var out bytes.Buffer
	if err := doStatus(root, p, "", &out); err != nil {
		t.Fatalf("doStatus: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "Not found (run 'dew keygen')") {
		t.Errorf("expected keygen hint:\n%s", s)
	}
	if !strings.Contains(s, "Not found (run 'dew init')") {
		t.Errorf("expected init hint:\n%s", s)
	}
}
