package scanner

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	gitignore "github.com/sabhiram/go-gitignore"

	"github.com/vedanta/dew/internal/deny"
)

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
// as a candidate (git-ignored and not denied) or skipped (denied noise). The
// deny set is the built-in patterns plus extraDeny (per-manifest deny: rules).
// .gitignore is only a hint: it never makes a path eligible without the user
// opting in, and an absent .gitignore yields no candidates.
func Scan(root string, extraDeny []string) (Result, error) {
	var res Result

	ign, err := loadGitignore(filepath.Join(root, ".gitignore"))
	if err != nil {
		return res, err
	}
	denied := deny.New(extraDeny)

	walkErr := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		slash := filepath.ToSlash(rel)

		if d.IsDir() {
			return classifyDir(d.Name(), slash, denied, &res)
		}
		classifyFile(slash, ign, denied, &res)
		return nil
	})
	if walkErr != nil {
		return res, walkErr
	}

	sort.Strings(res.Candidates)
	sort.Strings(res.Skipped)
	return res, nil
}

func classifyDir(name, slash string, denied *deny.Matcher, res *Result) error {
	// dew's own internals and VCS metadata are never scanned.
	if name == ".git" || name == ".dew" {
		return filepath.SkipDir
	}
	if denied.Match(slash, true) {
		res.Skipped = append(res.Skipped, slash+"/")
		return filepath.SkipDir
	}
	return nil
}

func classifyFile(slash string, ign *gitignore.GitIgnore, denied *deny.Matcher, res *Result) {
	// .gitignore itself is tracked by Git; never suggest it.
	if filepath.Base(slash) == ".gitignore" {
		return
	}
	if denied.Match(slash, false) {
		res.Skipped = append(res.Skipped, slash)
		return
	}
	if ign != nil && ign.MatchesPath(slash) {
		res.Candidates = append(res.Candidates, slash)
	}
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
