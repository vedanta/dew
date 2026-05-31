package crypto

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

// Encrypt encrypts everything read from src to dst for the given recipient (an
// age1... X25519 public key). It uses the native age library — no external
// tooling.
func Encrypt(dst io.Writer, src io.Reader, recipient string) error {
	recip, err := age.ParseX25519Recipient(recipient)
	if err != nil {
		return fmt.Errorf("crypto: parse recipient: %w", err)
	}
	w, err := age.Encrypt(dst, recip)
	if err != nil {
		return fmt.Errorf("crypto: init encryption: %w", err)
	}
	if _, err := io.Copy(w, src); err != nil {
		return fmt.Errorf("crypto: encrypt: %w", err)
	}
	// Close finalizes the age stream; it must run before dst is used.
	if err := w.Close(); err != nil {
		return fmt.Errorf("crypto: finalize encryption: %w", err)
	}
	return nil
}

// Decrypt decrypts everything read from src to dst using the age identity in
// keyFile. It returns an error if no identity in the file matches.
func Decrypt(dst io.Writer, src io.Reader, keyFile string) error {
	f, err := os.Open(keyFile) //nolint:gosec // G304: identity file is dew-home-local
	if err != nil {
		return fmt.Errorf("crypto: open identity %s: %w", keyFile, err)
	}
	defer func() { _ = f.Close() }()

	ids, err := age.ParseIdentities(f)
	if err != nil {
		return fmt.Errorf("crypto: parse identity %s: %w", keyFile, err)
	}
	r, err := age.Decrypt(src, ids...)
	if err != nil {
		return fmt.Errorf("crypto: decrypt (wrong key or corrupt image?): %w", err)
	}
	if _, err := io.Copy(dst, r); err != nil { //nolint:gosec // G110: image size is bounded by the local archive
		return fmt.Errorf("crypto: read decrypted stream: %w", err)
	}
	return nil
}
