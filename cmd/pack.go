package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/archive"
	"github.com/vedanta/dew/internal/compress"
	"github.com/vedanta/dew/internal/crypto"
	"github.com/vedanta/dew/internal/deny"
	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
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
	return doPack(root, identity.NewPaths(home), cmd.OutOrStdout())
}

func doPack(root string, p identity.Paths, out io.Writer) error {
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("pack: no manifest found — run 'dew init' first")
		}
		return err
	}

	st, err := identity.Inspect(p)
	if err != nil {
		return err
	}
	if !st.Present {
		return errors.New("pack: no identity found — run 'dew keygen' first")
	}

	if len(m.Allow) == 0 {
		return errors.New("pack: nothing to pack — add files with 'dew add <path>'")
	}
	for _, rel := range m.Allow {
		if _, statErr := os.Stat(filepath.Join(root, filepath.FromSlash(rel))); statErr != nil {
			return fmt.Errorf("pack: allow-listed path %q not found: %w", rel, statErr)
		}
	}

	if err := os.MkdirAll(p.ImagesDir, 0o700); err != nil {
		return fmt.Errorf("pack: %w", err)
	}
	imagePath := filepath.Join(p.ImagesDir, m.Image)
	denied := deny.New(m.Deny)
	skip := func(rel string, isDir bool) bool { return denied.Match(rel, isDir) }
	if err := writeImage(imagePath, p.ImagesDir, m.Image, root, m.Allow, st.PublicKey, skip); err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "Packed %d tracked path(s) → %s\n", len(m.Allow), imagePath)
	return err
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
	rootCmd.AddCommand(packCmd)
}
