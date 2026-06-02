package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/config"
	"github.com/vedanta/dew/internal/identity"
)

var remoteCmd = &cobra.Command{
	Use:     "remote",
	GroupID: groupSync,
	Short:   "Show or change where dew syncs images",
	Long: `Manage the destination dew pushes images to and pulls them from — a
local/mounted path (e.g. /Volumes/nas/dew) or an scp-style remote
(e.g. nas:/volume1/dew). With no subcommand, 'dew remote' prints the current
destination.

The destination is stored in ~/.dew/config.yaml and shared across all repos.
Once set, 'dew sync' pushes to it and 'dew sync pull' fetches from it.`,
	Example: `  dew remote                     # show the current destination
  dew remote set nas:/vol1/dew   # set it (local path or host:path)
  dew remote unset               # clear it`,
	Args: cobra.NoArgs,
	RunE: runRemoteShow,
}

var remoteSetCmd = &cobra.Command{
	Use:   "set <dest>",
	Short: "Set the sync destination (local path or host:path)",
	Long: `Record where dew syncs this machine's images — a local/mounted path, or an
scp-style 'host:path' remote that uses your existing SSH config. Saved to
~/.dew/config.yaml and shared across all repos; run 'dew sync' afterward to push.
Replaces any existing destination.`,
	Example: `  dew remote set /Volumes/nas/dew   # a local or mounted folder
  dew remote set nas:/volume1/dew   # an scp-style remote`,
	Args: cobra.ExactArgs(1),
	RunE: runRemoteSet,
}

var remoteUnsetCmd = &cobra.Command{
	Use:   "unset",
	Short: "Clear the sync destination",
	Long: `Remove the configured sync destination from ~/.dew/config.yaml. 'dew sync' then
has nowhere to push until you set one again with 'dew remote set'. A no-op if no
destination is configured.`,
	Example: "  dew remote unset",
	Args:    cobra.NoArgs,
	RunE:    runRemoteUnset,
}

func runRemoteShow(cmd *cobra.Command, _ []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	return doRemoteShow(home, cmd.OutOrStdout())
}

func doRemoteShow(home string, out io.Writer) error {
	cfg, err := config.Load(config.Path(home))
	if err != nil {
		return err
	}
	if cfg.Sync.Destination == "" {
		_, werr := io.WriteString(out, "No remote configured. Set one with 'dew remote set <dest>'.\n")
		return werr
	}
	_, err = fmt.Fprintf(out, "%s\n", cfg.Sync.Destination)
	return err
}

func runRemoteSet(cmd *cobra.Command, args []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	return doRemoteSet(home, args[0], cmd.OutOrStdout())
}

func doRemoteSet(home, dest string, out io.Writer) error {
	dest = strings.TrimSpace(dest)
	if dest == "" {
		return errors.New("remote: destination is empty")
	}
	if err := config.SetDestination(config.Path(home), dest); err != nil {
		return err
	}
	_, err := fmt.Fprintf(out, "Remote set to %s\n", dest)
	return err
}

func runRemoteUnset(cmd *cobra.Command, _ []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	return doRemoteUnset(home, cmd.OutOrStdout())
}

func doRemoteUnset(home string, out io.Writer) error {
	if err := config.ClearDestination(config.Path(home)); err != nil {
		return err
	}
	_, err := io.WriteString(out, "Remote cleared.\n")
	return err
}

func init() {
	remoteCmd.AddCommand(remoteSetCmd)
	remoteCmd.AddCommand(remoteUnsetCmd)
	rootCmd.AddCommand(remoteCmd)
}
