package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/depcheck"
	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/keyxfer"
)

// sshToolHint is shown when ssh/scp aren't installed (only needed by key push).
const sshToolHint = "install the OpenSSH client (e.g. 'brew install openssh' or 'apt-get install openssh-client')"

// keygenCmd creates the one global age identity shared across all repos.
var keygenCmd = &cobra.Command{
	Use:     "keygen",
	GroupID: groupIdentity,
	Short:   "Create your dew identity (the key that encrypts images)",
	Long: `Generate the one age keypair dew uses to encrypt and decrypt your images,
stored in ~/.dew/. Run this once per machine, before you pack anything.

dew never commits or syncs the private key. To set up another machine, run
'dew key push <user@host>' (an explicit, opt-in copy over your SSH access) —
don't run 'dew keygen' there, which would create a different identity that can't
decrypt your images. Guard the key like any private key: without it, images
can't be decrypted. keygen won't overwrite an existing identity. Next:
'dew init' inside a repo.`,
	Example: "  dew keygen",
	Args:    cobra.NoArgs,
	RunE:    runKeygen,
}

// keyCmd groups identity inspection subcommands.
var keyCmd = &cobra.Command{
	Use:     "key",
	GroupID: groupIdentity,
	Short:   "Inspect or provision your dew identity",
	Long:    "Manage your dew identity: inspect it ('dew key status') or provision it onto another machine ('dew key push').",
	Args:    cobra.NoArgs,
}

var keyStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Show whether an identity exists and its public key",
	Long:    "Report whether this machine has a dew identity and print its public key. Use it to confirm a machine is set up before you restore.",
	Args:    cobra.NoArgs,
	Example: "  dew key status",
	RunE:    runKeyStatus,
}

var (
	keyPushForce bool
	keyPushYes   bool
)

var keyPushCmd = &cobra.Command{
	Use:   "push <user@host>",
	Short: "Provision your dew identity onto another machine over SSH",
	Long: `Copy this machine's dew identity to <user@host> so it can decrypt your images —
the one-time bootstrap for a second machine. It uses your existing SSH access
(the host key is verified the normal way; an unknown host aborts), writes the
key 0600 under ~/.dew there, and won't overwrite a different identity without
--force.

This is the one command that moves your private key — and only when you run it,
to a machine you control. 'dew sync' still never transmits the key. Prefer this
over copying the key by hand, and don't run 'dew keygen' on the new machine.`,
	Example: `  dew key push vbarooah@nvk2
  dew key push vbarooah@nvk2 --yes     # skip the confirmation prompt`,
	Args: cobra.ExactArgs(1),
	RunE: runKeyPush,
}

func runKeyPush(cmd *cobra.Command, args []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("key push: %w", err)
	}
	return doKeyPush(identity.NewPaths(home), args[0], keyPushForce, keyPushYes, cmd.InOrStdin(), cmd.OutOrStdout())
}

func doKeyPush(p identity.Paths, host string, force, assumeYes bool, in io.Reader, out io.Writer) error {
	st, err := identity.Inspect(p)
	if err != nil {
		return err
	}
	if !st.Present {
		return errors.New("key push: no identity on this machine — run 'dew keygen' first")
	}

	// Confirm before checking tools, so a decline never depends on ssh/scp.
	if !assumeYes {
		outf(out, "This sends your PRIVATE dew identity (%s) to %s.\n", st.PublicKey, host)
		outf(out, "Anyone with access to that machine can then decrypt all your images.\n")
		if !confirm(in, out, fmt.Sprintf("Push your identity to %s?", host)) {
			return errors.New("key push: cancelled")
		}
	}

	if err := depcheck.RequireTool("ssh", sshToolHint); err != nil {
		return err
	}
	if err := depcheck.RequireTool("scp", sshToolHint); err != nil {
		return err
	}

	outcome, err := keyxfer.Push(host, st.KeyFile, st.PublicKey, force)
	if err != nil {
		if errors.Is(err, keyxfer.ErrDifferentIdentity) {
			return fmt.Errorf("key push: %s already has a different identity; re-run with --force to overwrite it", host)
		}
		return fmt.Errorf("key push: %w", err)
	}

	if outcome == keyxfer.AlreadyPresent {
		outf(out, "%s already has this identity (%s) — nothing to do.\n", host, st.PublicKey)
		return nil
	}
	outf(out, "Provisioned identity (%s) → %s\n", st.PublicKey, host)
	outf(out, "On %s: clone the repo, then 'dew remote set <dest> && dew sync pull && dew restore'.\n", host)
	return nil
}

// confirm prompts for a yes/no answer defaulting to NO (empty input or EOF
// declines) — the safe default for an action that moves your private key.
func confirm(in io.Reader, out io.Writer, prompt string) bool {
	outf(out, "%s [y/N] ", prompt)
	line, _ := bufio.NewReader(in).ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}

func runKeygen(cmd *cobra.Command, _ []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("keygen: %w", err)
	}
	return doKeygen(identity.NewPaths(home), cmd.OutOrStdout())
}

func doKeygen(p identity.Paths, out io.Writer) error {
	pub, err := identity.Generate(p)
	if err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintln(&b, "Created identity")
	fmt.Fprintf(&b, "  Private key: %s\n", p.KeyFile)
	fmt.Fprintf(&b, "  Public key:  %s\n", pub)
	_, err = io.WriteString(out, b.String())
	return err
}

func runKeyStatus(cmd *cobra.Command, _ []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("key status: %w", err)
	}
	return doKeyStatus(identity.NewPaths(home), cmd.OutOrStdout())
}

func doKeyStatus(p identity.Paths, out io.Writer) error {
	s, err := identity.Inspect(p)
	if err != nil {
		return err
	}
	var b strings.Builder
	if s.Present {
		fmt.Fprintln(&b, "Identity: Present")
		fmt.Fprintf(&b, "Private key: %s\n", s.KeyFile)
		fmt.Fprintf(&b, "Public key:  %s\n", s.PublicKey)
	} else {
		fmt.Fprintln(&b, "Identity: Not found")
		fmt.Fprintln(&b, "Run 'dew keygen' to create one.")
	}
	_, err = io.WriteString(out, b.String())
	return err
}

func init() {
	keyPushCmd.Flags().BoolVar(&keyPushForce, "force", false, "overwrite a different identity already on the target")
	keyPushCmd.Flags().BoolVarP(&keyPushYes, "yes", "y", false, "skip the confirmation prompt")
	keyCmd.AddCommand(keyStatusCmd)
	keyCmd.AddCommand(keyPushCmd)
	rootCmd.AddCommand(keygenCmd)
	rootCmd.AddCommand(keyCmd)
}
