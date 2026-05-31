package compress

import (
	"bytes"
	"strings"
	"testing"
)

func TestCompressDecompressRoundTrip(t *testing.T) {
	original := []byte("the local half of your repo, restored after every clone")

	var compressed bytes.Buffer
	if err := Compress(&compressed, bytes.NewReader(original)); err != nil {
		t.Fatalf("Compress: %v", err)
	}

	var out bytes.Buffer
	if err := Decompress(&out, &compressed); err != nil {
		t.Fatalf("Decompress: %v", err)
	}
	if !bytes.Equal(out.Bytes(), original) {
		t.Errorf("round-trip = %q, want %q", out.Bytes(), original)
	}
}

func TestCompressShrinksCompressibleData(t *testing.T) {
	original := []byte(strings.Repeat("dew", 10000))

	var compressed bytes.Buffer
	if err := Compress(&compressed, bytes.NewReader(original)); err != nil {
		t.Fatalf("Compress: %v", err)
	}
	if compressed.Len() >= len(original) {
		t.Errorf("compressed size %d, want smaller than %d", compressed.Len(), len(original))
	}
}

func TestStreamingWriterReaderRoundTrip(t *testing.T) {
	original := []byte("streamed payload")

	var compressed bytes.Buffer
	w, err := NewWriter(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(original); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	r, err := NewReader(&compressed)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(out.Bytes(), original) {
		t.Errorf("round-trip = %q, want %q", out.Bytes(), original)
	}
}
