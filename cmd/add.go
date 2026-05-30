package cmd

import "github.com/spf13/cobra"

var addCmd = &cobra.Command{
	Use:   "add <path>",
	Short: "Add a file or directory to the manifest allow-list",
	Long:  "Add a path to the allow-list. 'dew add .' adds discovered candidates — not every file in the repo.",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

func init() {
	rootCmd.AddCommand(addCmd)
}
