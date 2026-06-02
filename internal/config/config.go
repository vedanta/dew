package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	// CurrentVersion is the config schema version this build understands.
	CurrentVersion = 1
	// FileName is the config filename inside the dew home directory.
	FileName = "config.yaml"
)

// Config is the global dew configuration stored at ~/.dew/config.yaml. It holds
// no secrets — only preferences such as the sync destination and a user-level
// deny-list applied across all repos.
type Config struct {
	Version int      `yaml:"version"`
	Sync    Sync     `yaml:"sync"`
	Deny    []string `yaml:"deny,omitempty"`
}

// Sync holds sync preferences.
type Sync struct {
	// Destination is where images are pushed/pulled. Either a local/mounted
	// path (e.g. /Volumes/nas/dew) or an scp-style remote (e.g. nas:/vol/dew).
	Destination string `yaml:"destination,omitempty"`
}

// Path returns the config path for a dew home directory.
func Path(home string) string {
	return filepath.Join(home, FileName)
}

// Default returns an empty config at the current version.
func Default() *Config {
	return &Config{Version: CurrentVersion}
}

// Load reads the config at path. A missing file is not an error — it returns
// the default config, since configuration is optional.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: config path is dew-home-local
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}
	// A hand-written config may omit version; treat it as current.
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	if c.Version > CurrentVersion {
		return nil, fmt.Errorf("config: unsupported version %d (this build supports %d)", c.Version, CurrentVersion)
	}
	return &c, nil
}

// SetDestination updates the sync destination and persists the config at path,
// preserving every other field. A missing config is created.
func SetDestination(path, dest string) error {
	c, err := Load(path)
	if err != nil {
		return err
	}
	c.Sync.Destination = dest
	return Save(path, c)
}

// ClearDestination removes the sync destination and persists the config,
// preserving every other field.
func ClearDestination(path string) error {
	c, err := Load(path)
	if err != nil {
		return err
	}
	c.Sync.Destination = ""
	return Save(path, c)
}

// Save writes the config to path, creating the dew home directory if needed.
func Save(path string, c *Config) error {
	if c.Version == 0 {
		c.Version = CurrentVersion
	}
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("config: create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}
