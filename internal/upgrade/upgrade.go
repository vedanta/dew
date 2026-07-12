// Package upgrade self-updates the dew binary from GitHub releases: resolve a
// release (latest or a named tag), pick the asset for the running platform,
// verify it against the release's checksums.txt, and atomically replace the
// current executable. Reading the public releases API is distribution, not a
// product integration — dew still never touches GitHub on the user's behalf.
package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const (
	apiLatest = "https://api.github.com/repos/vedanta/dew/releases/latest"
	apiByTag  = "https://api.github.com/repos/vedanta/dew/releases/tags/"

	// maxBinarySize caps extraction (decompression-bomb guard); dew binaries
	// are ~10 MiB.
	maxBinarySize = 200 << 20
)

var tagRe = regexp.MustCompile(`^v[0-9][0-9A-Za-z.\-+]*$`)

// ValidTag reports whether tag looks like a dew release tag (v0.6.0).
func ValidTag(tag string) bool { return tagRe.MatchString(tag) }

// Asset is one downloadable file attached to a release.
type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

// Release is the subset of the GitHub release payload dew needs.
type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}

// Version returns the release's version without the leading v.
func (r *Release) Version() string { return strings.TrimPrefix(r.Tag, "v") }

func client() *http.Client { return &http.Client{Timeout: 3 * time.Minute} }

// FetchRelease resolves the latest release, or the given tag when non-empty.
func FetchRelease(tag string) (*Release, error) {
	u := apiLatest
	if tag != "" {
		if !ValidTag(tag) {
			return nil, fmt.Errorf("upgrade: %q is not a release tag (expected e.g. v0.6.0)", tag)
		}
		u = apiByTag + tag
	}
	resp, err := client().Get(u) //nolint:gosec // G107: fixed API endpoints + validated tag suffix
	if err != nil {
		return nil, fmt.Errorf("upgrade: reach GitHub: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotFound {
		if tag != "" {
			return nil, fmt.Errorf("upgrade: no release tagged %s", tag)
		}
		return nil, errors.New("upgrade: no releases found")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upgrade: GitHub responded %s", resp.Status)
	}
	var rel Release
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&rel); err != nil {
		return nil, fmt.Errorf("upgrade: parse release: %w", err)
	}
	if rel.Tag == "" {
		return nil, errors.New("upgrade: release has no tag")
	}
	return &rel, nil
}

// AssetName returns the expected archive name for a version and platform,
// matching the GoReleaser naming (dew_0.6.0_darwin_arm64.tar.gz; zip on
// Windows).
func AssetName(version, goos, goarch string) string {
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}
	return fmt.Sprintf("dew_%s_%s_%s.%s", version, goos, goarch, ext)
}

// FindAsset picks the named asset from a release.
func FindAsset(rel *Release, name string) (Asset, error) {
	for _, a := range rel.Assets {
		if a.Name == name {
			return a, nil
		}
	}
	return Asset{}, fmt.Errorf("upgrade: release %s has no asset %s", rel.Tag, name)
}

// Download fetches an asset, refusing non-GitHub hosts.
func Download(a Asset) ([]byte, error) {
	u, err := url.Parse(a.URL)
	if err != nil || u.Scheme != "https" ||
		(u.Host != "github.com" && !strings.HasSuffix(u.Host, ".github.com") &&
			!strings.HasSuffix(u.Host, ".githubusercontent.com")) {
		return nil, fmt.Errorf("upgrade: refusing asset URL %q (not a GitHub host)", a.URL)
	}
	resp, err := client().Get(a.URL) //nolint:gosec // G107: host allow-listed to GitHub above
	if err != nil {
		return nil, fmt.Errorf("upgrade: download %s: %w", a.Name, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("upgrade: download %s: %s", a.Name, resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBinarySize))
	if err != nil {
		return nil, fmt.Errorf("upgrade: download %s: %w", a.Name, err)
	}
	return data, nil
}

// ParseChecksums reads a GoReleaser checksums.txt ("<hex>  <name>" lines).
func ParseChecksums(data []byte) map[string]string {
	out := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 {
			out[fields[1]] = strings.ToLower(fields[0])
		}
	}
	return out
}

// VerifySHA256 checks data against a lowercase hex digest.
func VerifySHA256(data []byte, wantHex string) error {
	got := sha256.Sum256(data)
	if hex.EncodeToString(got[:]) != strings.ToLower(wantHex) {
		return errors.New("upgrade: checksum mismatch — download corrupted or tampered; nothing was installed")
	}
	return nil
}

// ExtractBinary pulls the dew binary out of a release archive (tar.gz, or zip
// when zipArchive is set).
func ExtractBinary(archive []byte, zipArchive bool) ([]byte, error) {
	want := "dew"
	if zipArchive {
		want = "dew.exe"
		zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
		if err != nil {
			return nil, fmt.Errorf("upgrade: open zip: %w", err)
		}
		for _, f := range zr.File {
			if filepath.Base(f.Name) != want {
				continue
			}
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer func() { _ = rc.Close() }()
			return readCapped(rc)
		}
		return nil, fmt.Errorf("upgrade: %s not found in archive", want)
	}

	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("upgrade: open tar.gz: %w", err)
	}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("upgrade: %s not found in archive", want)
		}
		if err != nil {
			return nil, err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == want {
			return readCapped(tr)
		}
	}
}

func readCapped(r io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, maxBinarySize))
	if err != nil {
		return nil, err
	}
	if len(data) == maxBinarySize {
		return nil, errors.New("upgrade: binary in archive exceeds size cap")
	}
	return data, nil
}

// BrewManaged reports whether the executable resolves into a Homebrew
// Caskroom — replacing such a binary corrupts brew's bookkeeping.
func BrewManaged(exePath string) bool {
	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		resolved = exePath
	}
	return strings.Contains(filepath.ToSlash(resolved), "/Caskroom/")
}

// ReplaceExecutable atomically swaps the binary at exePath with newBin: write
// a temp file alongside, then rename over (Windows: park the running exe as
// .old first, since a running image can't be overwritten in place).
func ReplaceExecutable(exePath string, newBin []byte) error {
	resolved, err := filepath.EvalSymlinks(exePath)
	if err == nil {
		exePath = resolved
	}
	dir := filepath.Dir(exePath)
	tmp, err := os.CreateTemp(dir, ".dew-upgrade-*")
	if err != nil {
		return fmt.Errorf("upgrade: stage new binary: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op after successful rename

	if _, err := tmp.Write(newBin); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("upgrade: stage new binary: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("upgrade: stage new binary: %w", err)
	}
	if err := os.Chmod(tmpName, 0o755); err != nil { //nolint:gosec // G302: it's an executable
		return fmt.Errorf("upgrade: stage new binary: %w", err)
	}

	if runtime.GOOS == "windows" {
		old := exePath + ".old"
		_ = os.Remove(old)
		if err := os.Rename(exePath, old); err != nil {
			return fmt.Errorf("upgrade: park running binary: %w", err)
		}
		defer func() { _ = os.Remove(old) }() // best-effort; fails (harmlessly) while it's the running image
	}
	if err := os.Rename(tmpName, exePath); err != nil {
		return fmt.Errorf("upgrade: install new binary: %w", err)
	}
	return nil
}
