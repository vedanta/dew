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

// builtinDirNames are directory names always treated as noise, even when an
// allow-listed directory would otherwise sweep them in.
var builtinDirNames = []string{
	"node_modules", "dist", "build", "target", ".venv", "__pycache__",
	".next", ".nuxt", "coverage", ".cache", ".turbo", ".parcel-cache",
}

var builtinDirs = func() map[string]bool {
	m := make(map[string]bool, len(builtinDirNames))
	for _, d := range builtinDirNames {
		m[d] = true
	}
	return m
}()

const (
	builtinDSStore = ".DS_Store"
	builtinLogGlob = "*.log"
)

// Builtin returns the built-in deny patterns in display form (directories with
// a trailing slash). It is the single source of truth shared with Match.
func Builtin() []string {
	out := make([]string, 0, len(builtinDirNames)+2)
	for _, d := range builtinDirNames {
		out = append(out, d+"/")
	}
	return append(out, builtinDSStore, builtinLogGlob)
}

// Matcher tests repo-relative slash paths against the built-in rules plus any
// extra per-manifest patterns.
type Matcher struct {
	extra *gitignore.GitIgnore // nil when there are no extra patterns
}

// New builds a matcher with the built-in rules plus extra patterns in
// .gitignore syntax (e.g. "*.tmp", ".gradle/").
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
	case !isDir && (base == builtinDSStore || strings.HasSuffix(base, ".log")):
		return true
	case m.extra != nil && m.extra.MatchesPath(slashPath):
		return true
	}
	return false
}
