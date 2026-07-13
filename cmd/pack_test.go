package cmd

import (
	"archive/tar"
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	if err := doPack(root, p, false, false, false, &out); err != nil {
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

func TestDoPackHonorsDeny(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)

	// An allow-listed directory containing both a wanted file and noise.
	writeRepoContent(t, root, "data/keep.txt", "keep")
	writeRepoContent(t, root, "data/scratch.tmp", "noise")

	// Set a per-manifest deny rule and allow the whole directory.
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	m.Deny = []string{"*.tmp"}
	m.AddAllow("data")
	if err := manifest.Save(manifest.Path(root), m); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doPack(root, p, false, false, false, &out); err != nil {
		t.Fatalf("doPack: %v", err)
	}

	imagePath := filepath.Join(p.ImagesDir, m.Image)
	names := imageNames(t, imagePath, p.KeyFile)
	if !slices.Contains(names, "data/keep.txt") {
		t.Errorf("image %v should contain data/keep.txt", names)
	}
	for _, n := range names {
		if strings.HasSuffix(n, ".tmp") {
			t.Errorf("deny-listed %q was packed (names %v)", n, names)
		}
	}
}

func TestDoPackDryRun(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)
	mustTouch(t, root, ".env.local")
	var d bytes.Buffer
	if err := doAdd(root, []string{".env.local"}, &d); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	if err := doPack(root, p, false, true, false, &out); err != nil {
		t.Fatalf("doPack --dry-run: %v", err)
	}
	if !strings.Contains(out.String(), "Dry run") || !strings.Contains(out.String(), ".env.local") {
		t.Errorf("dry-run output = %q", out.String())
	}
	m, _ := manifest.Load(manifest.Path(root))
	if _, err := os.Stat(filepath.Join(p.ImagesDir, m.Image)); err == nil {
		t.Error("dry-run wrote an image")
	}
}

func TestDoPackDryRunNeedsNoIdentity(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	mustTouch(t, root, ".env.local")
	var d bytes.Buffer
	if err := doAdd(root, []string{".env.local"}, &d); err != nil {
		t.Fatal(err)
	}
	// Identity never generated — dry-run should still work.
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	var out bytes.Buffer
	if err := doPack(root, p, false, true, false, &out); err != nil {
		t.Fatalf("dry-run should not require an identity, got %v", err)
	}
}

func TestDoPackNoManifest(t *testing.T) {
	root := t.TempDir()
	var out bytes.Buffer
	err := doPack(root, mustIdentityPaths(t), false, false, false, &out)
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
	err := doPack(root, p, false, false, false, &out)
	if err == nil || !strings.Contains(err.Error(), "dew keygen") {
		t.Fatalf("expected keygen hint, got %v", err)
	}
}

func TestDoPackEmptyAllowList(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	var out bytes.Buffer
	err := doPack(root, mustIdentityPaths(t), false, false, false, &out)
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
	err := doPack(root, mustIdentityPaths(t), false, false, false, &out)
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected not-found error, got %v", err)
	}
}

func TestDoPackSameRepoRepackAllowed(t *testing.T) {
	root, p := packedFixture(t, "TOKEN=abc") // packed once already
	// Re-packing the same repo is fine — it owns the image.
	if err := doPack(root, p, false, false, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("re-pack same repo should be allowed, got %v", err)
	}
}

func TestDoPackRefusesCrossRepoOverwrite(t *testing.T) {
	p := mustIdentityPaths(t) // one shared identity + images dir

	rootA := t.TempDir()
	setupSharedRepo(t, rootA, "shared", "AAA")
	if err := doPack(rootA, p, false, false, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("pack A: %v", err)
	}

	// Repo B uses the same project (→ same image name) but a different id.
	rootB := t.TempDir()
	setupSharedRepo(t, rootB, "shared", "BBB")

	err := doPack(rootB, p, false, false, false, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "different repo") {
		t.Fatalf("expected cross-repo refusal, got %v", err)
	}

	// --force lets B overwrite anyway.
	if err := doPack(rootB, p, true, false, false, &bytes.Buffer{}); err != nil {
		t.Fatalf("pack B --force: %v", err)
	}
}

