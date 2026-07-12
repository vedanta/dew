package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestValidTag(t *testing.T) {
	for tag, want := range map[string]bool{
		"v0.6.0":        true,
		"v1.2.3-rc.1":   true,
		"0.6.0":         false,
		"latest":        false,
		"v":             false,
		"v0.6.0;rm -rf": false,
	} {
		if got := ValidTag(tag); got != want {
			t.Errorf("ValidTag(%q) = %v, want %v", tag, got, want)
		}
	}
}

func TestAssetName(t *testing.T) {
	if got := AssetName("0.6.0", "darwin", "arm64"); got != "dew_0.6.0_darwin_arm64.tar.gz" {
		t.Errorf("AssetName darwin = %q", got)
	}
	if got := AssetName("0.6.0", "windows", "amd64"); got != "dew_0.6.0_windows_amd64.zip" {
		t.Errorf("AssetName windows = %q", got)
	}
}

func TestFindAsset(t *testing.T) {
	rel := &Release{Tag: "v0.6.0", Assets: []Asset{{Name: "checksums.txt"}, {Name: "dew_0.6.0_linux_amd64.tar.gz"}}}
	if _, err := FindAsset(rel, "dew_0.6.0_linux_amd64.tar.gz"); err != nil {
		t.Errorf("FindAsset existing: %v", err)
	}
	if _, err := FindAsset(rel, "dew_0.6.0_plan9_386.tar.gz"); err == nil {
		t.Error("FindAsset missing asset should error")
	}
}

func TestDownloadRefusesNonGitHubHosts(t *testing.T) {
	for _, u := range []string{
		"https://evil.example.com/dew.tar.gz",
		"http://github.com/insecure.tar.gz", // https only
		"https://github.com.evil.example/x.tar.gz",
	} {
		if _, err := Download(Asset{Name: "x", URL: u}); err == nil {
			t.Errorf("Download(%q) should refuse", u)
		}
	}
}

func TestParseAndVerifyChecksums(t *testing.T) {
	data := []byte("apple\nDEADBEEF  dew_0.6.0_linux_amd64.tar.gz\ncafe  other.zip\n")
	sums := ParseChecksums(data)
	if sums["dew_0.6.0_linux_amd64.tar.gz"] != "deadbeef" {
		t.Errorf("checksum parse = %v", sums)
	}

	payload := []byte("hello dew")
	sum := sha256.Sum256(payload)
	if err := VerifySHA256(payload, hex.EncodeToString(sum[:])); err != nil {
		t.Errorf("VerifySHA256 valid: %v", err)
	}
	if err := VerifySHA256(payload, "deadbeef"); err == nil {
		t.Error("VerifySHA256 should fail on mismatch")
	}
}

func makeTarGz(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = tw.Close()
	_ = gz.Close()
	return buf.Bytes()
}

func makeZip(t *testing.T, name string, content []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	_ = zw.Close()
	return buf.Bytes()
}

func TestExtractBinary(t *testing.T) {
	bin := []byte("#!fake dew binary")

	got, err := ExtractBinary(makeTarGz(t, "dew", bin), false)
	if err != nil || !bytes.Equal(got, bin) {
		t.Errorf("tar.gz extract = %q, %v", got, err)
	}
	if _, err := ExtractBinary(makeTarGz(t, "README.md", bin), false); err == nil {
		t.Error("tar.gz without dew should error")
	}

	got, err = ExtractBinary(makeZip(t, "dew.exe", bin), true)
	if err != nil || !bytes.Equal(got, bin) {
		t.Errorf("zip extract = %q, %v", got, err)
	}
	if _, err := ExtractBinary(makeZip(t, "docs/manual.md", bin), true); err == nil {
		t.Error("zip without dew.exe should error")
	}
}

func TestBrewManaged(t *testing.T) {
	if !BrewManaged("/opt/homebrew/Caskroom/dew/0.5.0/dew") {
		t.Error("Caskroom path should be brew-managed")
	}
	if BrewManaged("/usr/local/bin/dew") {
		t.Error("plain path should not be brew-managed")
	}
}

func TestReplaceExecutable(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "dew")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil { //nolint:gosec // test binary must be executable
		t.Fatal(err)
	}
	if err := ReplaceExecutable(exe, []byte("new")); err != nil {
		t.Fatalf("ReplaceExecutable: %v", err)
	}
	got, err := os.ReadFile(exe) //nolint:gosec // test-local path
	if err != nil || string(got) != "new" {
		t.Fatalf("replaced content = %q, %v", got, err)
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(exe)
		if info.Mode().Perm()&0o111 == 0 {
			t.Error("replaced binary lost its executable bit")
		}
	}
	// No stray temp files left behind.
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("expected only the binary in dir, got %d entries", len(entries))
	}
}
