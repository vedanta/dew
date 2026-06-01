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

var (
	restoreForce  bool
	restoreDryRun bool
)

var restoreCmd = &cobra.Command{
	Use:     "restore",
	Aliases: []string{"hydrate"},
	Short:   "Extract the encrypted image back into the repo (alias: hydrate)",
	Long:    "Restore local files: age decrypt -> zstd decompress -> tar extract. Atomic and non-destructive — files that differ from the image are reported as conflicts and left untouched unless --force is given. Also available as 'dew hydrate'.",
	Args:    cobra.NoArgs,
	RunE:    runRestore,
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
	return doRestore(root, identity.NewPaths(home), restoreForce, restoreDryRun, cmd.OutOrStdout())
}

func doRestore(root string, p identity.Paths, force, dryRun bool, out io.Writer) error {
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

	res, err := restore.Restore(zr, root, restore.Options{Force: force, DryRun: dryRun})
	if err != nil {
		return err
	}

	if err := reportRestore(out, res, dryRun); err != nil {
		return err
	}
	// A dry run is a preview; only a real run with unresolved conflicts errors.
	if !dryRun && len(res.Conflicts) > 0 && !force {
		return fmt.Errorf("restore: %d file(s) differ from the image; re-run with --force to overwrite", len(res.Conflicts))
	}
	return nil
}

func reportRestore(out io.Writer, res restore.Result, dryRun bool) error {
	var b strings.Builder
	verb := "Restored"
	if dryRun {
		fmt.Fprintln(&b, "Dry run — no files changed.")
		verb = "Would restore"
	}
	fmt.Fprintf(&b, "%s: %d written, %d unchanged, %d overwritten",
		verb, len(res.Written), len(res.Skipped), len(res.Overwritten))
	if len(res.Conflicts) > 0 {
		fmt.Fprintf(&b, ", %d conflict(s)", len(res.Conflicts))
	}
	fmt.Fprintln(&b)
	if dryRun {
		for _, w := range res.Written {
			fmt.Fprintf(&b, "  new:        %s\n", w)
		}
		for _, o := range res.Overwritten {
			fmt.Fprintf(&b, "  overwrite:  %s\n", o)
		}
	}
	for _, c := range res.Conflicts {
		fmt.Fprintf(&b, "  conflict:   %s (differs from image)\n", c)
	}
	_, err := io.WriteString(out, b.String())
	return err
}

func init() {
	restoreCmd.Flags().BoolVar(&restoreForce, "force", false, "overwrite local files that differ from the image")
	restoreCmd.Flags().BoolVar(&restoreDryRun, "dry-run", false, "preview what would be restored without changing the working tree")
	rootCmd.AddCommand(restoreCmd)
}
