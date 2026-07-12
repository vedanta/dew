package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/manifest"
)

func manifestAndImage(t *testing.T, root, imagesDir string) (string, string) {
	t.Helper()
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	return manifest.Path(root), filepath.Join(imagesDir, m.Image)
}

func TestDoCleanRemovesBoth(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	manifestPath, imagePath := manifestAndImage(t, root, p.ImagesDir)

	var out bytes.Buffer
	// assumeYes=true so no prompt; force=false (no owner mismatch here).
	if err := doClean(root, p, false, true, false, false, strings.NewReader(""), &out); err != nil {
		t.Fatalf("doClean: %v", err)
	}
	if fileExists(imagePath) {
		t.Error("image was not removed")
	}
	if fileExists(ownerMarkerPath(imagePath)) {
		t.Error(".id marker was not removed")
	}
	if fileExists(manifestPath) {
		t.Error("manifest was not removed")
	}
	if fileExists(filepath.Dir(manifestPath)) {
		t.Error("empty .dew dir was left behind")
	}
	if !strings.Contains(out.String(), "no longer manages this repo") {
		t.Errorf("output = %q", out.String())
	}
}

func TestDoCleanImageOnlyKeepsManifest(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	manifestPath, imagePath := manifestAndImage(t, root, p.ImagesDir)

	var out bytes.Buffer
	if err := doClean(root, p, false, true, true /*imageOnly*/, false, strings.NewReader(""), &out); err != nil {
		t.Fatalf("doClean --image-only: %v", err)
	}
	if fileExists(imagePath) {
		t.Error("image should have been removed")
	}
	if !fileExists(manifestPath) {
		t.Error("manifest should have been kept")
	}
}

func TestDoCleanManifestOnlyKeepsImage(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	manifestPath, imagePath := manifestAndImage(t, root, p.ImagesDir)

	var out bytes.Buffer
	if err := doClean(root, p, false, true, false, true /*manifestOnly*/, strings.NewReader(""), &out); err != nil {
		t.Fatalf("doClean --manifest-only: %v", err)
	}
	if !fileExists(imagePath) {
		t.Error("image should have been kept")
	}
	if fileExists(manifestPath) {
		t.Error("manifest should have been removed")
	}
}

func TestDoCleanPromptCancels(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	manifestPath, imagePath := manifestAndImage(t, root, p.ImagesDir)

	var out bytes.Buffer
	// Decline at the prompt (assumeYes=false, input "n").
	err := doClean(root, p, false, false, false, false, strings.NewReader("n\n"), &out)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if !fileExists(imagePath) || !fileExists(manifestPath) {
		t.Error("a cancelled clean must not remove anything")
	}
}

func TestDoCleanNoManifest(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	var out bytes.Buffer
	err := doClean(root, p, false, true, false, false, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "no manifest") {
		t.Fatalf("expected no-manifest error, got %v", err)
	}
}

func TestDoCleanRefusesCrossRepoImage(t *testing.T) {
	p := mustIdentityPaths(t)

	// Repo A packs the shared image name and owns it.
	rootA := t.TempDir()
	setupSharedRepo(t, rootA, "shared", "AAA")
	if err := doPack(rootA, p, false, false, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("pack A: %v", err)
	}

	// Repo B claims the same project (same image name) with a different id.
	rootB := t.TempDir()
	setupSharedRepo(t, rootB, "shared", "BBB")

	var out bytes.Buffer
	err := doClean(rootB, p, false /*force*/, true, false, false, strings.NewReader(""), &out)
	if err == nil || !strings.Contains(err.Error(), "different repo") {
		t.Fatalf("expected cross-repo refusal, got %v", err)
	}

	// --force overrides the guard.
	if err := doClean(rootB, p, true, true, false, false, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("doClean --force: %v", err)
	}
}

func TestDoCleanNothingToClean(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	_, imagePath := manifestAndImage(t, root, p.ImagesDir)
	// Remove the image out of band, then clean --image-only finds nothing.
	if _, err := removeImageFile(imagePath); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := doClean(root, p, false, true, true, false, strings.NewReader(""), &out); err != nil {
		t.Fatalf("doClean: %v", err)
	}
	if !strings.Contains(out.String(), "Nothing to clean") {
		t.Errorf("output = %q", out.String())
	}
}

func TestDoImagesRemove(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	m, _ := manifest.Load(manifest.Path(root))
	project := strings.TrimSuffix(m.Image, ".dew.age")
	imagePath := filepath.Join(p.ImagesDir, m.Image)

	var out bytes.Buffer
	if err := doImagesRemove(p.ImagesDir, []string{project}, true, strings.NewReader(""), &out); err != nil {
		t.Fatalf("doImagesRemove: %v", err)
	}
	if fileExists(imagePath) || fileExists(ownerMarkerPath(imagePath)) {
		t.Error("image and marker should be gone")
	}
	if !strings.Contains(out.String(), "removed") {
		t.Errorf("output = %q", out.String())
	}
}

func TestDoImagesRemoveAcceptsExtension(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	m, _ := manifest.Load(manifest.Path(root))
	imagePath := filepath.Join(p.ImagesDir, m.Image)

	// Passing the full "<project>.dew.age" must work the same as the bare name.
	if err := doImagesRemove(p.ImagesDir, []string{m.Image}, true, strings.NewReader(""), &bytes.Buffer{}); err != nil {
		t.Fatalf("doImagesRemove: %v", err)
	}
	if fileExists(imagePath) {
		t.Error("image should be gone")
	}
}

func TestDoImagesRemoveUnknownIsNoOp(t *testing.T) {
	p := mustIdentityPaths(t)
	if err := os.MkdirAll(p.ImagesDir, 0o700); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := doImagesRemove(p.ImagesDir, []string{"ghost"}, true, strings.NewReader(""), &out); err != nil {
		t.Fatalf("doImagesRemove: %v", err)
	}
	if !strings.Contains(out.String(), "no image for") {
		t.Errorf("output = %q", out.String())
	}
}

func TestDoImagesRemoveRejectsTraversal(t *testing.T) {
	p := mustIdentityPaths(t)
	for _, bad := range []string{"../escape", "a/b", `..\win`, ".."} {
		err := doImagesRemove(p.ImagesDir, []string{bad}, true, strings.NewReader(""), &bytes.Buffer{})
		if err == nil || !strings.Contains(err.Error(), "invalid project name") {
			t.Errorf("project %q: expected rejection, got %v", bad, err)
		}
	}
}

func TestDoImagesRemovePromptCancels(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	m, _ := manifest.Load(manifest.Path(root))
	project := strings.TrimSuffix(m.Image, ".dew.age")
	imagePath := filepath.Join(p.ImagesDir, m.Image)

	err := doImagesRemove(p.ImagesDir, []string{project}, false, strings.NewReader("n\n"), &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Fatalf("expected cancellation, got %v", err)
	}
	if !fileExists(imagePath) {
		t.Error("a cancelled images rm must not remove the image")
	}
}
