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
	Use:     "init",
	GroupID: groupRepo,
	Short:   "Set up dew in this repo (creates .dew/manifest.yaml)",
	Long: `Start managing this repo's local context with dew. Creates .dew/manifest.yaml —
the list of which local files dew should manage — plus a short .dew/README.md
that explains the directory to anyone browsing the repo. The manifest holds no
secrets or file contents, only paths, so it's safe to commit; committing it
means teammates and other machines know what to hydrate.

The project name defaults to the directory name (override with --project); it
becomes the image name. With --from-gitignore, dew seeds the allow-list with
candidates discovered from your .gitignore so you can go straight to pack.
Next: 'dew add <path>' (or 'dew scan') to choose what to manage, then 'dew pack'.`,
	Example: `  dew init                        # create the manifest (project = folder name)
  dew init --project billing-svc  # name the project independently of the folder
  dew init --from-gitignore       # seed the allow-list from .gitignore`,
	Args: cobra.NoArgs,
	RunE: runInit,
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
	if err := writeManifestReadme(root); err != nil {
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

// manifestReadme explains the committed .dew/ directory to anyone browsing the
// repo (GitHub renders it in the directory listing). It carries no secrets.
const manifestReadme = "# .dew/\n" + `
This directory is managed by [**dew**](https://github.com/vedanta/dew) — a
local-first CLI for the private, local files a repository needs but Git
intentionally ignores (` + "`.env.local`" + `, dev certs,
` + "`docker-compose.override.yml`" + `, local config, …).

- **` + "`manifest.yaml`" + `** lists which local files dew tracks for this repo.
  It holds only paths and rules — never secrets, file contents, or keys — so it
  is safe to commit. The actual file contents live in an encrypted image outside
  the repo (` + "`~/.dew/images/`" + `), never in Git.
- **You don't need dew to use this repo.** This directory is just metadata; if
  you don't use dew you can ignore it. To restore the local files dew manages:

  ` + "```bash" + `
  dew sync pull   # fetch the encrypted image
  dew restore     # write the local files back into the working tree
  ` + "```" + `

Learn more at <https://github.com/vedanta/dew>.
`

// writeManifestReadme writes .dew/README.md, leaving an existing one untouched
// so a user's edits (or a deliberate deletion + re-init) are respected.
func writeManifestReadme(root string) error {
	path := manifest.ReadmePath(root)
	if fileExists(path) {
		return nil
	}
	if err := os.WriteFile(path, []byte(manifestReadme), 0o644); err != nil { //nolint:gosec // G306: non-secret, committed to Git
		return fmt.Errorf("init: write %s: %w", filepath.Join(manifest.Dir, manifest.Readme), err)
	}
	return nil
}

func init() {
	initCmd.Flags().BoolVar(&initFromGitignore, "from-gitignore", false, "seed the allow-list with candidates found in .gitignore")
	initCmd.Flags().StringVarP(&initProject, "project", "p", "", "name this project (defaults to the directory name)")
	rootCmd.AddCommand(initCmd)
}
