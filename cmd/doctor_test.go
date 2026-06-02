package cmd

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
)

func TestDoctorNoIdentity(t *testing.T) {
	root := t.TempDir()
	assertDoctor(t, runDoctorFor(t, root, emptyIdentityPaths(t)), "No identity", "dew keygen")
}

func TestDoctorNoManifest(t *testing.T) {
	root := t.TempDir()
	assertDoctor(t, runDoctorFor(t, root, mustIdentityPaths(t)), "No manifest", "dew init")
}

func TestDoctorEmptyAllowList(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)
	assertDoctor(t, runDoctorFor(t, root, p), "tracks no files", "dew add")
}

func TestDoctorNoImage(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)
	writeRepoContent(t, root, ".env.local", "x")
	var d bytes.Buffer
	if err := doAdd(root, []string{".env.local"}, &d); err != nil {
		t.Fatal(err)
	}
	assertDoctor(t, runDoctorFor(t, root, p), "No image", "dew pack")
}

func TestDoctorMissingFileRecommendsRestore(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	removeRepoFile(t, root, ".env.local")
	assertDoctor(t, runDoctorFor(t, root, p), "missing", "dew restore")
}

func TestDoctorHealthy(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc")
	if out := runDoctorFor(t, root, p); !strings.Contains(out, "fully hydrated") {
		t.Errorf("expected fully hydrated:\n%s", out)
	}
}

func TestDoctorUndecryptableImage(t *testing.T) {
	// Pack with one identity, then diagnose with a different identity whose
	// images dir holds that image — it must report it cannot decrypt.
	root, pA := packedFixture(t, "TOKEN=abc")
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	pB := mustIdentityPaths(t)
	copyFile(t, filepath.Join(pA.ImagesDir, m.Image), filepath.Join(pB.ImagesDir, m.Image))

	assertDoctor(t, runDoctorFor(t, root, pB), "different identity", "dew keygen")
}

func runDoctorFor(t *testing.T, root string, p identity.Paths) string {
	t.Helper()
	var out bytes.Buffer
	if err := doDoctor(root, p, &out); err != nil {
		t.Fatalf("doDoctor: %v", err)
	}
	return out.String()
}

func assertDoctor(t *testing.T, out, wantProblem, wantRec string) {
	t.Helper()
	if !strings.Contains(out, wantProblem) {
		t.Errorf("output missing problem %q:\n%s", wantProblem, out)
	}
	if !strings.Contains(out, wantRec) {
		t.Errorf("output missing recommendation %q:\n%s", wantRec, out)
	}
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		t.Fatal(err)
	}
	in, err := os.Open(src) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = in.Close() }()
	out, err := os.Create(dst) //nolint:gosec // test path
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = out.Close() }()
	if _, err := io.Copy(out, in); err != nil {
		t.Fatal(err)
	}
}
