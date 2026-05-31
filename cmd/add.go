package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/manifest"
)

var addCmd = &cobra.Command{
	Use:   "add <path>...",
	Short: "Add a file or directory to the manifest allow-list",
	Long:  "Add one or more paths to the allow-list. ('dew add .' for discovered candidates arrives in Phase 4 — it does not mean every file in the repo.)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAdd,
}

func runAdd(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("add: %w", err)
	}
	return doAdd(root, args, cmd.OutOrStdout())
}

func doAdd(root string, args []string, out io.Writer) error {
	path := manifest.Path(root)
	m, err := manifest.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("add: no manifest found — run 'dew init' first")
		}
		return err
	}

	var b strings.Builder
	changed := false
	for _, arg := range args {
		rel, relErr := repoRelPath(root, arg)
		if relErr != nil {
			return fmt.Errorf("add: %w", relErr)
		}
		if _, statErr := os.Stat(filepath.Join(root, rel)); errors.Is(statErr, os.ErrNotExist) {
			fmt.Fprintf(&b, "warning: %s does not exist yet\n", rel)
		}
		if m.AddAllow(rel) {
			fmt.Fprintf(&b, "added %s\n", rel)
			changed = true
		} else {
			fmt.Fprintf(&b, "already tracked: %s\n", rel)
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
	rootCmd.AddCommand(addCmd)
}
