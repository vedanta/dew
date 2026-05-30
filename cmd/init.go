package cmd

import "github.com/spf13/cobra"

var initFromGitignore bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .dew/manifest.yaml in the current repo",
	Long:  "Create the repo-level dew manifest. With --from-gitignore, seed candidates from .gitignore (discovery lands in Phase 4).",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return errNotImplemented
	},
}

func init() {
	initCmd.Flags().BoolVar(&initFromGitignore, "from-gitignore", false, "seed candidates discovered from .gitignore")
	rootCmd.AddCommand(initCmd)
}
