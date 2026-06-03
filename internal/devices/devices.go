// Package devices maintains ~/.dew/devices.yaml — a best-effort, local audit
// log of where this machine's identity has been sent or received over
// 'dew key push'/'pull'.
//
// It is NOT a registry or a revocation tool: it records only dew-initiated
// transfers (manual copies bypass it), and removing an entry does not
// de-provision a machine (dew has no key rotation). Treat it as "where have I
// distributed this, and when", nothing more.
package devices

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// CurrentVersion is the schema version this build writes.
	CurrentVersion = 1
	// FileName is the inventory filename inside the dew home directory.
	FileName = "devices.yaml"
)

// Direction is the perspective of an entry, from the writer's point of view.
type Direction string

const (
	// SentTo means this machine sent its identity to the peer.
	SentTo Direction = "sent-to"
	// ReceivedFrom means this machine received its identity from the peer.
	ReceivedFrom Direction = "received-from"
)

// Entry is one recorded transfer.
type Entry struct {
	Peer        string    `yaml:"peer"`
	Direction   Direction `yaml:"direction"`
	Fingerprint string    `yaml:"fingerprint"`
	At          string    `yaml:"at"`
	Label       string    `yaml:"label,omitempty"`
}

// Log is the on-disk inventory.
type Log struct {
	Version int     `yaml:"version"`
	Devices []Entry `yaml:"devices"`
}

// Path returns the inventory path for a dew home directory.
func Path(home string) string { return filepath.Join(home, FileName) }

// Parse reads a Log from YAML bytes; empty input yields an empty Log.
func Parse(data []byte) (*Log, error) {
	if strings.TrimSpace(string(data)) == "" {
		return &Log{Version: CurrentVersion}, nil
	}
	var l Log
	if err := yaml.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("devices: parse: %w", err)
	}
	if l.Version == 0 {
		l.Version = CurrentVersion
	}
	return &l, nil
}

// Load reads the inventory at path. A missing file is not an error — it returns
// an empty Log.
func Load(path string) (*Log, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: devices path is dew-home-local
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &Log{Version: CurrentVersion}, nil
		}
		return nil, fmt.Errorf("devices: read %s: %w", path, err)
	}
	return Parse(data)
}

// Marshal serializes the Log to YAML.
func (l *Log) Marshal() ([]byte, error) {
	if l.Version == 0 {
		l.Version = CurrentVersion
	}
	data, err := yaml.Marshal(l)
	if err != nil {
		return nil, fmt.Errorf("devices: marshal: %w", err)
	}
	return data, nil
}

// Save writes the inventory to path, creating the dew home directory if needed.
func Save(path string, l *Log) error {
	data, err := l.Marshal()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("devices: create dir: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("devices: write %s: %w", path, err)
	}
	return nil
}

// Record adds e, or replaces an existing entry with the same (peer, direction)
// — so re-running a transfer refreshes the record rather than duplicating it.
func (l *Log) Record(e Entry) {
	for i := range l.Devices {
		if l.Devices[i].Peer == e.Peer && l.Devices[i].Direction == e.Direction {
			l.Devices[i] = e
			return
		}
	}
	l.Devices = append(l.Devices, e)
}
