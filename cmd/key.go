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
	Short:   "Create the global age identity",
	Long: `Generate the one age identity shared across all your repos:
~/.dew/identity.age.key (private, 0600) and identity.age.pub.

This key decrypts your images, so it is never synced or committed. To use dew on
another machine, copy it there yourself. keygen refuses to overwrite an existing
identity.`,
	Args: cobra.NoArgs,
	RunE: runKeygen,
}

// keyCmd groups identity inspection subcommands.
var keyCmd = &cobra.Command{
	Use:     "key",
	GroupID: groupIdentity,
	Short:   "Inspect the global age identity",
	Long:    "Inspect the global age identity. See 'dew key status'.",
	Args:    cobra.NoArgs,
}

var keyStatusCmd = &cobra.Command{
	Use:     "status",
	Short:   "Report whether an identity is present and show its public key",
	Long:    "Report whether a global identity exists and print its public key (derived from the private key if the .pub file is missing).",
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
