package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoteShowWhenUnset(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".dew")
	var out bytes.Buffer
	if err := doRemoteShow(home, &out); err != nil {
		t.Fatalf("doRemoteShow: %v", err)
	}
	if !strings.Contains(out.String(), "No remote configured") {
		t.Errorf("expected 'no remote' hint:\n%s", out.String())
	}
}

func TestRemoteSetShowUnsetRoundTrip(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".dew")

	var setOut bytes.Buffer
	if err := doRemoteSet(home, "nas:/vol1/dew", &setOut); err != nil {
		t.Fatalf("doRemoteSet: %v", err)
	}
	if !strings.Contains(setOut.String(), "nas:/vol1/dew") {
		t.Errorf("set output missing destination:\n%s", setOut.String())
	}

	var showOut bytes.Buffer
	if err := doRemoteShow(home, &showOut); err != nil {
		t.Fatalf("doRemoteShow: %v", err)
	}
	if strings.TrimSpace(showOut.String()) != "nas:/vol1/dew" {
		t.Errorf("show = %q, want nas:/vol1/dew", strings.TrimSpace(showOut.String()))
	}

	var unsetOut bytes.Buffer
	if err := doRemoteUnset(home, &unsetOut); err != nil {
		t.Fatalf("doRemoteUnset: %v", err)
	}
	var afterOut bytes.Buffer
	if err := doRemoteShow(home, &afterOut); err != nil {
		t.Fatalf("doRemoteShow after unset: %v", err)
	}
	if !strings.Contains(afterOut.String(), "No remote configured") {
		t.Errorf("expected cleared after unset:\n%s", afterOut.String())
	}
}

func TestRemoteSetTrimsAndRejectsEmpty(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".dew")

	if err := doRemoteSet(home, "   ", &bytes.Buffer{}); err == nil {
		t.Fatal("expected error setting an empty destination")
	}

	if err := doRemoteSet(home, "  nas:/vol1/dew  ", &bytes.Buffer{}); err != nil {
		t.Fatalf("doRemoteSet: %v", err)
	}
	var showOut bytes.Buffer
	if err := doRemoteShow(home, &showOut); err != nil {
		t.Fatalf("doRemoteShow: %v", err)
	}
	if strings.TrimSpace(showOut.String()) != "nas:/vol1/dew" {
		t.Errorf("expected trimmed destination, got %q", strings.TrimSpace(showOut.String()))
	}
}
