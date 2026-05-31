package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/config"
	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
	dewsync "github.com/vedanta/dew/internal/sync"
)

// syncCmd with no subcommand pushes the current repo's image to the configured
// destination. 'dew sync pull' fetches it back. Sync moves encrypted images
// only — never the private key.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Push the current repo's encrypted image to the sync destination",
	Args:  cobra.NoArgs,
	RunE:  runSyncPush,
}

var syncPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull the encrypted image from the sync destination",
	Args:  cobra.NoArgs,
	RunE:  runSyncPull,
}

func runSyncPush(cmd *cobra.Command, _ []string) error {
	root, p, dest, err := syncContext()
	if err != nil {
		return err
	}
	return doSyncPush(root, p, dest, cmd.OutOrStdout())
}

func doSyncPush(root string, p identity.Paths, destination string, out io.Writer) error {
	m, err := loadForSync(root, destination)
	if err != nil {
		return err
	}
	localImage := filepath.Join(p.ImagesDir, m.Image)
	if !fileExists(localImage) {
		return fmt.Errorf("sync: no image at %s — run 'dew pack' first", localImage)
	}
	if err := dewsync.Push(localImage, destination); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Pushed %s → %s\n", m.Image, destination)
	return err
}

func runSyncPull(cmd *cobra.Command, _ []string) error {
	root, p, dest, err := syncContext()
	if err != nil {
		return err
	}
	return doSyncPull(root, p, dest, cmd.OutOrStdout())
}

func doSyncPull(root string, p identity.Paths, destination string, out io.Writer) error {
	m, err := loadForSync(root, destination)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(p.ImagesDir, 0o700); err != nil {
		return fmt.Errorf("sync: %w", err)
	}
	localImage := filepath.Join(p.ImagesDir, m.Image)
	if err := dewsync.Pull(localImage, destination); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Pulled %s from %s\nRun 'dew restore' to hydrate the repo.\n", m.Image, destination)
	return err
}

// syncContext resolves the repo root, identity paths, and configured sync
// destination shared by push and pull.
func syncContext() (string, identity.Paths, string, error) {
	root, err := os.Getwd()
	if err != nil {
		return "", identity.Paths{}, "", fmt.Errorf("sync: %w", err)
	}
	home, err := identity.DefaultHome()
	if err != nil {
		return "", identity.Paths{}, "", fmt.Errorf("sync: %w", err)
	}
	cfg, err := config.Load(config.Path(home))
	if err != nil {
		return "", identity.Paths{}, "", err
	}
	return root, identity.NewPaths(home), cfg.Sync.Destination, nil
}

// loadForSync validates a destination is configured and loads the manifest.
func loadForSync(root, destination string) (*manifest.Manifest, error) {
	if destination == "" {
		return nil, errors.New("sync: no destination configured — set sync.destination in ~/.dew/config.yaml")
	}
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, errors.New("sync: no manifest found — run 'dew init' first")
		}
		return nil, err
	}
	return m, nil
}

func init() {
	syncCmd.AddCommand(syncPullCmd)
	rootCmd.AddCommand(syncCmd)
}
