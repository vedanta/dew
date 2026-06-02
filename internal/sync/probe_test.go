package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestProbeLocalWritableDir(t *testing.T) {
	dir := t.TempDir()
	res, err := Probe(dir)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if res.Remote {
		t.Error("local path classified as remote")
	}
	if !res.OK() {
		t.Errorf("expected usable, got %+v", res.Checks)
	}
}

func TestProbeLocalMissingButCreatable(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-yet")
	res, err := Probe(dir)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !res.OK() {
		t.Errorf("a missing dir under a writable parent should be creatable: %+v", res.Checks)
	}
	if res.Checks[0].Name != "creatable" {
		t.Errorf("expected a 'creatable' check, got %+v", res.Checks)
	}

	// Multi-level: sync's MkdirAll creates the whole path, so a deep path under
	// a writable ancestor is still creatable.
	deep := filepath.Join(t.TempDir(), "a", "b", "c")
	res, err = Probe(deep)
	if err != nil {
		t.Fatalf("Probe deep: %v", err)
	}
	if !res.OK() {
		t.Errorf("deep path under a writable ancestor should be creatable: %+v", res.Checks)
	}
}

func TestProbeLocalNotADirectory(t *testing.T) {
	f := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(f, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	res, err := Probe(f)
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if res.OK() {
		t.Error("a regular file should not be a usable destination")
	}
}

func TestClassifyRemoteProbe(t *testing.T) {
	tests := []struct {
		name        string
		code        int
		out         string
		wantOK      bool
		wantFailing string // name of the first failing check (empty if all OK)
	}{
		{"success", 0, "", true, ""},
		{"untrusted host key", 255, "Host key verification failed.", false, "trusted"},
		{"auth denied", 255, "user@host: Permission denied (publickey).", false, "authenticated"},
		{"unreachable", 255, "ssh: connect to host nas port 22: Connection refused", false, "reachable"},
		{"path missing", 1, "", false, "path writable"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			checks := classifyRemoteProbe("nas", "/vol1/dew", tc.out, tc.code)
			res := ProbeResult{Checks: checks}
			if res.OK() != tc.wantOK {
				t.Fatalf("OK() = %v, want %v (%+v)", res.OK(), tc.wantOK, checks)
			}
			if tc.wantFailing == "" {
				return
			}
			var firstFail string
			for _, c := range checks {
				if !c.OK {
					firstFail = c.Name
					break
				}
			}
			if firstFail != tc.wantFailing {
				t.Errorf("first failing check = %q, want %q (%+v)", firstFail, tc.wantFailing, checks)
			}
		})
	}
}

func TestProbeRemoteUsesSSHStub(t *testing.T) {
	// Skip if ssh isn't on PATH (e.g. a minimal Windows runner) — depcheck runs
	// before the stub and would fail for reasons unrelated to this test.
	if _, err := exec.LookPath("ssh"); err != nil {
		t.Skip("ssh not available; classification is covered by TestClassifyRemoteProbe")
	}
	orig := runSSH
	t.Cleanup(func() { runSSH = orig })
	runSSH = func(_ ...string) (string, int, error) { return "", 0, nil }

	res, err := Probe("nas:/vol1/dew")
	if err != nil {
		t.Fatalf("Probe: %v", err)
	}
	if !res.Remote || !res.OK() {
		t.Errorf("expected remote + usable, got %+v", res)
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/vol1/dew"); got != "'/vol1/dew'" {
		t.Errorf("shellQuote = %q", got)
	}
	if got := shellQuote("a'b"); got != `'a'\''b'` {
		t.Errorf("shellQuote with quote = %q", got)
	}
}
