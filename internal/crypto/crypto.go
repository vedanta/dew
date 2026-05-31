package crypto

import (
	"fmt"
	"io"
	"os"

	"filippo.io/age"
)

// EncryptWriter returns a WriteCloser that encrypts everything written to it for
// recipient (an age1... X25519 public key) and writes the ciphertext to dst.
// Close finalizes the age stream and must be called before dst is used. It uses
// the native age library — no external tooling.
func EncryptWriter(dst io.Writer, recipient string) (io.WriteCloser, error) {
	recip, err := age.ParseX25519Recipient(recipient)
	if err != nil {
		return nil, fmt.Errorf("crypto: parse recipient: %w", err)
	}
	w, err := age.Encrypt(dst, recip)
	if err != nil {
		return nil, fmt.Errorf("crypto: init encryption: %w", err)
	}
	return w, nil
}

// Encrypt encrypts everything read from src to dst for the given recipient.
func Encrypt(dst io.Writer, src io.Reader, recipient string) error {
	w, err := EncryptWriter(dst, recipient)
	if err != nil {
		return err
	}
	if _, err := io.Copy(w, src); err != nil {
		return fmt.Errorf("crypto: encrypt: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("crypto: finalize encryption: %w", err)
	}
	return nil
}

// DecryptReader returns a reader that decrypts src using the age identity in
// keyFile. The key file is read and closed up front; the returned reader draws
// from src lazily. It errors if no identity in the file matches.
func DecryptReader(src io.Reader, keyFile string) (io.Reader, error) {
	f, err := os.Open(keyFile) //nolint:gosec // G304: identity file is dew-home-local
	if err != nil {
		return nil, fmt.Errorf("crypto: open identity %s: %w", keyFile, err)
	}
	defer func() { _ = f.Close() }()

	ids, err := age.ParseIdentities(f)
	if err != nil {
		return nil, fmt.Errorf("crypto: parse identity %s: %w", keyFile, err)
	}
	r, err := age.Decrypt(src, ids...)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt (wrong key or corrupt image?): %w", err)
	}
	return r, nil
}

// Decrypt decrypts everything read from src to dst using the age identity in
// keyFile.
func Decrypt(dst io.Writer, src io.Reader, keyFile string) error {
	r, err := DecryptReader(src, keyFile)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, r); err != nil { //nolint:gosec // G110: image size is bounded by the local archive
		return fmt.Errorf("crypto: read decrypted stream: %w", err)
	}
	return nil
}
