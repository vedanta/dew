package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// denyDirs are directory names that are always treated as noise and never
// suggested, even if .gitignore would ignore them.
var denyDirs = map[string]bool{
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".venv":        true,
	"__pycache__":  true,
}

// Result is the outcome of scanning a repo for candidate local-only files.
type Result struct {
	// Candidates are git-ignored, non-noise files worth tracking (repo-relative,
	// slash-separated).
	Candidates []string
	// Skipped lists noise paths that were excluded (deny-listed dirs reported
	// once with a trailing slash).
	Skipped []string
}

// Scan reads root/.gitignore and walks the working tree, classifying each path
// as a candidate (git-ignored and not noise) or skipped (noise). .gitignore is
// only a hint: it never makes a path eligible on its own without the user
// opting in. If there is no .gitignore, there are no candidates.
func Scan(root string) (Result, error) {
	var res Result

	ign, err := loadGitignore(filepath.Join(root, ".gitignore"))
	if err != nil {
		return res, err
	}

	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		slash := filepath.ToSlash(rel)

		if d.IsDir() {
			return classifyDir(d.Name(), slash, &res)
		}
		classifyFile(d.Name(), slash, ign, &res)
		return nil
	})
	if walkErr != nil {
		return res, walkErr
	}

	sort.Strings(res.Candidates)
	sort.Strings(res.Skipped)
	return res, nil
}

func classifyDir(name, slash string, res *Result) error {
	// dew's own internals and VCS metadata are never scanned.
	if name == ".git" || name == ".dew" {
		return filepath.SkipDir
	}
	if denyDirs[name] {
		res.Skipped = append(res.Skipped, slash+"/")
		return filepath.SkipDir
	}
	return nil
}

func classifyFile(name, slash string, ign *gitignore.GitIgnore, res *Result) {
	// .gitignore itself is tracked by Git; never suggest it.
	if name == ".gitignore" {
		return
	}
	if isNoiseFile(name) {
		res.Skipped = append(res.Skipped, slash)
		return
	}
	if ign != nil && ign.MatchesPath(slash) {
		res.Candidates = append(res.Candidates, slash)
	}
}

func isNoiseFile(name string) bool {
	return name == ".DS_Store" || strings.HasSuffix(name, ".log")
}

func loadGitignore(path string) (*gitignore.GitIgnore, error) {
	ign, err := gitignore.CompileIgnoreFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("scanner: read .gitignore: %w", err)
	}
	return ign, nil
}
