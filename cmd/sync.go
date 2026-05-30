package cmd

import "github.com/spf13/cobra"

// syncCmd with no subcommand pushes the current repo's image to the configured
// destination. 'dew sync pull' fetches it back. Sync moves encrypted images
// only — never the private key.
var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Push the current repo's encrypted image to the sync destination",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

var syncPullCmd = &cobra.Command{
	Use:   "pull",
	Short: "Pull the encrypted image from the sync destination",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

func init() {
	syncCmd.AddCommand(syncPullCmd)
	rootCmd.AddCommand(syncCmd)
}
