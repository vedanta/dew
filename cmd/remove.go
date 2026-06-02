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

var removeCmd = &cobra.Command{
	Use:     "remove <path>...",
	Aliases: []string{"rm"},
	GroupID: groupRepo,
	Short:   "Stop tracking a path (alias: rm)",
	Long:    "Remove one or more paths from this repo's allow-list so dew no longer manages them. Removing a path that isn't tracked is a harmless no-op. The next 'dew pack' reflects the change.",
	Example: "  dew remove .env.local",
	Args:    cobra.MinimumNArgs(1),
	RunE:    runRemove,
}

func runRemove(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("remove: %w", err)
	}
	return doRemove(root, args, cmd.OutOrStdout())
}

func doRemove(root string, args []string, out io.Writer) error {
	path := manifest.Path(root)
	m, err := manifest.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("remove: no manifest found — run 'dew init' first")
		}
		return err
	}

	var b strings.Builder
	changed := false
	for _, arg := range args {
		rel, relErr := repoRelPath(root, arg)
		if relErr != nil {
			return fmt.Errorf("remove: %w", relErr)
		}
		if m.RemoveAllow(rel) {
			fmt.Fprintf(&b, "removed %s\n", rel)
			changed = true
		} else {
			fmt.Fprintf(&b, "not tracked: %s\n", rel)
		}
	}

	if changed {
		if err := manifest.Save(path, m); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, b.String())
	return err
}

func init() {
	rootCmd.AddCommand(removeCmd)
}
