package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/manifest"
)

var listCmd = &cobra.Command{
	Use:     "list",
	Aliases: []string{"ls"},
	GroupID: groupRepo,
	Short:   "List the files dew tracks for this repo (alias: ls)",
	Long: `Show this repo's allow-list — the local files dew manages — and the project
name. These are the paths 'dew pack' will include. To see the deny rules by
layer as well, use 'dew rules'.`,
	Example: "  dew list",
	Args:    cobra.NoArgs,
	RunE:    runList,
}

func runList(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("list: %w", err)
	}
	return doList(root, cmd.OutOrStdout())
}

func doList(root string, out io.Writer) error {
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("list: no manifest found — run 'dew init' first")
		}
		return err
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Project: %s\n\n", m.Project)
	if len(m.Allow) == 0 {
		fmt.Fprintln(&b, "Tracked: (none) — add files with 'dew add <path>'")
	} else {
		fmt.Fprintln(&b, "Tracked:")
		for _, p := range m.Allow {
			fmt.Fprintf(&b, "  %s\n", p)
		}
	}

	_, err = io.WriteString(out, b.String())
	return err
}

func init() {
	rootCmd.AddCommand(listCmd)
}
