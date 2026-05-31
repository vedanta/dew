package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/manifest"
	"github.com/vedanta/dew/internal/scanner"
)

var initFromGitignore bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .dew/manifest.yaml in the current repo",
	Long:  "Create the repo-level dew manifest. With --from-gitignore, seed candidates from .gitignore (discovery lands in Phase 4).",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	return doInit(root, initFromGitignore, cmd.OutOrStdout())
}

// doInit creates a fresh manifest under root. It refuses to clobber an existing
// manifest. The project name defaults to the repo directory name.
func doInit(root string, fromGitignore bool, out io.Writer) error {
	path := manifest.Path(root)

	switch _, err := os.Stat(path); {
	case err == nil:
		return fmt.Errorf("init: manifest already exists at %s", filepath.Join(manifest.Dir, manifest.File))
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("init: %w", err)
	}

	project := filepath.Base(root)
	m := manifest.New(project)

	seeded := 0
	if fromGitignore {
		res, scanErr := scanner.Scan(root)
		if scanErr != nil {
			return scanErr
		}
		for _, c := range res.Candidates {
			if m.AddAllow(c) {
				seeded++
			}
		}
	}

	if err := manifest.Save(path, m); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(out, "Created %s (project %q)\n", filepath.Join(manifest.Dir, manifest.File), project); err != nil {
		return err
	}
	if fromGitignore {
		if _, err := fmt.Fprintf(out, "Seeded %d candidate(s) from .gitignore.\n", seeded); err != nil {
			return err
		}
		for _, p := range m.Allow {
			if _, err := fmt.Fprintf(out, "  %s\n", p); err != nil {
				return err
			}
		}
	}
	return nil
}

func init() {
	initCmd.Flags().BoolVar(&initFromGitignore, "from-gitignore", false, "seed candidates discovered from .gitignore")
	rootCmd.AddCommand(initCmd)
}
