package cmd

import (
	"fmt"
	"io"
	"runtime"

	"github.com/spf13/cobra"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version, commit, and build information",
	Long: `Print dew's version along with the build commit, date, and Go toolchain —
handy when reporting a bug. (See also the --version flag.)`,
	Example: "  dew version",
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, _ []string) error {
		return writeVersion(cmd.OutOrStdout())
	},
}

func writeVersion(out io.Writer) error {
	_, err := fmt.Fprintf(out, "dew %s\n  commit: %s\n  built:  %s\n  go:     %s %s/%s\n",
		version, commit, date, runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return err
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
