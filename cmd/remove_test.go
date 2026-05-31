package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestDoRemoveExisting(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	mustTouch(t, root, ".env.local")

	var out bytes.Buffer
	if err := doAdd(root, []string{".env.local"}, &out); err != nil {
		t.Fatalf("add: %v", err)
	}

	out.Reset()
	if err := doRemove(root, []string{".env.local"}, &out); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if got := allowList(t, root); len(got) != 0 {
		t.Errorf("allow = %v, want empty after remove", got)
	}
	if !strings.Contains(out.String(), "removed .env.local") {
		t.Errorf("output = %q, want 'removed .env.local'", out.String())
	}
}

func TestDoRemoveAbsentIsNoop(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)

	var out bytes.Buffer
	if err := doRemove(root, []string{"never-added"}, &out); err != nil {
		t.Fatalf("remove absent should not error, got %v", err)
	}
	if !strings.Contains(out.String(), "not tracked") {
		t.Errorf("output = %q, want 'not tracked'", out.String())
	}
}

func TestDoRemoveWithoutManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	err := doRemove(root, []string{"x"}, &out)
	if err == nil || !strings.Contains(err.Error(), "dew init") {
		t.Fatalf("expected init hint, got %v", err)
	}
}
