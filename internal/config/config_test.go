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

func TestSetAndClearDestinationPreserveOtherFields(t *testing.T) {
	path := Path(filepath.Join(t.TempDir(), ".dew"))

	// Seed a config with a deny-list so we can confirm it survives.
	seed := Default()
	seed.Deny = []string{"*.swp"}
	if err := Save(path, seed); err != nil {
		t.Fatalf("Save seed: %v", err)
	}

	if err := SetDestination(path, "nas:/vol1/dew"); err != nil {
		t.Fatalf("SetDestination: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Sync.Destination != "nas:/vol1/dew" {
		t.Errorf("destination = %q, want nas:/vol1/dew", got.Sync.Destination)
	}
	if len(got.Deny) != 1 || got.Deny[0] != "*.swp" {
		t.Errorf("deny not preserved: %v", got.Deny)
	}

	if err := ClearDestination(path); err != nil {
		t.Fatalf("ClearDestination: %v", err)
	}
	got, err = Load(path)
	if err != nil {
		t.Fatalf("Load after clear: %v", err)
	}
	if got.Sync.Destination != "" {
		t.Errorf("destination = %q, want empty after clear", got.Sync.Destination)
	}
	if len(got.Deny) != 1 || got.Deny[0] != "*.swp" {
		t.Errorf("deny not preserved after clear: %v", got.Deny)
	}
}

func TestSetDestinationCreatesMissingConfig(t *testing.T) {
	path := Path(filepath.Join(t.TempDir(), ".dew"))
	if err := SetDestination(path, "/Volumes/nas/dew"); err != nil {
		t.Fatalf("SetDestination: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.Sync.Destination != "/Volumes/nas/dew" {
		t.Errorf("destination = %q", got.Sync.Destination)
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
