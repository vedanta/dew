package devices

import (
	"path/filepath"
	"testing"
)

func TestRecordAppendsAndReplaces(t *testing.T) {
	l := &Log{Version: CurrentVersion}
	l.Record(Entry{Peer: "u@nvk2", Direction: SentTo, Fingerprint: "age1aaa", At: "t1"})
	l.Record(Entry{Peer: "u@laptop", Direction: ReceivedFrom, Fingerprint: "age1aaa", At: "t1"})
	if len(l.Devices) != 2 {
		t.Fatalf("got %d entries, want 2", len(l.Devices))
	}

	// Same (peer, direction) → replace, not append.
	l.Record(Entry{Peer: "u@nvk2", Direction: SentTo, Fingerprint: "age1aaa", At: "t2", Label: "nvk2"})
	if len(l.Devices) != 2 {
		t.Fatalf("re-record should not add an entry, got %d", len(l.Devices))
	}
	for _, e := range l.Devices {
		if e.Peer == "u@nvk2" && e.Direction == SentTo && (e.At != "t2" || e.Label != "nvk2") {
			t.Errorf("entry not refreshed: %+v", e)
		}
	}

	// Same peer, different direction → distinct entry.
	l.Record(Entry{Peer: "u@nvk2", Direction: ReceivedFrom, Fingerprint: "age1aaa", At: "t3"})
	if len(l.Devices) != 3 {
		t.Errorf("different direction should be a new entry, got %d", len(l.Devices))
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	path := Path(filepath.Join(t.TempDir(), ".dew"))
	want := &Log{Version: CurrentVersion}
	want.Record(Entry{Peer: "u@nvk2", Direction: SentTo, Fingerprint: "age1aaa", At: "t1", Label: "nvk2"})
	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Devices) != 1 || got.Devices[0].Peer != "u@nvk2" || got.Devices[0].Label != "nvk2" {
		t.Errorf("round-trip mismatch: %+v", got.Devices)
	}
}

func TestLoadMissingIsEmpty(t *testing.T) {
	got, err := Load(Path(filepath.Join(t.TempDir(), ".dew")))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Devices) != 0 {
		t.Errorf("missing inventory should be empty, got %+v", got.Devices)
	}
}

func TestParseEmpty(t *testing.T) {
	got, err := Parse([]byte("  \n"))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if got.Version != CurrentVersion || len(got.Devices) != 0 {
		t.Errorf("empty parse = %+v", got)
	}
}
