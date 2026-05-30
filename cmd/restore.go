package cmd

import "github.com/spf13/cobra"

var restoreCmd = &cobra.Command{
	Use:   "restore",
	Short: "Extract the encrypted image back into the repo",
	Long:  "Restore local files: age decrypt -> zstd decompress -> tar extract. Atomic and non-destructive — compares before overwriting.",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

func init() {
	rootCmd.AddCommand(restoreCmd)
}
