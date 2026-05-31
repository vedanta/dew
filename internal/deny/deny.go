// Package deny decides which repo-relative paths are noise that dew must never
// pack or suggest. It combines a built-in pattern set with the per-manifest
// deny: rules, so the allow-list says what to include and the deny-list
// guarantees noise stays out (design §10).
package deny

import (
	"path"
	"strings"

	gitignore "github.com/sabhiram/go-gitignore"
)

// builtinDirs are directory names always treated as noise, even when an
// allow-listed directory would otherwise sweep them in.
var builtinDirs = map[string]bool{
	"node_modules": true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".venv":        true,
	"__pycache__":  true,
}

// Matcher tests repo-relative slash paths against the built-in rules plus any
// extra per-manifest patterns.
type Matcher struct {
	extra *gitignore.GitIgnore // nil when there are no extra patterns
}

// New builds a matcher with the built-in rules plus extra patterns in
// .gitignore syntax (e.g. "*.tmp", ".next/").
func New(extra []string) *Matcher {
	m := &Matcher{}
	if len(extra) > 0 {
		m.extra = gitignore.CompileIgnoreLines(extra...)
	}
	return m
}

// Match reports whether the repo-relative slash path is denied. isDir indicates
// the path is a directory (so directory-only rules apply).
func (m *Matcher) Match(slashPath string, isDir bool) bool {
	base := path.Base(slashPath)
	switch {
	case isDir && builtinDirs[base]:
		return true
	case !isDir && (base == ".DS_Store" || strings.HasSuffix(base, ".log")):
		return true
	case m.extra != nil && m.extra.MatchesPath(slashPath):
		return true
	}
	return false
}
