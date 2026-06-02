package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListLocal(t *testing.T) {
	dir := t.TempDir()
	write := func(name, content string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("acme-api.dew.age", "1234")
	write("billing.dew.age", "12")
	write("notes.txt", "ignored")           // not an image
	write("acme-api.dew.age.id", "ignored") // ownership marker, not an image
	if err := os.Mkdir(filepath.Join(dir, "sub.dew.age"), 0o700); err != nil {
		t.Fatal(err) // a directory, even with the suffix, is skipped
	}

	imgs, err := List(dir)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(imgs) != 2 {
		t.Fatalf("got %d images, want 2: %+v", len(imgs), imgs)
	}
	// sorted by name
	if imgs[0].Name != "acme-api.dew.age" || imgs[1].Name != "billing.dew.age" {
		t.Errorf("unexpected names/order: %+v", imgs)
	}
	if imgs[0].Size != 4 {
		t.Errorf("size = %d, want 4", imgs[0].Size)
	}
	if imgs[0].Modified.IsZero() {
		t.Error("expected a local mtime")
	}
}

func TestListLocalMissingDirIsEmpty(t *testing.T) {
	imgs, err := List(filepath.Join(t.TempDir(), "nope"))
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(imgs) != 0 {
		t.Errorf("missing dir should be empty, got %+v", imgs)
	}
}

func TestParseLSList(t *testing.T) {
	out := `total 16
-rw-r--r-- 1 user group 2048 May 30 12:00 /vol1/dew/acme-api.dew.age
-rw-r--r-- 1 user group 64 May 31 09:30 /vol1/dew/billing.dew.age
-rw-r--r-- 1 user group 10 May 31 09:30 /vol1/dew/notes.txt
garbage line`
	imgs := parseLSList(out)
	if len(imgs) != 2 {
		t.Fatalf("got %d, want 2: %+v", len(imgs), imgs)
	}
	if imgs[0].Name != "acme-api.dew.age" || imgs[0].Size != 2048 {
		t.Errorf("first = %+v", imgs[0])
	}
	if imgs[1].Name != "billing.dew.age" || imgs[1].Size != 64 {
		t.Errorf("second = %+v", imgs[1])
	}
	for _, im := range imgs {
		if !im.Modified.IsZero() {
			t.Errorf("remote mtime should be zero (unknown): %+v", im)
		}
	}
}

func TestParseLSListEmpty(t *testing.T) {
	if imgs := parseLSList(""); len(imgs) != 0 {
		t.Errorf("empty output should yield no images, got %+v", imgs)
	}
}
