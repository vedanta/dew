package cmd

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// progressBar renders a byte-based progress bar as data streams through it.
//
// It wraps the writer at the head of the pack pipeline (tar -> zstd -> age): the
// bytes flowing in are the uncompressed tar stream, so total — the sum of the
// tracked files' sizes — is an accurate denominator (tar framing adds a little
// overhead, so the fraction is capped at 100%).
//
// Rendering only happens when sink is a real terminal; for piped output, CI, and
// tests the bar is a transparent pass-through, so the machine-readable line dew
// prints to stdout is never polluted with carriage returns.
type progressBar struct {
	dst      io.Writer // the real pipeline writer (zstd encoder)
	sink     io.Writer // where the bar is drawn (os.Stderr), or nil to disable
	label    string
	total    int64
	written  int64
	lastDraw time.Time
}

// newPackProgress returns a progress bar for packing total bytes, drawing to
// stderr only when stderr is an interactive terminal and there is something to
// show. Otherwise it returns nil — callers must treat a nil bar as "no progress".
func newPackProgress(total int64) *progressBar {
	if total <= 0 || !isTerminalWriter(os.Stderr) {
		return nil
	}
	return &progressBar{sink: os.Stderr, label: "Packing", total: total}
}

// wrap attaches the real pipeline writer and draws the initial (0%) bar. It
// returns the io.Writer that archive.Build should write to: the bar itself when
// active, or dst unchanged when b is nil.
func (b *progressBar) wrap(dst io.Writer) io.Writer {
	if b == nil {
		return dst
	}
	b.dst = dst
	b.render(true)
	return b
}

func (b *progressBar) Write(p []byte) (int, error) {
	n, err := b.dst.Write(p)
	b.written += int64(n)
	b.render(false)
	return n, err
}

// render draws the bar, throttled to ~10 fps unless force is set.
func (b *progressBar) render(force bool) {
	if b == nil || b.sink == nil {
		return
	}
	now := time.Now()
	if !force && now.Sub(b.lastDraw) < 100*time.Millisecond {
		return
	}
	b.lastDraw = now

	const width = 24
	frac := float64(b.written) / float64(b.total)
	if frac > 1 {
		frac = 1
	}
	filled := int(frac * width)

	var bar strings.Builder
	for i := range width {
		switch {
		case i < filled:
			bar.WriteByte('=')
		case i == filled && frac < 1:
			bar.WriteByte('>')
		default:
			bar.WriteByte(' ')
		}
	}
	// \r returns to column 0; \x1b[K clears to end of line so a shorter line
	// never leaves stale characters behind.
	// Best-effort cosmetic output; a failed terminal write is not worth surfacing.
	_, _ = fmt.Fprintf(b.sink, "\r\x1b[K%s [%s] %3.0f%%  %s / %s",
		b.label, bar.String(), frac*100, humanBytes(b.written), humanBytes(b.total))
}

// finish clears the bar line so the command's normal stdout output lands clean.
func (b *progressBar) finish() {
	if b == nil || b.sink == nil {
		return
	}
	_, _ = fmt.Fprint(b.sink, "\r\x1b[K")
}

// isTerminalWriter reports whether w is an interactive terminal. It uses the
// character-device bit, which holds for consoles on both Unix and Windows and
// needs no extra dependency — enough to decide whether drawing a live bar is
// appropriate.
func isTerminalWriter(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := f.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}
