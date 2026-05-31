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
	Use:   "keygen",
	Short: "Create the global age identity",
	Long:  "Generate ~/.dew/identity.age.key and identity.age.pub. Refuses to overwrite an existing identity.",
	Args:  cobra.NoArgs,
	RunE:  runKeygen,
}

// keyCmd groups identity inspection subcommands.
var keyCmd = &cobra.Command{
	Use:   "key",
	Short: "Inspect the global age identity",
	Args:  cobra.NoArgs,
}

var keyStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Report whether an identity is present and show its public key",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
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

func init() {
	keyCmd.AddCommand(keyStatusCmd)
	rootCmd.AddCommand(keygenCmd)
	rootCmd.AddCommand(keyCmd)
}
