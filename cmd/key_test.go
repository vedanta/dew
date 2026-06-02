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

func TestDoKeyPushNoIdentity(t *testing.T) {
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew")) // no identity
	var out bytes.Buffer
	err := doKeyPush(p, "user@host", false, false, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "no identity") {
		t.Fatalf("expected no-identity error, got %v", err)
	}
}

func TestDoKeyPushCancelledAtPrompt(t *testing.T) {
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	var gen bytes.Buffer
	if err := doKeygen(p, &gen); err != nil {
		t.Fatalf("doKeygen: %v", err)
	}
	// Decline at the prompt — must cancel before any ssh/scp dependency check.
	var out bytes.Buffer
	err := doKeyPush(p, "user@host", false, false, strings.NewReader("n\n"), &out)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancelled, got %v", err)
	}
	if !strings.Contains(out.String(), "PRIVATE dew identity") {
		t.Errorf("expected the warning to be shown, got %q", out.String())
	}
}

func TestDoKeyStatusAbsent(t *testing.T) {
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	var out bytes.Buffer
	if err := doKeyStatus(p, &out); err != nil {
		t.Fatalf("doKeyStatus: %v", err)
	}
	if !strings.Contains(out.String(), "Not found") {
		t.Errorf("output = %q, want 'Not found'", out.String())
	}
}

func TestDoKeyStatusPresent(t *testing.T) {
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	var gen bytes.Buffer
	if err := doKeygen(p, &gen); err != nil {
		t.Fatalf("doKeygen: %v", err)
	}

	var out bytes.Buffer
	if err := doKeyStatus(p, &out); err != nil {
		t.Fatalf("doKeyStatus: %v", err)
	}
	if !strings.Contains(out.String(), "Present") || !strings.Contains(out.String(), "age1") {
		t.Errorf("output = %q, want 'Present' and the public key", out.String())
	}
}
