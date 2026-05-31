package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/manifest"
)

func TestDoSyncPushNoDestination(t *testing.T) {
	root := t.TempDir()
	err := doSyncPush(root, mustIdentityPaths(t), "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no destination") {
		t.Fatalf("expected no-destination error, got %v", err)
	}
}

func TestDoSyncPushNoManifest(t *testing.T) {
	root := t.TempDir()
	err := doSyncPush(root, mustIdentityPaths(t), t.TempDir(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "dew init") {
		t.Fatalf("expected init hint, got %v", err)
	}
}

func TestDoSyncPushNoImage(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)
	err := doSyncPush(root, p, t.TempDir(), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "dew pack") {
		t.Fatalf("expected pack hint, got %v", err)
	}
}

func TestDoSyncPushLocal(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	dest := filepath.Join(t.TempDir(), "store")

	var out bytes.Buffer
	if err := doSyncPush(root, p, dest, &out); err != nil {
		t.Fatalf("doSyncPush: %v", err)
	}

	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dest, m.Image)); err != nil {
		t.Errorf("image not pushed to destination: %v", err)
	}
	if !strings.Contains(out.String(), "Pushed") {
		t.Errorf("output = %q, want 'Pushed'", out.String())
	}
}

func TestDoSyncPushPullLocalRoundTrip(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	dest := filepath.Join(t.TempDir(), "store")
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	localImage := filepath.Join(p.ImagesDir, m.Image)

	// Push, then simulate a machine with no local image.
	if err := doSyncPush(root, p, dest, &bytes.Buffer{}); err != nil {
		t.Fatalf("push: %v", err)
	}
	if err := os.Remove(localImage); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doSyncPull(root, p, dest, &out); err != nil {
		t.Fatalf("pull: %v", err)
	}
	if _, err := os.Stat(localImage); err != nil {
		t.Errorf("image not pulled back: %v", err)
	}
	if !strings.Contains(out.String(), "Pulled") {
		t.Errorf("output = %q, want 'Pulled'", out.String())
	}
}

func TestDoSyncPullNoDestination(t *testing.T) {
	root := t.TempDir()
	err := doSyncPull(root, mustIdentityPaths(t), "", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "no destination") {
		t.Fatalf("expected no-destination error, got %v", err)
	}
}
