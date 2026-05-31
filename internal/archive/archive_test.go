package archive

import (
	"archive/tar"
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestBuildExtractRoundTrip(t *testing.T) {
	root := t.TempDir()
	writeSrc(t, root, ".env.local", "TOKEN=abc")
	writeSrc(t, root, "certs/dev.pem", "PEM")

	var buf bytes.Buffer
	if err := Build(&buf, root, []string{".env.local", "certs"}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	dest := t.TempDir()
	if err := Extract(&buf, dest); err != nil {
		t.Fatalf("Extract: %v", err)
	}

	assertFile(t, filepath.Join(dest, ".env.local"), "TOKEN=abc")
	assertFile(t, filepath.Join(dest, "certs", "dev.pem"), "PEM")
}

func TestBuildSkipsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation is unreliable on Windows runners")
	}
	root := t.TempDir()
	writeSrc(t, root, "real.txt", "data")
	if err := os.Symlink("real.txt", filepath.Join(root, "link.txt")); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Build(&buf, root, []string{"real.txt", "link.txt"}); err != nil {
		t.Fatalf("Build: %v", err)
	}

	names := tarNames(t, buf.Bytes())
	for _, n := range names {
		if n == "link.txt" {
			t.Errorf("archive should not contain the symlink, got names %v", names)
		}
	}
}

func TestExtractRejectsDotDot(t *testing.T) {
	mal := tarBytes(t, func(tw *tar.Writer) {
		body := []byte("pwned")
		_ = tw.WriteHeader(&tar.Header{Name: "../escape.txt", Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(body))})
		_, _ = tw.Write(body)
	})
	dest := t.TempDir()
	err := Extract(bytes.NewReader(mal), dest)
	if err == nil {
		t.Fatal("expected error on ../ traversal, got nil")
	}
	// The escaping file must not have been written outside dest.
	if _, statErr := os.Stat(filepath.Join(filepath.Dir(dest), "escape.txt")); statErr == nil {
		t.Fatal("traversal wrote a file outside dest")
	}
}

func TestExtractRejectsSymlinkEntry(t *testing.T) {
	mal := tarBytes(t, func(tw *tar.Writer) {
		_ = tw.WriteHeader(&tar.Header{Name: "link", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777})
	})
	if err := Extract(bytes.NewReader(mal), t.TempDir()); err == nil {
		t.Fatal("expected error on symlink entry, got nil")
	}
}

func TestExtractRejectsAbsolute(t *testing.T) {
	mal := tarBytes(t, func(tw *tar.Writer) {
		body := []byte("x")
		_ = tw.WriteHeader(&tar.Header{Name: "/abs/evil.txt", Typeflag: tar.TypeReg, Mode: 0o600, Size: int64(len(body))})
		_, _ = tw.Write(body)
	})
	if err := Extract(bytes.NewReader(mal), t.TempDir()); err == nil {
		t.Fatal("expected error on absolute path, got nil")
	}
}

func writeSrc(t *testing.T, root, rel, body string) {
	t.Helper()
	full := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path) //nolint:gosec // test path
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if string(got) != want {
		t.Errorf("%s = %q, want %q", path, got, want)
	}
}

func tarBytes(t *testing.T, build func(*tar.Writer)) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	build(tw)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func tarNames(t *testing.T, data []byte) []string {
	t.Helper()
	tr := tar.NewReader(bytes.NewReader(data))
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
