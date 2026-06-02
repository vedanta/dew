package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/identity"
)

// keygenCmd creates the one global age identity shared across all repos.
var keygenCmd = &cobra.Command{
	Use:     "keygen",
	GroupID: groupIdentity,
	Short:   "Create your dew identity (the key that encrypts images)",
	Long: `Generate the one age keypair dew uses to encrypt and decrypt your images,
stored in ~/.dew/. Run this once per machine, before you pack anything.

The private key never leaves your machine — dew never commits or syncs it — so to
use dew on another machine you copy ~/.dew/identity.age.key there yourself. Guard
it like any private key: without it, images can't be decrypted. keygen won't
overwrite an existing identity. Next: 'dew init' inside a repo.`,
	Example: "  dew keygen",
	Args:    cobra.NoArgs,
	RunE:    runKeygen,
}

// keyCmd groups identity inspection subcommands.
var keyCmd = &cobra.Command{
	Use:     "key",
	GroupID: groupIdentity,
	Short:   "Inspect your dew identity",
	Long:    "Inspect your dew identity. See 'dew key status'.",
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
	keyCmd.AddCommand(keyStatusCmd)
	rootCmd.AddCommand(keygenCmd)
	rootCmd.AddCommand(keyCmd)
}
