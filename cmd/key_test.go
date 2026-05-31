package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/identity"
)

func TestDoKeygenCreatesIdentity(t *testing.T) {
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))

	var out bytes.Buffer
	if err := doKeygen(p, &out); err != nil {
		t.Fatalf("doKeygen: %v", err)
	}
	if !strings.Contains(out.String(), "Public key") || !strings.Contains(out.String(), "age1") {
		t.Errorf("output = %q, want a public key line", out.String())
	}
	if _, err := os.Stat(p.KeyFile); err != nil {
		t.Errorf("key file not created: %v", err)
	}
}

func TestDoKeygenRefusesOverwrite(t *testing.T) {
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	var out bytes.Buffer
	if err := doKeygen(p, &out); err != nil {
		t.Fatalf("first doKeygen: %v", err)
	}
	err := doKeygen(p, &out)
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("expected already-exists error, got %v", err)
	}
}
