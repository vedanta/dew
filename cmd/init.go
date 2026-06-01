package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
	"github.com/vedanta/dew/internal/scanner"
)

var (
	initFromGitignore bool
	initProject       string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Create .dew/manifest.yaml in the current repo",
	Long:  "Create the repo-level dew manifest. The project name defaults to the directory name; override it with --project. With --from-gitignore, seed candidates from .gitignore.",
	Args:  cobra.NoArgs,
	RunE:  runInit,
}

func runInit(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("init: %w", err)
	}
	if err := loadGlobalDeny(); err != nil {
		return fmt.Errorf("init: %w", err)
	}
	return doInit(root, initProject, initFromGitignore, identity.NewPaths(home).ImagesDir, cmd.OutOrStdout())
}

// doInit creates a fresh manifest under root. It refuses to clobber an existing
// manifest. The project name comes from the --project value, or defaults to the
// repo directory name. imagesDir (may be empty) is used only for the
// name-collision warning.
func doInit(root, projectFlag string, fromGitignore bool, imagesDir string, out io.Writer) error {
	path := manifest.Path(root)

	switch _, err := os.Stat(path); {
	case err == nil:
		return fmt.Errorf("init: manifest already exists at %s", filepath.Join(manifest.Dir, manifest.File))
	case !errors.Is(err, os.ErrNotExist):
		return fmt.Errorf("init: %w", err)
	}

	project := projectFlag
	fromFolder := project == ""
	if fromFolder {
		project = filepath.Base(root)
	}
	if err := manifest.ValidateProjectName(project); err != nil {
		if fromFolder {
			return fmt.Errorf("init: could not use the directory name as the project: %w — pass --project <name>", err)
		}
		return fmt.Errorf("init: %w", err)
	}
	m := manifest.New(project)
	id, err := manifest.NewID()
	if err != nil {
		return err
	}
	m.ID = id

	seeded := 0
	if fromGitignore {
		// A fresh manifest has no deny: rules yet, so only built-in + global apply.
		res, scanErr := scanner.Scan(root, mergeDeny(globalDenyPatterns, m.Deny))
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
	if imagesDir != "" && fileExists(filepath.Join(imagesDir, m.Image)) {
		if _, err := fmt.Fprintf(out, "warning: an image named %s already exists in %s — another repo may use this name; consider 'dew init --project <name>'\n", m.Image, imagesDir); err != nil {
			return err
		}
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
	initCmd.Flags().StringVarP(&initProject, "project", "p", "", "project name (defaults to the directory name)")
	rootCmd.AddCommand(initCmd)
}
