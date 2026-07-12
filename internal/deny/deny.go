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
	"Pods", "DerivedData", ".gradle", ".cxx", ".expo",
}

var builtinDirs = func() map[string]bool {
	m := make(map[string]bool, len(builtinDirNames))
	for _, d := range builtinDirNames {
		m[d] = true
	}
	return m
}()

const builtinDSStore = ".DS_Store"

// builtinFileSuffixes are file-name suffixes always treated as noise.
var builtinFileSuffixes = []string{".log", ".tsbuildinfo"}

// Builtin returns the built-in deny patterns in display form (directories with
// a trailing slash). It is the single source of truth shared with Match.
func Builtin() []string {
	out := make([]string, 0, len(builtinDirNames)+len(builtinFileSuffixes)+1)
	for _, d := range builtinDirNames {
		out = append(out, d+"/")
	}
	out = append(out, builtinDSStore)
	for _, s := range builtinFileSuffixes {
		out = append(out, "*"+s)
	}
	return out
}

// rule is one extra deny line: a compiled gitignore pattern plus whether it
// was negated ("!pattern" — un-deny). Compiled per line so a lone negation can
// still override the built-ins; the library's own negation handling only works
// against earlier positives in the same list.
type rule struct {
	negate  bool
	matcher *gitignore.GitIgnore
}

// Matcher tests repo-relative slash paths against the built-in rules plus any
// extra patterns, in layer order.
type Matcher struct {
	rules []rule
}

// New builds a matcher with the built-in rules plus extra patterns in
// .gitignore syntax (e.g. "*.tmp", ".gradle/", "!keep.log"). Pass the layers
// in precedence order — global first, then per-manifest — the last matching
// rule wins (git semantics), and the built-ins are the base verdict, so a
// repo rule overrides a global rule overrides a built-in.
func New(extra []string) *Matcher {
	m := &Matcher{}
	for _, line := range extra {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		negate := strings.HasPrefix(line, "!")
		m.rules = append(m.rules, rule{
			negate:  negate,
			matcher: gitignore.CompileIgnoreLines(strings.TrimPrefix(line, "!")),
		})
	}
	return m
}

// Match reports whether the repo-relative slash path is denied. isDir indicates
// the path is a directory (so directory-only rules apply). Like git, a negation
// cannot re-include a file under a directory the walk has already pruned —
// "!Pods/" is the unit of rescue, not "!Pods/keep.txt".
func (m *Matcher) Match(slashPath string, isDir bool) bool {
	base := path.Base(slashPath)
	denied := (isDir && builtinDirs[base]) || (!isDir && deniedFile(base))
	for _, r := range m.rules {
		// Directories also match trailing-slash ("dir/") patterns.
		if r.matcher.MatchesPath(slashPath) || (isDir && r.matcher.MatchesPath(slashPath+"/")) {
			denied = !r.negate
		}
	}
	return denied
}

// deniedFile reports whether a file base name matches the built-in file rules.
func deniedFile(base string) bool {
	if base == builtinDSStore {
		return true
	}
	for _, s := range builtinFileSuffixes {
		if strings.HasSuffix(base, s) {
			return true
		}
	}
	return false
}
