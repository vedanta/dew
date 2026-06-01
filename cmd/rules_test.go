package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/manifest"
)

func TestDoRulesShowsAllLayers(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	// Give the repo a deny rule.
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	m.Deny = []string{"*.tmp"}
	if err := manifest.Save(manifest.Path(root), m); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doRules(root, []string{"*.swp"}, &out); err != nil {
		t.Fatalf("doRules: %v", err)
	}
	s := out.String()
	for _, want := range []string{
		"Allow-list (repo)",
		"Deny — built-in", "node_modules/", "*.log",
		"Deny — global", "*.swp",
		"Deny — repo", "*.tmp",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("rules output missing %q:\n%s", want, s)
		}
	}
}

func TestDoRulesNoManifest(t *testing.T) {
	var out bytes.Buffer
	if err := doRules(t.TempDir(), nil, &out); err != nil {
		t.Fatalf("doRules: %v", err)
	}
	if !strings.Contains(out.String(), "no manifest") {
		t.Errorf("expected a no-manifest note:\n%s", out.String())
	}
	// Built-in deny is repo-independent, so it must still show.
	if !strings.Contains(out.String(), "node_modules/") {
		t.Errorf("built-in deny should show without a manifest:\n%s", out.String())
	}
}

func TestGlobalDenyAffectsScan(t *testing.T) {
	root := t.TempDir()
	writeScanFile(t, root, ".gitignore", "*.local\n")
	writeScanFile(t, root, "keep.local", "k")
	writeScanFile(t, root, "scratch.local", "s")
	mustInit(t, root)

	// Activate a global deny rule for this invocation, then restore it.
	defer func() { globalDenyPatterns = nil }()
	globalDenyPatterns = []string{"scratch.local"}

	var out bytes.Buffer
	if err := doScan(root, &out); err != nil {
		t.Fatalf("doScan: %v", err)
	}
	// scratch.local must not appear among Candidates (global deny moves it to Skipped).
	candidates, _, _ := strings.Cut(out.String(), "Skipped")
	if strings.Contains(candidates, "scratch.local") {
		t.Errorf("global deny should keep scratch.local out of candidates:\n%s", out.String())
	}
	if !strings.Contains(candidates, "keep.local") {
		t.Errorf("keep.local should still be a candidate:\n%s", out.String())
	}
}
