package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingReturnsDefault(t *testing.T) {
	c, err := Load(Path(t.TempDir()))
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if c.Version != CurrentVersion {
		t.Errorf("version = %d, want %d", c.Version, CurrentVersion)
	}
	if c.Sync.Destination != "" {
		t.Errorf("destination = %q, want empty", c.Sync.Destination)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	home := filepath.Join(t.TempDir(), ".dew")
	path := Path(home)

	want := Default()
	want.Sync.Destination = "nas:/volume1/dew"
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Sync.Destination != "nas:/volume1/dew" {
		t.Errorf("destination = %q, want nas:/volume1/dew", got.Sync.Destination)
	}
}

func TestDenyRoundTrip(t *testing.T) {
	path := Path(filepath.Join(t.TempDir(), ".dew"))
	want := Default()
	want.Deny = []string{"*.swp", ".idea/"}
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Deny) != 2 || got.Deny[0] != "*.swp" || got.Deny[1] != ".idea/" {
		t.Errorf("deny = %v, want [*.swp .idea/]", got.Deny)
	}
}

func TestLoadMalformed(t *testing.T) {
	path := Path(t.TempDir())
	if err := os.WriteFile(path, []byte("sync: : not yaml ]["), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error on malformed config, got nil")
	}
}

func TestLoadVersionlessTreatedAsCurrent(t *testing.T) {
	path := Path(t.TempDir())
	if err := os.WriteFile(path, []byte("sync:\n  destination: /Volumes/nas/dew\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	c, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if c.Version != CurrentVersion {
		t.Errorf("version = %d, want defaulted to %d", c.Version, CurrentVersion)
	}
	if c.Sync.Destination != "/Volumes/nas/dew" {
		t.Errorf("destination = %q", c.Sync.Destination)
	}
}
