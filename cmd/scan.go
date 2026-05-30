package cmd

import "github.com/spf13/cobra"

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Discover candidate local-only files",
	Long:  "Read .gitignore and walk the working tree to suggest candidate files. .gitignore is a hint, not an authority.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
