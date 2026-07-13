// Package cmd defines the dew command-line surface.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Build metadata, overridden at build time via -ldflags "-X .../cmd.<var>=...".
// GoReleaser sets all three on a tagged release; a plain `go build` leaves the
// defaults.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var rootCmd = &cobra.Command{
	Use:   "dew",
	Short: "Manage the private local files Git intentionally ignores",
	Long: `dew is a local-first CLI that manages the private repository state Git
intentionally ignores (.env.local, dev certs, docker-compose.override.yml,
private fixtures, local config).

It packages an allow-listed set of files into a single encrypted image per repo
and can sync that image to a remote, so a fresh clone can be hydrated back to a
working state:

  git clone <repo> && cd <repo>
  dew sync pull
  dew restore

dew complements Git — it never touches your tracked source; it carries the local
context Git is meant to ignore. Run 'dew <command> --help' for any command.
Guide: https://vedanta.github.io/dew/`,
	Example: `  # First time on a machine
  dew keygen

  # Set up a repo and push it
  dew init
  dew add .env.local certs/
  dew pack
  dew remote set nas:/volume1/dew
  dew sync

  # Hydrate a fresh clone elsewhere
  dew sync pull
  dew restore`,
	Version:      version,
	SilenceUsage: true,
}

// Command group IDs, used to organize 'dew --help'.
const (
	groupIdentity = "identity"
	groupRepo     = "repo"
	groupImage    = "image"
	groupSync     = "sync"
	groupHealth   = "health"
)

func init() {
	rootCmd.AddGroup(
		&cobra.Group{ID: groupIdentity, Title: "Identity:"},
		&cobra.Group{ID: groupRepo, Title: "Repository:"},
		&cobra.Group{ID: groupImage, Title: "Image:"},
		&cobra.Group{ID: groupSync, Title: "Sync:"},
		&cobra.Group{ID: groupHealth, Title: "Health & inventory:"},
	)
}

// Root returns the fully-assembled root command. Exposed so the docs generator
// (tools/gendocs) can walk the real command tree — the reference site is built
// from these definitions, so it can never drift from 'dew --help'.
func Root() *cobra.Command { return rootCmd }

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "dew: error:", err)
		os.Exit(1)
	}
}
