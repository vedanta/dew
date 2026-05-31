package manifest

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// CurrentVersion is the manifest schema version this build understands.
	CurrentVersion = 1
	// Dir is the per-repo dew directory, relative to the repo root.
	Dir = ".dew"
	// File is the manifest filename inside Dir.
	File = "manifest.yaml"
	// maxProjectNameLen bounds the project name so the derived image filename
	// stays well within filesystem limits.
	maxProjectNameLen = 64
)

// projectNameRe restricts project names to a path-safe charset (no separators,
// spaces, or control characters), since the name becomes the image filename.
var projectNameRe = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

// ValidateProjectName reports whether name is usable as a project name. The
// name becomes the image filename (<name>.dew.age) under ~/.dew/images, so it
// must not contain path separators or other unsafe characters.
func ValidateProjectName(name string) error {
	switch {
	case name == "":
		return errors.New("project name is empty")
	case len(name) > maxProjectNameLen:
		return fmt.Errorf("project name %q is too long (max %d characters)", name, maxProjectNameLen)
	case !projectNameRe.MatchString(name):
		return fmt.Errorf("project name %q may contain only letters, digits, '.', '_', '-' (no path separators or spaces)", name)
	case strings.Trim(name, ".") == "":
		return fmt.Errorf("project name %q must contain a non-dot character", name)
	}
	return nil
}

// Manifest is the repo-level contract committed to Git. It declares which local
// files dew manages; it never holds secrets, file contents, or keys.
type Manifest struct {
	Version int      `yaml:"version"`
	Project string   `yaml:"project"`
	Image   string   `yaml:"image"`
	Allow   []string `yaml:"allow"`
	Deny    []string `yaml:"deny,omitempty"`
}

// Path returns the manifest path for a repo rooted at repoRoot.
func Path(repoRoot string) string {
	return filepath.Join(repoRoot, Dir, File)
}

// New returns a manifest for project with sensible defaults: the current schema
// version, an empty allow-list, and the conventional image name.
func New(project string) *Manifest {
	return &Manifest{
		Version: CurrentVersion,
		Project: project,
		Image:   project + ".dew.age",
		Allow:   []string{},
	}
}

// Validate reports whether the manifest is well-formed and supported.
func (m *Manifest) Validate() error {
	switch {
	case m.Version == 0:
		return errors.New("manifest: missing version")
	case m.Version != CurrentVersion:
		return fmt.Errorf("manifest: unsupported version %d (this build supports %d)", m.Version, CurrentVersion)
	case m.Project == "":
		return errors.New("manifest: missing project")
	case m.Image == "":
		return errors.New("manifest: missing image")
	}
	return nil
}

// AddAllow adds p to the allow-list if not already present, returning true if
// the manifest changed.
func (m *Manifest) AddAllow(p string) bool {
	for _, e := range m.Allow {
		if e == p {
			return false
		}
	}
	m.Allow = append(m.Allow, p)
	return true
}

// RemoveAllow removes p from the allow-list if present, returning true if the
// manifest changed.
func (m *Manifest) RemoveAllow(p string) bool {
	for i, e := range m.Allow {
		if e == p {
			m.Allow = append(m.Allow[:i], m.Allow[i+1:]...)
			return true
		}
	}
	return false
}

// Load reads and validates the manifest at path.
func Load(path string) (*Manifest, error) {
	// path is the repo's own .dew/manifest.yaml, not attacker-controlled input.
	data, err := os.ReadFile(path) //nolint:gosec // G304: manifest path is repo-local
	if err != nil {
		return nil, fmt.Errorf("manifest: read %s: %w", path, err)
	}
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("manifest: parse %s: %w", path, err)
	}
	if err := m.Validate(); err != nil {
		return nil, err
	}
	return &m, nil
}

// Save validates the manifest and writes it to path, creating the .dew
// directory if needed. The manifest is non-secret and meant to be committed to
// Git, so it is written world-readable.
func Save(path string, m *Manifest) error {
	if err := m.Validate(); err != nil {
		return err
	}
	data, err := yaml.Marshal(m)
	if err != nil {
		return fmt.Errorf("manifest: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil { //nolint:gosec // G301: .dew is a non-secret repo dir
		return fmt.Errorf("manifest: create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil { //nolint:gosec // G306: manifest is non-secret, committed to Git
		return fmt.Errorf("manifest: write %s: %w", path, err)
	}
	return nil
}