func setupSharedRepo(t *testing.T, root, project, content string) {
	t.Helper()
	var d bytes.Buffer
	if err := doInit(root, project, false, "", &d); err != nil {
		t.Fatal(err)
	}
	writeRepoContent(t, root, "data.txt", content)
	if err := doAdd(root, []string{"data.txt"}, &d); err != nil {
		t.Fatal(err)
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

// imageNames reverses the pack pipeline and returns the tar entry names.
func imageNames(t *testing.T, imagePath, keyFile string) []string {
	t.Helper()
	tr := imageTar(t, imagePath, keyFile)
	var names []string
	for {
		hdr, err := tr.Next()
		if err != nil {
			break
		}
		names = append(names, hdr.Name)
	}
	return names
}

// imageTar reverses the pack pipeline (decrypt -> decompress) and returns a tar
// reader over the result.
func imageTar(t *testing.T, imagePath, keyFile string) *tar.Reader {
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
	tarball := &bytes.Buffer{}
	if err := compress.Decompress(tarball, &compressed); err != nil {
		t.Fatalf("decompress: %v", err)
	}
	return tar.NewReader(tarball)
}

// unpackEntry returns the content of the named entry in the image.
func unpackEntry(t *testing.T, imagePath, keyFile, name string) string {
	t.Helper()
	tr := imageTar(t, imagePath, keyFile)
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

// mustGitRepo turns root into a git repository, committing the given paths
// (which must already exist). Skips the test when git isn't on PATH.
func mustGitRepo(t *testing.T, root string, commitPaths ...string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	run := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...) //nolint:gosec // G204: test-local git invocation under t.TempDir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if len(commitPaths) > 0 {
		run(append([]string{"add", "--"}, commitPaths...)...)
		run("-c", "user.email=test@dew", "-c", "user.name=dew test", "commit", "-qm", "seed")
	}
}

func TestDoPackAllPacksOnlyTheLocalHalf(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)

	// Tracked source (Git carries it), private state, deny-listed noise, and
	// a vendored nested .git.
	writeRepoContent(t, root, "src/main.go", "package main")
	writeRepoContent(t, root, "wip-notes.md", "not committed yet")
	writeRepoContent(t, root, ".env.local", "TOKEN=xyz")
	writeRepoContent(t, root, "node_modules/pkg/index.js", "noise")
	writeRepoContent(t, root, "vendor/thirdparty/.git/config", "[core]")
	mustGitRepo(t, root, "src/main.go")

	var out bytes.Buffer
	if err := doPack(root, p, false, false, true, &out); err != nil {
		t.Fatalf("doPack --all: %v", err)
	}
	if !strings.Contains(out.String(), "local file") {
		t.Errorf("output = %q, want 'local file'", out.String())
	}

	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Allow) != 0 {
		t.Errorf("--all must not touch the allow-list, got %v", m.Allow)
	}

	names := imageNames(t, filepath.Join(p.ImagesDir, m.Image), p.KeyFile)
	for _, want := range []string{".env.local", "wip-notes.md"} {
		if !slices.Contains(names, want) {
			t.Errorf("image %v should contain %s", names, want)
		}
	}
	if slices.Contains(names, "src/main.go") {
		t.Errorf("tracked src/main.go must not be packed (Git carries it), image %v", names)
	}
	for _, n := range names {
		if strings.HasPrefix(n, "node_modules/") || strings.Contains(n, ".git/") || strings.HasPrefix(n, ".dew/") {
			t.Errorf("image must not contain %q (names %v)", n, names)
		}
	}
}

func TestDoPackAllRequiresGitRepo(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)
	writeRepoContent(t, root, ".env.local", "TOKEN=xyz")
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}

	var out bytes.Buffer
	err := doPack(root, p, false, false, true, &out)
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Fatalf("expected not-a-git-repository error, got %v", err)
	}
}

func TestDoPackAllRefusesWhenNothingLocal(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)
	writeRepoContent(t, root, "src/main.go", "package main")
	mustGitRepo(t, root, "src/main.go")

	var out bytes.Buffer
	err := doPack(root, p, false, false, true, &out)
	if err == nil || !strings.Contains(err.Error(), "no local files") {
		t.Fatalf("expected no-local-files error, got %v", err)
	}
}

func TestDoPackAllDryRun(t *testing.T) {
	root := t.TempDir()
	mustInit(t, root)
	writeRepoContent(t, root, "src/main.go", "package main")
	writeRepoContent(t, root, ".env.local", "TOKEN=xyz")
	// Fresh repo, nothing committed: both files are still local.
	mustGitRepo(t, root)

	// Allow-list empty: fine with --all, and dry-run needs no identity.
	p := identity.NewPaths(filepath.Join(t.TempDir(), ".dew"))
	var out bytes.Buffer
	if err := doPack(root, p, false, true, true, &out); err != nil {
		t.Fatalf("doPack --all --dry-run: %v", err)
	}
	for _, want := range []string{"Dry run", "src/main.go", ".env.local"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("dry-run output %q should mention %s", out.String(), want)
		}
	}
	m, _ := manifest.Load(manifest.Path(root))
	if _, err := os.Stat(filepath.Join(p.ImagesDir, m.Image)); err == nil {
		t.Error("dry-run wrote an image")
	}
}

func TestDoPackExplicitFileAllowBeatsDeny(t *testing.T) {
	root := t.TempDir()
	p := mustIdentityPaths(t)
	mustInit(t, root)

	// An explicitly added .log file, and a directory whose .log content is
	// swept in only via the dir entry.
	writeRepoContent(t, root, "keep.log", "explicit")
	writeRepoContent(t, root, "data/app.log", "swept")
	writeRepoContent(t, root, "data/config.yaml", "real")

	var discard bytes.Buffer
	if err := doAdd(root, []string{"keep.log", "data"}, &discard); err != nil {
		t.Fatal(err)
	}
	if err := doPack(root, p, false, false, false, &discard); err != nil {
		t.Fatalf("doPack: %v", err)
	}

	m, _ := manifest.Load(manifest.Path(root))
	names := imageNames(t, filepath.Join(p.ImagesDir, m.Image), p.KeyFile)
	if !slices.Contains(names, "keep.log") {
		t.Errorf("explicitly added keep.log missing from image %v", names)
	}
	if !slices.Contains(names, "data/config.yaml") {
		t.Errorf("data/config.yaml missing from image %v", names)
	}
	if slices.Contains(names, "data/app.log") {
		t.Errorf("dir-swept data/app.log should stay deny-filtered, image %v", names)
	}
}
