package restore

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/vedanta/dew/internal/archive"
)

// Options controls restore behavior.
type Options struct {
	// Force overwrites files that exist and differ from the image. Without it,
	// such files are reported as conflicts and left untouched.
	Force bool
	// DryRun classifies every file exactly as a real restore would (respecting
	// Force) but writes nothing to the working tree.
	DryRun bool
}

// Result summarizes what a restore did.
type Result struct {
	Written     []string // newly created files
	Skipped     []string // already present and identical
	Conflicts   []string // present but differ; not overwritten (Force was false)
	Overwritten []string // present, differed, overwritten because Force was set
}

// Restore extracts the tar stream in r into destRoot, atomically and
// non-destructively. It stages into a temp directory under destRoot (so moves
// are same-filesystem and atomic), compares each file against any existing one
// by content hash, and only overwrites a differing file when Force is set.
// Path sanitization (tar-slip / symlink escape) is handled by the archive
// extractor before anything reaches the working tree.
func Restore(r io.Reader, destRoot string, opts Options) (Result, error) {
	var res Result

	stage, err := os.MkdirTemp(destRoot, ".dew-restore-*")
	if err != nil {
		return res, fmt.Errorf("restore: create staging dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(stage) }()

	if err := archive.Extract(r, stage); err != nil {
		return res, err
	}

	walkErr := filepath.WalkDir(stage, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return err
		}
		return placeFile(path, filepath.Join(destRoot, rel), rel, opts, &res)
	})
	if walkErr != nil {
		return res, walkErr
	}
	return res, nil
}

func placeFile(staged, target, rel string, opts Options, res *Result) error {
	switch _, statErr := os.Stat(target); {
	case errors.Is(statErr, os.ErrNotExist):
		if !opts.DryRun {
			if err := moveInto(staged, target); err != nil {
				return err
			}
		}
		res.Written = append(res.Written, rel)
		return nil
	case statErr != nil:
		return statErr
	}

	same, err := sameContent(staged, target)
	if err != nil {
		return err
	}
	switch {
	case same:
		res.Skipped = append(res.Skipped, rel)
	case !opts.Force:
		res.Conflicts = append(res.Conflicts, rel)
	default:
		if !opts.DryRun {
			if err := moveInto(staged, target); err != nil {
				return err
			}
		}
		res.Overwritten = append(res.Overwritten, rel)
	}
	return nil
}

func moveInto(staged, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("restore: mkdir for %s: %w", target, err)
	}
	if err := os.Rename(staged, target); err != nil {
		return fmt.Errorf("restore: install %s: %w", target, err)
	}
	return nil
}

func sameContent(a, b string) (bool, error) {
	ha, err := hashFile(a)
	if err != nil {
		return false, err
	}
	hb, err := hashFile(b)
	if err != nil {
		return false, err
	}
	return ha == hb, nil
}

func hashFile(path string) ([sha256.Size]byte, error) {
	var sum [sha256.Size]byte
	f, err := os.Open(path) //nolint:gosec // G304: paths are the repo working tree and our staging dir
	if err != nil {
		return sum, fmt.Errorf("restore: hash %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return sum, fmt.Errorf("restore: hash %s: %w", path, err)
	}
	copy(sum[:], h.Sum(nil))
	return sum, nil
}
