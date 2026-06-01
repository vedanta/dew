package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDoImagesEmpty(t *testing.T) {
	var out bytes.Buffer
	// Nonexistent images dir → friendly empty message.
	if err := doImages(filepath.Join(t.TempDir(), "images"), &out); err != nil {
		t.Fatalf("doImages: %v", err)
	}
	if !strings.Contains(out.String(), "No images yet") {
		t.Errorf("output = %q, want empty message", out.String())
	}
}

func TestDoImagesLists(t *testing.T) {
	dir := t.TempDir()
	writeImagesFile(t, dir, "acme-api.dew.age", "ciphertext-a")
	writeImagesFile(t, dir, "acme-api.dew.age.id", "7f3a1b2c9d0e") // ownership marker
	writeImagesFile(t, dir, "liway.dew.age", "ciphertext-b")       // no marker
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o750); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doImages(dir, &out); err != nil {
		t.Fatalf("doImages: %v", err)
	}
	s := out.String()

	if !strings.Contains(s, "IMAGE") || !strings.Contains(s, "PROJECT") {
		t.Errorf("missing header:\n%s", s)
	}
	for _, want := range []string{"acme-api.dew.age", "acme-api", "liway.dew.age", "liway", "7f3a1b2c"} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q:\n%s", want, s)
		}
	}
	// The ownership marker must not be listed as its own image row.
	if strings.Contains(s, "acme-api.dew.age.id") {
		t.Errorf("ownership marker listed as an image:\n%s", s)
	}
	// liway has no marker → owner shows as "-".
	if !strings.Contains(s, "-") {
		t.Errorf("expected '-' owner for unmarked image:\n%s", s)
	}
}

func writeImagesFile(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}
