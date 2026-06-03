package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/devices"
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

func TestDoKeyPullCancelledAtPrompt(t *testing.T) {
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	var out bytes.Buffer
	err := doKeyPull(p, "user@host", false, false, strings.NewReader("n\n"), &out)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancelled, got %v", err)
	}
}

func TestInstallPulledKeyVerifiesAndInstalls(t *testing.T) {
	src := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	if _, err := identity.Generate(src); err != nil {
		t.Fatal(err)
	}
	stSrc, err := identity.Inspect(src)
	if err != nil {
		t.Fatal(err)
	}

	tgt := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	if err := os.MkdirAll(tgt.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	tmp := tgt.KeyFile + ".pulling"
	data, err := os.ReadFile(src.KeyFile)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(tmp, data, 0o600); err != nil { //nolint:gosec // G703: tmp is a test-local temp path
		t.Fatal(err)
	}

	if err := installPulledKey(tgt, tmp, stSrc.PublicKey); err != nil {
		t.Fatalf("installPulledKey: %v", err)
	}
	st, err := identity.Inspect(tgt)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Present || st.PublicKey != stSrc.PublicKey {
		t.Errorf("target identity = %+v, want pub %s", st, stSrc.PublicKey)
	}
}

func TestInstallPulledKeyRejectsMismatch(t *testing.T) {
	src := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	if _, err := identity.Generate(src); err != nil {
		t.Fatal(err)
	}
	tgt := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	if err := os.MkdirAll(tgt.Home, 0o700); err != nil {
		t.Fatal(err)
	}
	tmp := tgt.KeyFile + ".pulling"
	data, _ := os.ReadFile(src.KeyFile)
	if err := os.WriteFile(tmp, data, 0o600); err != nil { //nolint:gosec // G703: tmp is a test-local temp path
		t.Fatal(err)
	}

	err := installPulledKey(tgt, tmp, "age1qqqwrongkey0000000000000000000000000000000000000000")
	if err == nil || !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("expected verification failure, got %v", err)
	}
	if _, e := os.Stat(tgt.KeyFile); e == nil {
		t.Error("key must not be installed on a verification failure")
	}
}

func TestDoKeyDevicesEmptyAndPopulated(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".dew")

	var empty bytes.Buffer
	if err := doKeyDevices(home, &empty); err != nil {
		t.Fatalf("doKeyDevices: %v", err)
	}
	if !strings.Contains(empty.String(), "No identity transfers recorded") {
		t.Errorf("empty output = %q", empty.String())
	}

	// Recording a transfer should then show up in the listing.
	recordLocal(home, devices.Entry{
		Peer: "vbarooah@nvk2", Direction: devices.SentTo,
		Fingerprint: "age1qdu770rzemhrmnukjhdu3pa5p2374fhprnm3l0aymcxxfx3wre0syc8rza",
		At:          "2026-06-02T17:00:00Z", Label: "nvk2",
	}, &bytes.Buffer{})

	var out bytes.Buffer
	if err := doKeyDevices(home, &out); err != nil {
		t.Fatalf("doKeyDevices: %v", err)
	}
	for _, want := range []string{"vbarooah@nvk2", "sent-to", "age1qdu770rzemhr…", "nvk2"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("listing missing %q:\n%s", want, out.String())
		}
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
