package crypto

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"filippo.io/age"
)

func TestEncryptDecryptRoundTrip(t *testing.T) {
	id := mustIdentity(t)
	keyFile := writeKey(t, id)
	plaintext := []byte("the local half of your repo")

	var ct bytes.Buffer
	if err := Encrypt(&ct, bytes.NewReader(plaintext), id.Recipient().String()); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(ct.Bytes(), plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}

	var pt bytes.Buffer
	if err := Decrypt(&pt, &ct, keyFile); err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(pt.Bytes(), plaintext) {
		t.Errorf("round-trip = %q, want %q", pt.Bytes(), plaintext)
	}
}

func TestDecryptWrongKeyFails(t *testing.T) {
	encTo := mustIdentity(t) // image encrypted for this recipient
	wrong := mustIdentity(t) // a different identity tries to read it
	wrongKey := writeKey(t, wrong)

	var ct bytes.Buffer
	if err := Encrypt(&ct, bytes.NewReader([]byte("secret")), encTo.Recipient().String()); err != nil {
		t.Fatalf("Encrypt: %v", err)
	}

	var pt bytes.Buffer
	if err := Decrypt(&pt, &ct, wrongKey); err == nil {
		t.Fatal("expected decryption with the wrong key to fail, got nil")
	}
}

func TestEncryptRejectsBadRecipient(t *testing.T) {
	var ct bytes.Buffer
	if err := Encrypt(&ct, bytes.NewReader([]byte("x")), "not-a-recipient"); err == nil {
		t.Fatal("expected error for malformed recipient, got nil")
	}
}

func mustIdentity(t *testing.T) *age.X25519Identity {
	t.Helper()
	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func writeKey(t *testing.T, id *age.X25519Identity) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "identity.age.key")
	if err := os.WriteFile(path, []byte(id.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
