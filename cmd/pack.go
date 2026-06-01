package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/archive"
	"github.com/vedanta/dew/internal/compress"
	"github.com/vedanta/dew/internal/crypto"
	"github.com/vedanta/dew/internal/deny"
	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
)

var (
	packForce  bool
	packDryRun bool
)

var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Build the encrypted image from allow-listed files",
	Long:  "Package allow-listed files: tar -> zstd -> age encrypt -> ~/.dew/images/<project>.dew.age.",
	Args:  cobra.NoArgs,
	RunE:  runPack,
}

func runPack(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	return doPack(root, identity.NewPaths(home), packForce, packDryRun, cmd.OutOrStdout())
}

func doPack(root string, p identity.Paths, force, dryRun bool, out io.Writer) error {
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("pack: no manifest found — run 'dew init' first")
		}
		return err
	}

	if len(m.Allow) == 0 {
		return errors.New("pack: nothing to pack — add files with 'dew add <path>'")
	}
	for _, rel := range m.Allow {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); statErr != nil {
			return fmt.Errorf("pack: allow-listed path %q not found: %w", rel, statErr)
		}
	}

	denied := deny.New(m.Deny)
	skip := func(rel string, isDir bool) bool { return denied.Match(rel, isDir) }

	if dryRun {
		entries, listErr := archive.List(root, m.Allow, skip)
		if listErr != nil {
			return listErr
		}
		return reportPackDryRun(out, entries)
	}

	st, err := identity.Inspect(p)
	if err != nil {
		return err
	}
	if !st.Present {
		return errors.New("pack: no identity found — run 'dew keygen' first")
	}

	if err := os.MkdirAll(p.ImagesDir, 0o700); err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	imagePath := filepath.Join(p.ImagesDir, m.Image)
	if err := checkImageOwner(imagePath, m.ID, force); err != nil {
		return err
	}

	if err := writeImage(imagePath, p.ImagesDir, m.Image, root, m.Allow, st.PublicKey, skip); err != nil {
		return err
	}
	if err := writeImageOwner(imagePath, m.ID); err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "Packed %d tracked path(s) → %s\n", len(m.Allow), imagePath)
	return err
}

func reportPackDryRun(out io.Writer, entries []archive.Entry) error {
	var b strings.Builder
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	fmt.Fprintf(&b, "Dry run — would pack %d file(s) (%s), no image written:\n", len(entries), humanBytes(total))
	for _, e := range entries {
		fmt.Fprintf(&b, "  %s (%s)\n", e.Name, humanBytes(e.Size))
	}
	_, err := io.WriteString(out, b.String())
	return err
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for x := n / unit; x >= unit; x /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGT"[exp])
}

func ownerMarkerPath(imagePath string) string { return imagePath + ".id" }

// checkImageOwner refuses to overwrite an image created by a different repo
// (its ownership marker holds a different manifest id), unless force is set. An
// unmarked image, a matching id, or an id-less manifest is allowed through.
func checkImageOwner(imagePath, manifestID string, force bool) error {
	if manifestID == "" || force {
		return nil
	}
	owner, err := readImageOwner(imagePath)
	if err != nil {
		return err
	}
	if owner != "" && owner != manifestID {
		return fmt.Errorf("pack: %s was created by a different repo; use 'dew init --project <name>' for a unique name, or --force to overwrite it", filepath.Base(imagePath))
	}
	return nil
}

func readImageOwner(imagePath string) (string, error) {
	b, err := os.ReadFile(ownerMarkerPath(imagePath)) //nolint:gosec // G304: marker path is dew-home-local
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil
		}
		return "", fmt.Errorf("pack: read image owner: %w", err)
	}
	return strings.TrimSpace(string(b)), nil
}

func writeImageOwner(imagePath, manifestID string) error {
	if manifestID == "" {
		return nil
	}
	if err := os.WriteFile(ownerMarkerPath(imagePath), []byte(manifestID+"\n"), 0o600); err != nil {
		return fmt.Errorf("pack: write image owner: %w", err)
	}
	return nil
}

// writeImage builds the tar -> zstd -> age pipeline into a temp file, then
// renames it into place so a failed pack never leaves a half-written image.
func writeImage(imagePath, imagesDir, name, root string, allow []string, recipient string, skip func(rel string, isDir bool) bool) (err error) {
	tmp, err := os.CreateTemp(imagesDir, name+".tmp-*")
	if err != nil {
		return fmt.Errorf("pack: create temp image: %w", err)
	}
	tmpName := tmp.Name()
	// Clean up the temp file on any failure path.
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
		}
	}()

	encW, err := crypto.EncryptWriter(tmp, recipient)
	if err != nil {
		return err
	}
	zw, err := compress.NewWriter(encW)
	if err != nil {
		return err
	}
	if err = archive.Build(zw, root, allow, skip); err != nil {
		return err
	}
	if err = zw.Close(); err != nil {
		return fmt.Errorf("pack: finalize compression: %w", err)
	}
	if err = encW.Close(); err != nil {
		return fmt.Errorf("pack: finalize encryption: %w", err)
	}
	if err = tmp.Close(); err != nil {
		return fmt.Errorf("pack: close temp image: %w", err)
	}
	if err = os.Rename(tmpName, imagePath); err != nil {
		return fmt.Errorf("pack: install image: %w", err)
	}
	return nil
}

func init() {
	packCmd.Flags().BoolVar(&packForce, "force", false, "overwrite an image created by a different repo")
	packCmd.Flags().BoolVar(&packDryRun, "dry-run", false, "list what would be packed without writing an image")
	rootCmd.AddCommand(packCmd)
}
