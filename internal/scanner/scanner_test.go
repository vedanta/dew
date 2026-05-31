package scanner

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestScanClassifiesCandidatesAndNoise(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "/.env.local\n*.local\ncerts/\nnode_modules/\n*.log\n")

	// git-ignored, worth tracking → candidates
	writeFile(t, root, ".env.local", "TOKEN=abc")
	writeFile(t, root, "app.local", "x")
	writeFile(t, root, "certs/dev.pem", "PEM")
	// tracked source → neither
	writeFile(t, root, "src/main.go", "package main")
	// noise → skipped
	writeFile(t, root, "node_modules/pkg/index.js", "junk")
	writeFile(t, root, "debug.log", "log")
	writeFile(t, root, ".DS_Store", "")

	res, err := Scan(root, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	wantCandidates := []string{".env.local", "app.local", "certs/dev.pem"}
	for _, w := range wantCandidates {
		if !slices.Contains(res.Candidates, w) {
			t.Errorf("candidates %v missing %q", res.Candidates, w)
		}
	}
	for _, notWanted := range []string{"src/main.go", "debug.log", ".DS_Store"} {
		if slices.Contains(res.Candidates, notWanted) {
			t.Errorf("candidates %v should not contain %q", res.Candidates, notWanted)
		}
	}
	// node_modules must never be descended into or suggested.
	for _, c := range res.Candidates {
		if strings.HasPrefix(c, "node_modules") {
			t.Errorf("candidate %q is inside node_modules", c)
		}
	}

	if !slices.Contains(res.Skipped, "node_modules/") {
		t.Errorf("skipped %v should report node_modules/", res.Skipped)
	}
	if !slices.Contains(res.Skipped, ".DS_Store") || !slices.Contains(res.Skipped, "debug.log") {
		t.Errorf("skipped %v should include .DS_Store and debug.log", res.Skipped)
	}
}

func TestScanHonorsPerManifestDeny(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".gitignore", "*.local\n")
	writeFile(t, root, "keep.local", "k")
	writeFile(t, root, "scratch.local", "s")

	// Without extra deny, both are candidates.
	res, err := Scan(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Candidates) != 2 {
		t.Fatalf("candidates = %v, want 2 before deny", res.Candidates)
	}

	// A per-manifest deny pattern removes one.
	res, err = Scan(root, []string{"scratch.local"})
	if err != nil {
		t.Fatal(err)
	}
	if slices.Contains(res.Candidates, "scratch.local") {
		t.Errorf("scratch.local should be denied, candidates = %v", res.Candidates)
	}
	if !slices.Contains(res.Candidates, "keep.local") {
		t.Errorf("keep.local should remain a candidate, candidates = %v", res.Candidates)
	}
}

func TestScanNoGitignore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, ".env.local", "x")

	res, err := Scan(root, nil)
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if len(res.Candidates) != 0 {
		t.Errorf("candidates = %v, want none without a .gitignore", res.Candidates)
	}
}

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
