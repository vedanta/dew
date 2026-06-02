package cmd

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/config"
	"github.com/vedanta/dew/internal/identity"
	dewsync "github.com/vedanta/dew/internal/sync"
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

var remoteTestCmd = &cobra.Command{
	Use:   "test",
	Short: "Check the destination is reachable, trusted, and writable",
	Long: `Verify the configured destination is actually usable before you rely on
'dew sync'. A local/mounted path is checked in place (is the volume mounted? is
it writable?). A remote 'host:path' is checked over ssh — reachable, host key
trusted, and the path writable — reporting OpenSSH's own verdict (it never
prompts, so an untrusted host key fails cleanly). Exits non-zero if unusable.`,
	Example: "  dew remote test",
	Args:    cobra.NoArgs,
	RunE:    runRemoteTest,
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

func runRemoteTest(cmd *cobra.Command, _ []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("remote: %w", err)
	}
	return doRemoteTest(home, cmd.OutOrStdout())
}

func doRemoteTest(home string, out io.Writer) error {
	cfg, err := config.Load(config.Path(home))
	if err != nil {
		return err
	}
	if cfg.Sync.Destination == "" {
		return errors.New("remote: no remote configured — set one with 'dew remote set <dest>'")
	}

	res, err := dewsync.Probe(cfg.Sync.Destination)
	if err != nil {
		return err
	}

	var b strings.Builder
	kind := "local"
	if res.Remote {
		kind = "remote"
	}
	fmt.Fprintf(&b, "Testing %s destination %s\n", kind, res.Destination)
	for _, c := range res.Checks {
		mark := "ok  "
		if !c.OK {
			mark = "FAIL"
		}
		if c.Note != "" {
			fmt.Fprintf(&b, "  %s  %s — %s\n", mark, c.Name, c.Note)
		} else {
			fmt.Fprintf(&b, "  %s  %s\n", mark, c.Name)
		}
	}
	if res.OK() {
		fmt.Fprintln(&b, "\nDestination is usable.")
	}
	if _, werr := io.WriteString(out, b.String()); werr != nil {
		return werr
	}
	if !res.OK() {
		return errors.New("remote: destination is not usable (see above)")
	}
	return nil
}

func init() {
	remoteCmd.AddCommand(remoteSetCmd)
	remoteCmd.AddCommand(remoteUnsetCmd)
	remoteCmd.AddCommand(remoteTestCmd)
	rootCmd.AddCommand(remoteCmd)
}
