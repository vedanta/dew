package identity

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestGenerateCreatesIdentity(t *testing.T) {
	p := NewPaths(filepath.Join(t.TempDir(), ".dew"))

	pub, err := Generate(p)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if !strings.HasPrefix(pub, "age1") {
		t.Errorf("public key = %q, want an age1... recipient", pub)
	}

	// Key, pub, and images dir should exist.
	assertExists(t, p.KeyFile)
	assertExists(t, p.PubFile)
	if fi, err := os.Stat(p.ImagesDir); err != nil || !fi.IsDir() {
		t.Errorf("images dir not created: %v", err)
	}

	// Private key must be 0600 (Unix only — Windows has no Unix permission bits).
	if runtime.GOOS != "windows" {
		if fi, err := os.Stat(p.KeyFile); err != nil {
			t.Fatal(err)
		} else if perm := fi.Mode().Perm(); perm != 0o600 {
			t.Errorf("key file perm = %o, want 600", perm)
		}
	}

	// Pub file content matches the returned public key.
	got, err := os.ReadFile(p.PubFile)
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(string(got)) != pub {
		t.Errorf("pub file = %q, want %q", strings.TrimSpace(string(got)), pub)
	}
}

func TestGenerateRefusesOverwrite(t *testing.T) {
	p := NewPaths(filepath.Join(t.TempDir(), ".dew"))
	if _, err := Generate(p); err != nil {
		t.Fatalf("first Generate: %v", err)
	}
	_, err := Generate(p)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}
}

func TestInspectAbsentThenPresent(t *testing.T) {
	p := NewPaths(filepath.Join(t.TempDir(), ".dew"))

	s, err := Inspect(p)
	if err != nil {
		t.Fatalf("Inspect (absent): %v", err)
	}
	if s.Present {
		t.Error("Present = true, want false before keygen")
	}

	pub, err := Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	s, err = Inspect(p)
	if err != nil {
		t.Fatalf("Inspect (present): %v", err)
	}
	if !s.Present {
		t.Error("Present = false, want true after keygen")
	}
	if s.PublicKey != pub {
		t.Errorf("PublicKey = %q, want %q", s.PublicKey, pub)
	}
}

func TestInspectDerivesPublicKeyWhenPubMissing(t *testing.T) {
	p := NewPaths(filepath.Join(t.TempDir(), ".dew"))
	pub, err := Generate(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(p.PubFile); err != nil {
		t.Fatal(err)
	}

	s, err := Inspect(p)
	if err != nil {
		t.Fatalf("Inspect: %v", err)
	}
	if !s.Present || s.PublicKey != pub {
		t.Errorf("derived public key = %q (present=%v), want %q", s.PublicKey, s.Present, pub)
	}
}

func assertExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Errorf("expected %s to exist: %v", path, err)
	}
}
