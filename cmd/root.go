// Package cmd defines the dew command-line surface. Each command currently
// registers a stub; the real behavior lands per the phased plan in
// docs/build-plan.md.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// version is overridden at build time via -ldflags "-X .../cmd.version=...".
var version = "dev"

// errNotImplemented is returned by command stubs until their phase lands.
var errNotImplemented = errors.New("not implemented yet — see docs/build-plan.md")

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
  dew restore`,
	Version:      version,
	SilenceUsage: true,
}

// Execute runs the root command and exits non-zero on error.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "dew: error:", err)
		os.Exit(1)
	}
}
