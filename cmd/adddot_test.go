package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestDoAddDiscoveredAcceptAll(t *testing.T) {
	root := t.TempDir()
	writeScanFile(t, root, ".gitignore", ".env.local\nconfig.local\nnode_modules/\n")
	writeScanFile(t, root, ".env.local", "a")
	writeScanFile(t, root, "config.local", "b")
	writeScanFile(t, root, "node_modules/x.js", "junk")
	mustInit(t, root)

	var out bytes.Buffer
	if err := doAddDiscovered(root, strings.NewReader(""), &out, true); err != nil {
		t.Fatalf("doAddDiscovered: %v", err)
	}

	got := allowList(t, root)
	if len(got) != 2 {
		t.Fatalf("allow = %v, want 2 candidates", got)
	}
	for _, c := range got {
		if strings.HasPrefix(c, "node_modules") {
			t.Errorf("noise %q was added", c)
		}
	}
}

func TestDoAddDiscoveredInteractive(t *testing.T) {
	root := t.TempDir()
	writeScanFile(t, root, ".gitignore", ".env.local\nconfig.local\n")
	writeScanFile(t, root, ".env.local", "a")
	writeScanFile(t, root, "config.local", "b")
	mustInit(t, root)

	// Candidates are sorted: ".env.local" then "config.local".
	// Answer yes to the first, no to the second.
	var out bytes.Buffer
	if err := doAddDiscovered(root, strings.NewReader("y\nn\n"), &out, false); err != nil {
		t.Fatalf("doAddDiscovered: %v", err)
	}

	got := allowList(t, root)
	if len(got) != 1 || got[0] != ".env.local" {
		t.Errorf("allow = %v, want [.env.local]", got)
	}
}

func TestDoAddDiscoveredNoNewCandidates(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root) // no .gitignore → nothing discovered

	var out bytes.Buffer
	if err := doAddDiscovered(root, strings.NewReader(""), &out, true); err != nil {
		t.Fatalf("doAddDiscovered: %v", err)
	}
	if !strings.Contains(out.String(), "No new candidates") {
		t.Errorf("output = %q, want no-new-candidates message", out.String())
	}
}

func TestDoAddDiscoveredNoManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	err := doAddDiscovered(root, strings.NewReader(""), &out, true)
	if err == nil || !strings.Contains(err.Error(), "dew init") {
		t.Fatalf("expected init hint, got %v", err)
	}
}

func TestIsYes(t *testing.T) {
	for _, in := range []string{"", "y\n", "Y", "yes", "  yes  \n"} {
		if !isYes(in) {
			t.Errorf("isYes(%q) = false, want true", in)
		}
	}
	for _, in := range []string{"n", "no", "x", "0"} {
		if isYes(in) {
			t.Errorf("isYes(%q) = true, want false", in)
		}
	}
}
