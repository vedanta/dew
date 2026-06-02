package sync

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vedanta/dew/internal/depcheck"
)

const sshHint = scpHint // ssh ships with the same OpenSSH package as scp

// Check is one verified property of a destination (e.g. reachable, writable).
type Check struct {
	Name string // short label
	OK   bool
	Note string // detail, set when OK is false (or an informational aside)
}

// ProbeResult is the outcome of testing a destination.
type ProbeResult struct {
	Destination string
	Remote      bool
	Checks      []Check
}

// OK reports whether every check passed.
func (r ProbeResult) OK() bool {
	for _, c := range r.Checks {
		if !c.OK {
			return false
		}
	}
	return true
}

// runSSHProbe runs ssh and returns combined output and the process exit code
// (0 on success). Overridable in tests. A code of -1 means ssh could not be
// started. Arguments are argv (never a shell line), so a destination can't
// inject a local command.
var runSSHProbe = func(args ...string) (combined string, code int, err error) {
	cmd := exec.Command("ssh", args...) //nolint:gosec // G204: args are dew-controlled (configured host + a fixed remote test command)
	out, runErr := cmd.CombinedOutput()
	combined = strings.TrimSpace(string(out))
	if runErr == nil {
		return combined, 0, nil
	}
	var ee *exec.ExitError
	if errors.As(runErr, &ee) {
		return combined, ee.ExitCode(), nil
	}
	return combined, -1, runErr
}

// Probe tests whether destination is usable as a sync target. Local/mounted
// paths are checked in-process; remote host:path destinations are checked over
// ssh (trust/auth stay OpenSSH's job — we only report its verdict).
func Probe(destination string) (ProbeResult, error) {
	res := ProbeResult{Destination: destination, Remote: IsRemote(destination)}
	if res.Remote {
		return probeRemote(destination, res)
	}
	return probeLocal(destination, res), nil
}

func probeLocal(destination string, res ProbeResult) ProbeResult {
	info, err := os.Stat(destination)
	switch {
	case err == nil && info.IsDir():
		res.Checks = append(res.Checks, Check{Name: "exists", OK: true})
		if note := writableNote(destination); note != "" {
			res.Checks = append(res.Checks, Check{Name: "writable", OK: false, Note: note})
		} else {
			res.Checks = append(res.Checks, Check{Name: "writable", OK: true})
		}
	case err == nil && !info.IsDir():
		res.Checks = append(res.Checks, Check{Name: "exists", OK: false, Note: "exists but is not a directory"})
	default:
		// Doesn't exist. dew sync would create it (copyFile does MkdirAll on the
		// whole path), so report whether the nearest existing ancestor is
		// writable rather than failing outright.
		if anc := nearestExistingDir(destination); anc != "" && writableNote(anc) == "" {
			res.Checks = append(res.Checks, Check{Name: "creatable", OK: true,
				Note: "does not exist yet; 'dew sync' will create it"})
		} else {
			res.Checks = append(res.Checks, Check{Name: "exists", OK: false,
				Note: "not found, and no writable parent exists (is the volume mounted?)"})
		}
	}
	return res
}

// nearestExistingDir walks up from p's parent to the first existing directory,
// or "" if none exists up to the filesystem root.
func nearestExistingDir(p string) string {
	dir := filepath.Dir(p)
	for {
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// writableNote returns "" if dir is writable, or a short reason otherwise.
func writableNote(dir string) string {
	f, err := os.CreateTemp(dir, ".dew-probe-*")
	if err != nil {
		return "not writable"
	}
	name := f.Name()
	_ = f.Close()
	_ = os.Remove(name)
	return ""
}

func probeRemote(destination string, res ProbeResult) (ProbeResult, error) {
	if err := depcheck.RequireTool("ssh", sshHint); err != nil {
		return res, err
	}
	host, path, ok := splitRemote(destination)
	if !ok {
		return res, fmt.Errorf("sync: malformed remote destination %q (want host:path)", destination)
	}
	// One round trip checks connectivity/trust/auth (ssh's own exit) and the
	// path (the remote test command's exit). BatchMode means ssh never prompts,
	// so an unknown/changed host key fails cleanly instead of hanging.
	remoteCmd := fmt.Sprintf("test -d %s && test -w %s", shellQuote(path), shellQuote(path))
	out, code, err := runSSHProbe("-o", "BatchMode=yes", "-o", "ConnectTimeout=10", host, remoteCmd)
	if err != nil {
		return res, fmt.Errorf("sync: running ssh: %w", err)
	}
	res.Checks = classifyRemoteProbe(host, path, out, code)
	return res, nil
}

// classifyRemoteProbe turns an ssh exit code + output into ordered checks. It
// is pure (no process/network) so it can be unit-tested on any platform.
//
//	code 0   → connected, trusted, path usable
//	code 255 → ssh-level failure; classify from output (trust / auth / reach)
//	other    → connected but the remote test failed (path missing/not writable)
func classifyRemoteProbe(host, path, out string, code int) []Check {
	switch {
	case code == 0:
		return []Check{
			{Name: "reachable", OK: true},
			{Name: "trusted", OK: true},
			{Name: "path writable", OK: true},
		}
	case code == 255 && strings.Contains(out, "Host key verification failed"):
		return []Check{
			{Name: "reachable", OK: true},
			{Name: "trusted", OK: false,
				Note: "host key not trusted — run `ssh " + host + "` once to verify and accept it"},
		}
	case code == 255 && strings.Contains(out, "Permission denied"):
		return []Check{
			{Name: "reachable", OK: true},
			{Name: "trusted", OK: true},
			{Name: "authenticated", OK: false,
				Note: "ssh permission denied — check your key or agent for " + host},
		}
	case code == 255:
		note := firstLine(out)
		if note == "" {
			note = "could not connect (host unreachable, unresolved, or timed out)"
		}
		return []Check{{Name: "reachable", OK: false, Note: note}}
	default:
		return []Check{
			{Name: "reachable", OK: true},
			{Name: "trusted", OK: true},
			{Name: "path writable", OK: false,
				Note: "path " + path + " is missing or not writable"},
		}
	}
}

func splitRemote(destination string) (host, path string, ok bool) {
	i := strings.IndexByte(destination, ':')
	if i <= 0 || i == len(destination)-1 {
		return "", "", false
	}
	return destination[:i], destination[i+1:], true
}

// shellQuote single-quotes s for a remote POSIX shell, escaping embedded quotes.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}
