package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/compress"
	"github.com/vedanta/dew/internal/crypto"
	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
	"github.com/vedanta/dew/internal/restore"
)

var restoreForce bool

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Extract the encrypted image back into the repo",
	Long:  "Restore local files: age decrypt -> zstd decompress -> tar extract. Atomic and non-destructive — files that differ from the image are reported as conflicts and left untouched unless --force is given.",
	Args:  cobra.NoArgs,
	RunE:  runRestore,
}

func runRestore(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("restore: %w", err)
	}
	return doRestore(root, identity.NewPaths(home), restoreForce, cmd.OutOrStdout())
}

func doRestore(root string, p identity.Paths, force bool, out io.Writer) error {
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("restore: no manifest found — run 'dew init' first")
		}
		return err
	}

	st, err := identity.Inspect(p)
	if err != nil {
		return err
	}
	if !st.Present {
		return errors.New("restore: no identity found — run 'dew keygen' first")
	}

	imagePath := filepath.Join(p.ImagesDir, m.Image)
	f, err := os.Open(imagePath) //nolint:gosec // G304: image path is dew-home-local
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("restore: no image at %s — run 'dew pack' (or 'dew sync pull')", imagePath)
		}
		return fmt.Errorf("restore: open image: %w", err)
	}
	defer func() { _ = f.Close() }()

	decR, err := crypto.DecryptReader(f, p.KeyFile)
	if err != nil {
		return err
	}
	zr, err := compress.NewReader(decR)
	if err != nil {
		return err
	}
	defer zr.Close()

	res, err := restore.Restore(zr, root, restore.Options{Force: force})
	if err != nil {
		return err
	}

	if err := reportRestore(out, res); err != nil {
		return err
	}
	if len(res.Conflicts) > 0 && !force {
		return fmt.Errorf("restore: %d file(s) differ from the image; re-run with --force to overwrite", len(res.Conflicts))
	}
	return nil
}

func reportRestore(out io.Writer, res restore.Result) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Restored: %d written, %d unchanged, %d overwritten",
		len(res.Written), len(res.Skipped), len(res.Overwritten))
	if len(res.Conflicts) > 0 {
		fmt.Fprintf(&b, ", %d conflict(s)", len(res.Conflicts))
	}
	fmt.Fprintln(&b)
	for _, c := range res.Conflicts {
		fmt.Fprintf(&b, "  conflict: %s (differs from image; left unchanged)\n", c)
	}
	_, err := io.WriteString(out, b.String())
	return err
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "overwrite local files that differ from the image")
	rootCmd.AddCommand(restoreCmd)
}
