// Package gittrack asks Git which paths it already carries for a repository.
// dew's design center is the local half of a repo — the files Git does NOT
// carry — and 'pack --all' needs the authoritative tracked set to know where
// that boundary is. This is dew's only git invocation, needed only by --all
// (the same optional-tool pattern as scp for remote sync).
package gittrack

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/vedanta/dew/internal/depcheck"
)

// ErrNotARepo reports that root is not inside a Git repository.
var ErrNotARepo = errors.New("not a git repository")

// Tracked returns the set of repo-relative slash paths Git carries for the
// repository at root (the index: committed or staged). Untracked and ignored
// files — dew's domain — are absent from the set.
func Tracked(root string) (map[string]bool, error) {
	if err := depcheck.RequireTool("git", "install Git to use 'dew pack --all' (it asks Git what it already carries)"); err != nil {
		return nil, err
	}
	cmd := exec.Command("git", "-C", root, "ls-files", "-z") //nolint:gosec // G204: fixed argv; root is the repo the user ran dew in
	var out, stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if strings.Contains(stderr.String(), "not a git repository") {
			return nil, ErrNotARepo
		}
		return nil, fmt.Errorf("gittrack: git ls-files: %w (%s)", err, strings.TrimSpace(stderr.String()))
	}

	tracked := make(map[string]bool)
	for _, p := range strings.Split(out.String(), "\x00") {
		if p != "" {
			tracked[p] = true // git emits slash-separated paths on every OS
		}
	}
	return tracked, nil
}
