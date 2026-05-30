package cmd

import "github.com/spf13/cobra"

// keygenCmd creates the one global age identity shared across all repos.
var keygenCmd = &cobra.Command{
	Use:   "keygen",
	Short: "Create the global age identity",
	Long:  "Generate ~/.dew/identity.age.key and identity.age.pub. Refuses to overwrite an existing identity.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
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

func init() {
	keyCmd.AddCommand(keyStatusCmd)
	rootCmd.AddCommand(keygenCmd)
	rootCmd.AddCommand(keyCmd)
}
