package identity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"filippo.io/age"
)

const (
	// KeyFileName is the private identity file inside the dew home directory.
	KeyFileName = "identity.age.key"
	// PubFileName is the public key file inside the dew home directory.
	PubFileName = "identity.age.pub"
	// ImagesDirName holds the per-repo encrypted images.
	ImagesDirName = "images"
)

// Paths holds the on-disk locations for the global dew identity, rooted at a
// dew home directory (default ~/.dew).
type Paths struct {
	Home      string
	KeyFile   string
	PubFile   string
	ImagesDir string
}

// NewPaths derives the identity paths from a dew home directory.
func NewPaths(home string) Paths {
	return Paths{
		Home:      home,
		KeyFile:   filepath.Join(home, KeyFileName),
		PubFile:   filepath.Join(home, PubFileName),
		ImagesDir: filepath.Join(home, ImagesDirName),
	}
}

// DefaultHome returns the dew home directory: $DEW_HOME if set, otherwise
// ~/.dew.
func DefaultHome() (string, error) {
	if h := os.Getenv("DEW_HOME"); h != "" {
		return h, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("identity: locate home directory: %w", err)
	}
	return filepath.Join(home, ".dew"), nil
}

// Generate creates a new age X25519 identity at p, creating the home and images
// directories. It refuses to overwrite an existing identity. It returns the
// public key (an age1... recipient string).
func Generate(p Paths) (string, error) {
	if _, err := os.Stat(p.KeyFile); err == nil {
		return "", fmt.Errorf("identity: already exists at %s", p.KeyFile)
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("identity: %w", err)
	}

	if err := os.MkdirAll(p.ImagesDir, 0o700); err != nil {
		return "", fmt.Errorf("identity: create %s: %w", p.ImagesDir, err)
	}

	id, err := age.GenerateX25519Identity()
	if err != nil {
		return "", fmt.Errorf("identity: generate key: %w", err)
	}
	pub := id.Recipient().String()

	// Mirror age-keygen's file format so the key is usable with the age CLI too.
	keyContent := fmt.Sprintf("# created: %s\n# public key: %s\n%s\n",
		time.Now().UTC().Format(time.RFC3339), pub, id.String())
	if err := os.WriteFile(p.KeyFile, []byte(keyContent), 0o600); err != nil {
		return "", fmt.Errorf("identity: write %s: %w", p.KeyFile, err)
	}
	if err := os.WriteFile(p.PubFile, []byte(pub+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("identity: write %s: %w", p.PubFile, err)
	}
	return pub, nil
}

// Status describes the presence and public key of the global identity.
type Status struct {
	Present   bool
	KeyFile   string
	PubFile   string
	PublicKey string
}

// Inspect reports the identity status at p. A present identity whose public key
// file is missing has its public key derived from the private key.
func Inspect(p Paths) (Status, error) {
	s := Status{KeyFile: p.KeyFile, PubFile: p.PubFile}

	if _, err := os.Stat(p.KeyFile); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return s, nil
		}
		return s, fmt.Errorf("identity: %w", err)
	}
	s.Present = true

	pub, err := os.ReadFile(p.PubFile) //nolint:gosec // G304: pub file is dew-home-local
	if err == nil {
		s.PublicKey = strings.TrimSpace(string(pub))
		return s, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return s, fmt.Errorf("identity: read %s: %w", p.PubFile, err)
	}

	// Public key file missing: derive it from the private key.
	derived, derr := publicFromKeyFile(p.KeyFile)
	if derr != nil {
		return s, derr
	}
	s.PublicKey = derived
	return s, nil
}

// PublicKeyFromFile derives the age public key (recipient string) from a
// private key file — used to verify a key fetched from another machine.
func PublicKeyFromFile(keyFile string) (string, error) {
	return publicFromKeyFile(keyFile)
}

func publicFromKeyFile(keyFile string) (string, error) {
	f, err := os.Open(keyFile) //nolint:gosec // G304: key file is dew-home-local
	if err != nil {
		return "", fmt.Errorf("identity: open %s: %w", keyFile, err)
	}
	defer func() { _ = f.Close() }()

	ids, err := age.ParseIdentities(f)
	if err != nil {
		return "", fmt.Errorf("identity: parse %s: %w", keyFile, err)
	}
	for _, id := range ids {
		if x, ok := id.(*age.X25519Identity); ok {
			return x.Recipient().String(), nil
		}
	}
	return "", fmt.Errorf("identity: no X25519 key in %s", keyFile)
}
