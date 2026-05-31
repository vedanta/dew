package restore

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

func TestRestoreWritesNewFiles(t *testing.T) {
	tarball := makeTar(t, map[string]string{
		".env.local":    "TOKEN=abc",
		"certs/dev.pem": "PEM",
	})
	dest := t.TempDir()

	res, err := Restore(bytes.NewReader(tarball), dest, Options{})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	assertContent(t, filepath.Join(dest, ".env.local"), "TOKEN=abc")
	assertContent(t, filepath.Join(dest, "certs", "dev.pem"), "PEM")

	if got := sorted(res.Written); len(got) != 2 {
		t.Errorf("Written = %v, want 2 entries", got)
	}
	assertNoStagingLeft(t, dest)
}

func TestRestoreSkipsIdentical(t *testing.T) {
	dest := t.TempDir()
	writeRepoFile(t, dest, ".env.local", "TOKEN=abc")
	tarball := makeTar(t, map[string]string{".env.local": "TOKEN=abc"})

	res, err := Restore(bytes.NewReader(tarball), dest, Options{})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(res.Skipped) != 1 || len(res.Written) != 0 {
		t.Errorf("Skipped=%v Written=%v, want 1 skipped / 0 written", res.Skipped, res.Written)
	}
}

func TestRestoreConflictNotOverwrittenWithoutForce(t *testing.T) {
	dest := t.TempDir()
	writeRepoFile(t, dest, ".env.local", "LOCAL=keepme")
	tarball := makeTar(t, map[string]string{".env.local": "IMAGE=different"})

	res, err := Restore(bytes.NewReader(tarball), dest, Options{Force: false})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(res.Conflicts) != 1 {
		t.Errorf("Conflicts = %v, want 1", res.Conflicts)
	}
	// The existing file must be untouched — no silent data loss.
	assertContent(t, filepath.Join(dest, ".env.local"), "LOCAL=keepme")
}

func TestRestoreForceOverwrites(t *testing.T) {
	dest := t.TempDir()
	writeRepoFile(t, dest, ".env.local", "LOCAL=keepme")
	tarball := makeTar(t, map[string]string{".env.local": "IMAGE=different"})

	res, err := Restore(bytes.NewReader(tarball), dest, Options{Force: true})
	if err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if len(res.Overwritten) != 1 {
		t.Errorf("Overwritten = %v, want 1", res.Overwritten)
	}
	assertContent(t, filepath.Join(dest, ".env.local"), "IMAGE=different")
}

func makeTar(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, body := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name, Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(body)),
		}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeRepoFile(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

func assertNoStagingLeft(t *testing.T, dest string) {
	t.Helper()
	entries, err := os.ReadDir(dest)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if len(e.Name()) >= len(".dew-restore-") && e.Name()[:len(".dew-restore-")] == ".dew-restore-" {
			t.Errorf("staging dir left behind: %s", e.Name())
		}
	}
}

func sorted(s []string) []string {
	out := append([]string(nil), s...)
	sort.Strings(out)
	return out
}
