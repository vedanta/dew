package cmd

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/compress"
	"github.com/vedanta/dew/internal/crypto"
	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
)

func TestDoPackProducesDecryptableImage(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)
	mustTouch(t, root, ".env.local")

	var discard bytes.Buffer
	if err := doAdd(root, []string{".env.local"}, &discard); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".env.local"), []byte("TOKEN=xyz"), 0o600); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doPack(root, p, &out); err != nil {
		t.Fatalf("doPack: %v", err)
	}
	if !strings.Contains(out.String(), "Packed") {
		t.Errorf("output = %q, want 'Packed'", out.String())
	}

	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	imagePath := filepath.Join(p.ImagesDir, m.Image)
	if _, err := os.Stat(imagePath); err != nil {
		t.Fatalf("image not created: %v", err)
	}

	if got := unpackEntry(t, imagePath, p.KeyFile, ".env.local"); got != "TOKEN=xyz" {
		t.Errorf("packed .env.local = %q, want TOKEN=xyz", got)
	}
}

func TestDoPackNoManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	err := doPack(root, mustIdentityPaths(t), &out)
	if err == nil || !strings.Contains(err.Error(), "dew init") {
		t.Fatalf("expected init hint, got %v", err)
	}
}

func TestDoPackNoIdentity(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	mustTouch(t, root, ".env.local")
	var discard bytes.Buffer
	_ = doAdd(root, []string{".env.local"}, &discard)

	// Identity paths that were never generated.
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	var out bytes.Buffer
	err := doPack(root, p, &out)
	if err == nil || !strings.Contains(err.Error(), "dew keygen") {
		t.Fatalf("expected keygen hint, got %v", err)
	}
}

func TestDoPackEmptyAllowList(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	var out bytes.Buffer
	err := doPack(root, mustIdentityPaths(t), &out)
	if err == nil || !strings.Contains(err.Error(), "nothing to pack") {
		t.Fatalf("expected empty-allow error, got %v", err)
	}
}

func TestDoPackMissingTrackedFile(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	var discard bytes.Buffer
	// Add a path that does not exist on disk (add only warns).
	_ = doAdd(root, []string{"ghost.env"}, &discard)

	var out bytes.Buffer
	err := doPack(root, mustIdentityPaths(t), &out)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func mustIdentityPaths(t *testing.T) identity.Paths {
	t.Helper()
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	if _, err := identity.Generate(p); err != nil {
		t.Fatalf("generate identity: %v", err)
	}
	return p
}

// unpackEntry reverses the pack pipeline (decrypt -> decompress -> untar) and
// returns the content of the named entry.
func unpackEntry(t *testing.T, imagePath, keyFile, name string) string {
	t.Helper()
	f, err := os.Open(imagePath) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()

	var compressed bytes.Buffer
	if err := crypto.Decrypt(&compressed, f, keyFile); err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	var tarball bytes.Buffer
	if err := compress.Decompress(&tarball, &compressed); err != nil {
		t.Fatalf("decompress: %v", err)
	}

	tr := tar.NewReader(&tarball)
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		if hdr.Name == name {
			b, err := io.ReadAll(tr)
			if err != nil {
				t.Fatal(err)
			}
			return string(b)
		}
	}
	t.Fatalf("entry %q not found in image", name)
	return ""
}
