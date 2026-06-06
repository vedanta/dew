package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
)

var (
	cleanForce        bool
	cleanYes          bool
	cleanImageOnly    bool
	cleanManifestOnly bool
)

var cleanCmd = &cobra.Command{
	Use:     "clean",
	GroupID: groupRepo,
	Short:   "Remove dew's footprint for this repo (manifest + packed image)",
	Long: `Tear down what dew keeps for this repo: the committed .dew/ manifest and this
repo's encrypted image in ~/.dew/images. It is the inverse of 'dew init' plus
'dew pack' — run it when you're done using dew here, or to start over clean.

This is local-only and never touches your identity key (it's shared by every
repo) or any copy you've synced to a remote or another machine. dew has no
version history, so removal is permanent — but the image is just a repack of
files still on disk, and the manifest is normally committed to Git, so the
common case is recoverable. You are asked to confirm unless you pass --force.

Scope it with --image-only (drop the image, keep tracking config so the next
'dew pack' rebuilds it) or --manifest-only (stop managing the repo but keep the
image around). To delete an image whose repo is already gone, use
'dew images rm <project>' instead.`,
	Example: `  dew clean                  # remove the manifest and this repo's image (asks first)
  dew clean --force          # remove both without confirming
  dew clean --image-only     # drop the image; keep the manifest for a clean re-pack
  dew clean --manifest-only  # stop managing this repo; keep the packed image`,
	Args: cobra.NoArgs,
	RunE: runClean,
}

func runClean(cmd *cobra.Command, _ []string) error {
	if cleanImageOnly && cleanManifestOnly {
		return errors.New("clean: --image-only and --manifest-only are mutually exclusive")
	}
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("clean: %w", err)
	}
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("clean: %w", err)
	}
	// --force implies skipping the prompt; --yes skips it while keeping the guard.
	return doClean(root, identity.NewPaths(home), cleanForce, cleanForce || cleanYes,
		cleanImageOnly, cleanManifestOnly, cmd.InOrStdin(), cmd.OutOrStdout())
}

func doClean(root string, p identity.Paths, force, assumeYes, imageOnly, manifestOnly bool, in io.Reader, out io.Writer) error {
	manifestPath := manifest.Path(root)
	m, err := manifest.Load(manifestPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("clean: no manifest here — run this from a repo set up with 'dew init' (to delete a stray image, use 'dew images rm <project>')")
		}
		return err
	}

	delImage := !manifestOnly
	delManifest := !imageOnly
	imagePath := filepath.Join(p.ImagesDir, m.Image)

	// Refuse to delete an image another repo owns, unless forced — the same guard
	// 'dew pack' uses, so a name collision can't nuke a different repo's image.
	if delImage && !force && m.ID != "" {
		owner, ownErr := readImageOwner(imagePath)
		if ownErr != nil {
			return ownErr
		}
		if owner != "" && owner != m.ID {
			return fmt.Errorf("clean: %s belongs to a different repo (owner %s); re-run with --force to remove it anyway", m.Image, shortID(owner))
		}
	}

	// Build the list of things that actually exist, for an accurate prompt.
	var targets []string
	if delImage && fileExists(imagePath) {
		targets = append(targets, "image     "+imagePath)
	}
	if delManifest && fileExists(manifestPath) {
		targets = append(targets, "manifest  "+manifestPath)
	}
	if len(targets) == 0 {
		_, werr := io.WriteString(out, "Nothing to clean.\n")
		return werr
	}

	if !assumeYes {
		outf(out, "This will permanently remove:\n")
		for _, t := range targets {
			outf(out, "  - %s\n", t)
		}
		if delImage {
			outf(out, "The image is dew's only local copy of your packed files; 'dew pack' can rebuild it from the tracked files.\n")
		}
		if !confirm(in, out, "Remove these?") {
			return errors.New("clean: cancelled")
		}
	}

	var b strings.Builder
	if delImage {
		if _, rmErr := removeImageFile(imagePath); rmErr != nil {
			return fmt.Errorf("clean: %w", rmErr)
		}
		fmt.Fprintf(&b, "removed image %s\n", imagePath)
	}
	if delManifest {
		if rmErr := removeManifestDir(manifestPath); rmErr != nil {
			return fmt.Errorf("clean: %w", rmErr)
		}
		fmt.Fprintf(&b, "removed manifest %s\n", manifestPath)
	}
	if delManifest {
		b.WriteString("Cleaned — dew no longer manages this repo.\n")
	} else {
		b.WriteString("Removed the image; the manifest still tracks this repo (run 'dew pack' to rebuild).\n")
	}
	_, werr := io.WriteString(out, b.String())
	return werr
}

// removeImageFile deletes an image and its .id owner marker. A missing file is
// not an error, so cleanup is idempotent. It reports whether the image existed.
func removeImageFile(imagePath string) (existed bool, err error) {
	existed = fileExists(imagePath)
	if rmErr := os.Remove(imagePath); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return existed, fmt.Errorf("remove image: %w", rmErr)
	}
	if rmErr := os.Remove(ownerMarkerPath(imagePath)); rmErr != nil && !errors.Is(rmErr, os.ErrNotExist) {
		return existed, fmt.Errorf("remove image marker: %w", rmErr)
	}
	return existed, nil
}

// removeManifestDir deletes the manifest file and the explanatory README dew
// wrote beside it, then removes the .dew directory if that leaves it empty. A
// .dew holding other files (unexpected, but possible) is left in place.
func removeManifestDir(manifestPath string) error {
	if err := os.Remove(manifestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove manifest: %w", err)
	}
	readme := filepath.Join(filepath.Dir(manifestPath), manifest.Readme)
	if err := os.Remove(readme); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove manifest readme: %w", err)
	}
	// Best-effort: os.Remove on a directory only succeeds when it is empty.
	_ = os.Remove(filepath.Dir(manifestPath))
	return nil
}

func init() {
	cleanCmd.Flags().BoolVar(&cleanForce, "force", false, "remove without confirming, and override the image-owner guard")
	cleanCmd.Flags().BoolVarP(&cleanYes, "yes", "y", false, "skip the confirmation prompt (still respects the owner guard)")
	cleanCmd.Flags().BoolVar(&cleanImageOnly, "image-only", false, "remove only the packed image; keep the manifest")
	cleanCmd.Flags().BoolVar(&cleanManifestOnly, "manifest-only", false, "remove only the manifest; keep the packed image")
	rootCmd.AddCommand(cleanCmd)
}
