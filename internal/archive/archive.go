package archive

import (
	"archive/tar"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Build writes a tar archive of the given repo-relative paths (each resolved
// under root) to w. Directories are walked recursively; only regular files are
// archived — symlinks and special files are skipped, so an image never carries
// a symlink. Entry names are stored as clean, slash-separated repo-relative
// paths.
func Build(w io.Writer, root string, relPaths []string) error {
	tw := tar.NewWriter(w)
	for _, rel := range relPaths {
		abs := filepath.Join(root, filepath.FromSlash(rel))
		walkErr := filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.Type().IsRegular() {
				return nil // dirs are recreated on extract; symlinks/specials skipped
			}
			return writeEntry(tw, root, path, d)
		})
		if walkErr != nil {
			return fmt.Errorf("archive: build %s: %w", rel, walkErr)
		}
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("archive: finalize: %w", err)
	}
	return nil
}

func writeEntry(tw *tar.Writer, root, path string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	name, err := filepath.Rel(root, path)
	if err != nil {
		return err
	}
	hdr, err := tar.FileInfoHeader(info, "")
	if err != nil {
		return err
	}
	hdr.Name = filepath.ToSlash(name)
	if err := tw.WriteHeader(hdr); err != nil {
		return err
	}

	f, err := os.Open(path) //nolint:gosec // G304: path comes from walking repo-local allow-list entries
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(tw, f); err != nil {
		return err
	}
	return nil
}

// Extract reads a tar archive from r and writes its entries under dest. It
// rejects any entry that would escape dest (absolute paths or ".." traversal)
// and any non-regular, non-directory entry (notably symlinks/hardlinks),
// defending against tar-slip and symlink-escape attacks.
func Extract(r io.Reader, dest string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("archive: read: %w", err)
		}

		target, err := safeTarget(dest, hdr.Name)
		if err != nil {
			return err
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("archive: mkdir %s: %w", hdr.Name, err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return fmt.Errorf("archive: mkdir for %s: %w", hdr.Name, err)
			}
			if err := writeFile(target, tr, hdr.FileInfo().Mode().Perm()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("archive: refusing unsupported entry %q (tar type %d)", hdr.Name, hdr.Typeflag)
		}
	}
	return nil
}

// safeTarget resolves a tar entry name to an absolute path under dest, rejecting
// absolute paths and ".." traversal (with a containment check as a backstop).
func safeTarget(dest, name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("archive: empty entry name")
	}
	slashed := filepath.ToSlash(name)
	if filepath.IsAbs(name) || strings.HasPrefix(slashed, "/") {
		return "", fmt.Errorf("archive: refusing absolute path %q", name)
	}
	clean := filepath.Clean(filepath.FromSlash(name))
	if clean == ".." || strings.HasPrefix(clean, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive: refusing path traversal %q", name)
	}

	target := filepath.Join(dest, clean)
	cleanDest := filepath.Clean(dest)
	if target != cleanDest && !strings.HasPrefix(target, cleanDest+string(os.PathSeparator)) {
		return "", fmt.Errorf("archive: entry %q escapes destination", name)
	}
	return target, nil
}

func writeFile(target string, r io.Reader, perm fs.FileMode) error {
	// target is validated by safeTarget; perm preserves the archived file mode.
	f, err := os.OpenFile(target, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, perm) //nolint:gosec // G302/G304: validated target, mode preserved by design
	if err != nil {
		return fmt.Errorf("archive: create %s: %w", target, err)
	}
	defer func() { _ = f.Close() }()
	if _, err := io.Copy(f, r); err != nil { //nolint:gosec // G110: tar entry size is bounded by the (already-decrypted, local) archive
		return fmt.Errorf("archive: write %s: %w", target, err)
	}
	return nil
}
