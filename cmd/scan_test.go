package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoScanListsCandidatesAndSkipped(t *testing.T) {
	root := t.TempDir()
	writeScanFile(t, root, ".gitignore", ".env.local\nnode_modules/\n*.log\n")
	writeScanFile(t, root, ".env.local", "TOKEN=abc")
	writeScanFile(t, root, "node_modules/dep/index.js", "junk")
	writeScanFile(t, root, "debug.log", "log")

	var out bytes.Buffer
	if err := doScan(root, &out); err != nil {
		t.Fatalf("doScan: %v", err)
	}
	s := out.String()
	if !strings.Contains(s, "Candidates:") || !strings.Contains(s, ".env.local") {
		t.Errorf("output missing candidate .env.local:\n%s", s)
	}
	if !strings.Contains(s, "Skipped") || !strings.Contains(s, "node_modules/") {
		t.Errorf("output missing skipped node_modules/:\n%s", s)
	}
}

func TestDoScanMarksTracked(t *testing.T) {
	root := t.TempDir()
	writeScanFile(t, root, ".gitignore", ".env.local\n")
	writeScanFile(t, root, ".env.local", "TOKEN=abc")
	mustInit(t, root)
	var discard bytes.Buffer
	if err := doAdd(root, []string{".env.local"}, &discard); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doScan(root, &out); err != nil {
		t.Fatalf("doScan: %v", err)
	}
	if !strings.Contains(out.String(), ".env.local (already tracked)") {
		t.Errorf("output should mark .env.local tracked:\n%s", out.String())
	}
}

func TestDoScanNoCandidates(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	if err := doScan(root, &out); err != nil {
		t.Fatalf("doScan: %v", err)
	}
	if !strings.Contains(out.String(), "No candidate files found") {
		t.Errorf("output = %q, want no-candidates message", out.String())
	}
}

func writeScanFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
