package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestDoListEmpty(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)

	var out bytes.Buffer
	if err := doList(root, &out); err != nil {
		t.Fatalf("doList: %v", err)
	}
	if !strings.Contains(out.String(), "Project:") {
		t.Errorf("output = %q, want a Project header", out.String())
	}
	if !strings.Contains(out.String(), "(none)") {
		t.Errorf("output = %q, want an empty-list note", out.String())
	}
}

func TestDoListReflectsAddRemove(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	mustTouch(t, root, ".env.local")
	mustTouch(t, root, "certs/dev.pem")

	var discard bytes.Buffer
	if err := doAdd(root, []string{".env.local", "certs/dev.pem"}, &discard); err != nil {
		t.Fatalf("add: %v", err)
	}

	var out bytes.Buffer
	if err := doList(root, &out); err != nil {
		t.Fatalf("doList: %v", err)
	}
	if !strings.Contains(out.String(), ".env.local") || !strings.Contains(out.String(), "certs/dev.pem") {
		t.Errorf("output = %q, want both tracked paths", out.String())
	}

	// After removing one, list should no longer show it.
	if err := doRemove(root, []string{".env.local"}, &discard); err != nil {
		t.Fatalf("remove: %v", err)
	}
	out.Reset()
	if err := doList(root, &out); err != nil {
		t.Fatalf("doList: %v", err)
	}
	if strings.Contains(out.String(), ".env.local") {
		t.Errorf("output = %q, should not list removed .env.local", out.String())
	}
	if !strings.Contains(out.String(), "certs/dev.pem") {
		t.Errorf("output = %q, want remaining certs/dev.pem", out.String())
	}
}

func TestDoListWithoutManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	err := doList(root, &out)
	if err == nil || !strings.Contains(err.Error(), "dew init") {
		t.Fatalf("expected init hint, got %v", err)
	}
}
