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
	restoreImage  string
)

var restoreCmd = &cobra.Command{
	Use:     "restore",
	GroupID: groupImage,
	Short:   "Bring your local files back from the image",
	Long: `Hydrate the working tree from this repo's encrypted image: decrypt, decompress,
and write the tracked files back into place. This is what makes a fresh clone
runnable — pair it with 'dew sync pull' on a new machine.

Safe by default: files are staged first, and any local file that differs from the
image is reported as a conflict and left untouched, so you won't lose local edits
unless you pass --force. --dry-run shows exactly what would change, touching
nothing. ('dew hydrate' is the same command.)

By default the image comes from ~/.dew/images; --image restores from an explicit
.dew.age file instead — one copied over by hand or pulled from a backup. The same
safety rules apply, and no manifest is needed: the image defines what it holds.`,
	Example: `  dew restore             # write the tracked files back into the repo
  dew restore --dry-run   # preview new / unchanged / conflicts; change nothing
  dew restore --force     # overwrite local files that differ from the image
  dew restore --image ~/backups/my-app.dew.age   # restore from an explicit file`,
	Args: cobra.NoArgs,
	RunE: runRestore,
}

// hydrateCmd is dew's signature verb — the same operation as restore, surfaced
// as its own command for discoverability.
var hydrateCmd = &cobra.Command{
	Use:     "hydrate",
	GroupID: groupImage,
	Short:   "Bring your local files back from the image (same as restore)",
	Long: `Hydrate the working tree from this repo's encrypted image — identical to
'dew restore', with the same --force, --dry-run, and --image flags. dew's
signature verb; use whichever name you reach for first.`,
	Example: "  dew hydrate\n  dew hydrate --dry-run",
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
	return doRestore(root, identity.NewPaths(home), restoreForce, restoreDryRun, restoreImage, cmd.OutOrStdout())
}

func doRestore(root string, p identity.Paths, force, dryRun bool, imageOverride string, out io.Writer) error {
	imagePath := imageOverride
	if imagePath == "" {
		// The manifest's only role here is naming the default image; with an
		// explicit --image the file itself defines what is restored.
		m, err := manifest.Load(manifest.Path(root))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return errors.New("restore: no manifest found — run 'dew init' first")
			}
			return err
		}
		imagePath = filepath.Join(p.ImagesDir, m.Image)
	}

	st, err := identity.Inspect(p)
	if err != nil {
		return err
	}
	if !st.Present {
		return errors.New("restore: no identity found — run 'dew keygen' first")
	}

	f, err := os.Open(imagePath) //nolint:gosec // G304: dew-home-local or a user-given --image path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if imageOverride != "" {
				return fmt.Errorf("restore: no image at %s — check the --image path", imagePath)
			}
			return fmt.Errorf("restore: no image at %s — run 'dew pack' (or 'dew sync pull')", imagePath)
		}
		return fmt.Errorf("restore: open image: %w", err)
	}
	defer func() { _ = f.Close() }()

	decR, err := crypto.DecryptReader(f, p.KeyFile)
	if err != nil {
		if errors.Is(err, crypto.ErrWrongIdentity) {
			return errors.New("restore: this image was encrypted to a different identity.\n" +
				"  Copy ~/.dew/identity.age.key from the machine that packed it — dew never syncs\n" +
				"  the key, you carry it. Don't run 'dew keygen' on a new machine; that creates a\n" +
				"  different identity that can't decrypt this image")
		}
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

func addRestoreFlags(c *cobra.Command) {
	c.Flags().BoolVar(&restoreForce, "force", false, "overwrite local files that differ from the image")
	c.Flags().BoolVar(&restoreDryRun, "dry-run", false, "preview what would change; touch nothing")
	c.Flags().StringVar(&restoreImage, "image", "", "restore from this .dew.age file instead of the default under ~/.dew/images")
}

func init() {
	addRestoreFlags(restoreCmd)
	addRestoreFlags(hydrateCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(hydrateCmd)
}
