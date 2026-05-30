package cmd

import "github.com/spf13/cobra"

var packCmd = &cobra.Command{
	Use:   "pack",
	Short: "Build the encrypted image from allow-listed files",
	Long:  "Package allow-listed files: tar -> zstd -> age encrypt -> ~/.dew/images/<project>.dew.age.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

func init() {
	rootCmd.AddCommand(packCmd)
}
