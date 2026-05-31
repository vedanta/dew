package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsRemote(t *testing.T) {
	cases := map[string]bool{
		"nas:/volume1/dew": true,
		"user@host:/p/dew": true,
		"host:relative":    true,
		"/Volumes/nas/dew": false,
		"./local/dir":      false,
		"plainname":        false,
		`C:\Users\me\dew`:  false,
		"":                 false,
	}
	for dest, want := range cases {
		if got := IsRemote(dest); got != want {
			t.Errorf("IsRemote(%q) = %v, want %v", dest, got, want)
		}
	}
}

func TestPushPullLocalRoundTrip(t *testing.T) {
	srcDir := t.TempDir()
	localImage := filepath.Join(srcDir, "myrepo.dew.age")
	if err := os.WriteFile(localImage, []byte("ENCRYPTED-IMAGE-BYTES"), 0o600); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "store") // not yet created

	if err := Push(localImage, dest); err != nil {
		t.Fatalf("Push: %v", err)
	}
	pushed := filepath.Join(dest, "myrepo.dew.age")
	if b, err := os.ReadFile(pushed); err != nil || string(b) != "ENCRYPTED-IMAGE-BYTES" { //nolint:gosec // test path
		t.Fatalf("pushed file = %q, err=%v", b, err)
	}

	// Pull into a fresh local path.
	pullTo := filepath.Join(t.TempDir(), "images", "myrepo.dew.age")
	if err := Pull(pullTo, dest); err != nil {
		t.Fatalf("Pull: %v", err)
	}
	if b, err := os.ReadFile(pullTo); err != nil || string(b) != "ENCRYPTED-IMAGE-BYTES" { //nolint:gosec // test path
		t.Fatalf("pulled file = %q, err=%v", b, err)
	}
}

func TestGuardRejectsKeyFiles(t *testing.T) {
	dest := t.TempDir()
	for _, p := range []string{"/home/u/.dew/identity.age.key", "/x/secret.key"} {
		if err := Push(p, dest); err == nil || !strings.Contains(err.Error(), "refusing") {
			t.Errorf("Push(%q) should be refused, got %v", p, err)
		}
		if err := Pull(p, dest); err == nil || !strings.Contains(err.Error(), "refusing") {
			t.Errorf("Pull(%q) should be refused, got %v", p, err)
		}
	}
}

func TestRemotePushBuildsScpArgs(t *testing.T) {
	if _, err := exec.LookPath("scp"); err != nil {
		t.Skip("scp not on PATH; skipping remote arg check")
	}

	var gotArgs []string
	orig := runScp
	runScp = func(args ...string) error { gotArgs = args; return nil }
	defer func() { runScp = orig }()

	local := filepath.Join(t.TempDir(), "myrepo.dew.age")
	if err := Push(local, "nas:/volume1/dew"); err != nil {
		t.Fatalf("Push: %v", err)
	}
	want := []string{local, "nas:/volume1/dew/myrepo.dew.age"}
	if len(gotArgs) != 2 || gotArgs[0] != want[0] || gotArgs[1] != want[1] {
		t.Errorf("scp args = %v, want %v", gotArgs, want)
	}
}
