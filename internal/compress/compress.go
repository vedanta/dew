// Package compress wraps zstd compression for the pack/restore pipeline using
// the pure-Go klauspost/compress library — no external zstd binary.
package compress

import (
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

// NewWriter returns a zstd encoder that compresses to w. The caller must Close
// it to flush the final frame.
func NewWriter(w io.Writer) (*zstd.Encoder, error) {
	enc, err := zstd.NewWriter(w)
	if err != nil {
		return nil, fmt.Errorf("compress: init encoder: %w", err)
	}
	return enc, nil
}

// NewReader returns a zstd decoder reading from r. The caller must Close it.
func NewReader(r io.Reader) (*zstd.Decoder, error) {
	dec, err := zstd.NewReader(r)
	if err != nil {
		return nil, fmt.Errorf("compress: init decoder: %w", err)
	}
	return dec, nil
}

// Compress reads all of src and writes a zstd stream to dst.
func Compress(dst io.Writer, src io.Reader) error {
	enc, err := NewWriter(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(enc, src); err != nil {
		_ = enc.Close()
		return fmt.Errorf("compress: %w", err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("compress: finalize: %w", err)
	}
	return nil
}

// Decompress reads a zstd stream from src and writes the original bytes to dst.
func Decompress(dst io.Writer, src io.Reader) error {
	dec, err := NewReader(src)
	if err != nil {
		return err
	}
	defer dec.Close()
	// The image is the user's own data, encrypted under their key before it is
	// ever decompressed, so it is not untrusted input (no zstd-bomb vector).
	if _, err := io.Copy(dst, dec); err != nil { //nolint:gosec // G110: source is the user's own decrypted image
		return fmt.Errorf("compress: decompress: %w", err)
	}
	return nil
}
