package cmd

import (
	"bytes"
	"strings"
	"testing"
)

// newPackProgress must stay silent unless there is something to show on a real
// terminal. In tests os.Stderr is not a terminal, so it always returns nil.
func TestNewPackProgressDisabledOffTerminal(t *testing.T) {
	if bar := newPackProgress(1024); bar != nil {
		t.Fatalf("expected nil bar off a terminal, got %#v", bar)
	}
	if bar := newPackProgress(0); bar != nil {
		t.Fatalf("expected nil bar for zero total, got %#v", bar)
	}
}

// A nil bar must wrap to the underlying writer unchanged, so callers can pass it
// straight into archive.Build without a nil check.
func TestNilBarWrapPassthrough(t *testing.T) {
	var dst bytes.Buffer
	var bar *progressBar
	w := bar.wrap(&dst)
	if _, err := w.Write([]byte("hello")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if dst.String() != "hello" {
		t.Fatalf("nil bar should pass through verbatim, got %q", dst.String())
	}
	bar.finish() // must be a no-op, not a panic
}

// An active bar must forward every byte to the real writer and account for them,
// while drawing the bar to its sink.
func TestActiveBarCountsAndPassesThrough(t *testing.T) {
	var dst, sink bytes.Buffer
	bar := &progressBar{sink: &sink, label: "Packing", total: 10}
	w := bar.wrap(&dst) // draws the initial frame

	payload := []byte("0123456789")
	if _, err := w.Write(payload); err != nil {
		t.Fatalf("write: %v", err)
	}
	if dst.String() != string(payload) {
		t.Fatalf("payload not forwarded: got %q", dst.String())
	}
	if bar.written != int64(len(payload)) {
		t.Fatalf("written = %d, want %d", bar.written, len(payload))
	}
	bar.finish()
	if !strings.Contains(sink.String(), "Packing") {
		t.Fatalf("expected the bar to be drawn to its sink, got %q", sink.String())
	}
}

func TestIsTerminalWriterFalseForBuffer(t *testing.T) {
	if isTerminalWriter(&bytes.Buffer{}) {
		t.Fatal("a bytes.Buffer is not a terminal")
	}
}
