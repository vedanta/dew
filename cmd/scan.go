package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/manifest"
	"github.com/vedanta/dew/internal/scanner"
)

var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Discover candidate local-only files",
	Long:  "Read .gitignore and walk the working tree to suggest candidate files. .gitignore is a hint, not an authority.",
	Args:  cobra.NoArgs,
	RunE:  runScan,
}

func runScan(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("scan: %w", err)
	}
	return doScan(root, cmd.OutOrStdout())
}

func doScan(root string, out io.Writer) error {
	tracked, extraDeny := manifestHints(root)

	res, err := scanner.Scan(root, extraDeny)
	if err != nil {
		return err
	}

	var b strings.Builder
	if len(res.Candidates) == 0 {
		fmt.Fprintln(&b, "No candidate files found.")
	} else {
		fmt.Fprintln(&b, "Candidates:")
		for _, c := range res.Candidates {
			if tracked[c] {
				fmt.Fprintf(&b, "  %s (already tracked)\n", c)
			} else {
				fmt.Fprintf(&b, "  %s\n", c)
			}
		}
	}
	if len(res.Skipped) > 0 {
		fmt.Fprintln(&b)
		fmt.Fprintln(&b, "Skipped (noise):")
		for _, s := range res.Skipped {
			fmt.Fprintf(&b, "  %s\n", s)
		}
	}

	_, err = io.WriteString(out, b.String())
	return err
}

// manifestHints returns the manifest's allow-list (as a set) and deny patterns,
// or empties if there is no readable manifest (scan works without one).
func manifestHints(root string) (tracked map[string]bool, deny []string) {
	tracked = map[string]bool{}
	if m, err := manifest.Load(manifest.Path(root)); err == nil {
		for _, p := range m.Allow {
			tracked[p] = true
		}
		deny = m.Deny
	}
	return tracked, deny
}

func init() {
	rootCmd.AddCommand(scanCmd)
}
