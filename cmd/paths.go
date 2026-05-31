package cmd

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// repoRelPath resolves arg (taken relative to the repo root) to a clean,
// slash-separated repo-relative path. It rejects anything outside the repo and
// the repo root itself, so a manifest can never reference paths it must not.
func repoRelPath(root, arg string) (string, error) {
	abs := arg
	if !filepath.IsAbs(abs) {
		abs = filepath.Join(root, arg)
	}
	rel, err := filepath.Rel(root, filepath.Clean(abs))
	if err != nil {
		return "", fmt.Errorf("resolve %q: %w", arg, err)
	}
	rel = filepath.ToSlash(rel)
	switch {
	case rel == ".." || strings.HasPrefix(rel, "../"):
		return "", fmt.Errorf("path %q is outside the repository", arg)
	case rel == ".":
		return "", errors.New("the repository root is not a valid path; specify explicit paths ('dew add .' for discovered candidates arrives in Phase 4)")
	}
	return rel, nil
}
