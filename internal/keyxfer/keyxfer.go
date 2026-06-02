// Package keyxfer provisions the dew identity onto another machine over SSH —
// the one-time key bootstrap for a second machine.
//
// It is deliberately separate from sync: sync moves only encrypted images and
// never the private key; this moves the private key, by explicit user action,
// over the user's own SSH trust. Host-key verification is left to OpenSSH and
// never weakened (ssh runs in BatchMode, so an unknown host key fails rather
// than being auto-accepted).
package keyxfer

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// Remote paths, relative to the target's home (resolved by the remote shell /
// SFTP) — relative paths dodge the ~-expansion quirks of modern scp.
const (
	remoteKeyPath = ".dew/identity.age.key"
	inspectMarker = "__DEW_KEY__"
	connectHint   = "run 'ssh <host>' once to verify and accept its host key, then retry"
)

// ErrDifferentIdentity reports that the target already holds an identity whose
// public key differs from the one being pushed — refused unless force is set.
var ErrDifferentIdentity = errors.New("target already has a different identity")

// ErrUnreachable reports the target could not be reached, or its host key isn't
// verified (ssh refused in batch mode).
var ErrUnreachable = errors.New("target unreachable or host key not verified")

// Outcome of a Push.
type Outcome int

const (
	// Provisioned means the identity was written to the target.
	Provisioned Outcome = iota
	// AlreadyPresent means the target already had the same identity.
	AlreadyPresent
)

// sshRun runs command on host over ssh and returns combined output and the exit
// code (0 on success, -1 if ssh couldn't start). BatchMode preserves strict
// host-key checking. Overridable in tests.
var sshRun = func(host, command string) (string, int, error) {
	cmd := exec.Command("ssh", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", host, command) //nolint:gosec // G204: host is the user's target; command is a fixed dew-controlled string
	out, err := cmd.CombinedOutput()
	s := strings.TrimSpace(string(out))
	if err == nil {
		return s, 0, nil
	}
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return s, ee.ExitCode(), nil
	}
	return s, -1, err
}

// scpUpload copies localPath to host:remotePath. Overridable in tests.
var scpUpload = func(localPath, host, remotePath string) error {
	cmd := exec.Command("scp", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", localPath, host+":"+remotePath) //nolint:gosec // G204: localPath is dew-home-local; host is the user's target
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keyxfer: scp %s → %s: %w: %s", localPath, host, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// scpDownload copies host:remotePath to localPath. Overridable in tests.
var scpDownload = func(host, remotePath, localPath string) error {
	cmd := exec.Command("scp", "-o", "BatchMode=yes", "-o", "ConnectTimeout=10", host+":"+remotePath, localPath) //nolint:gosec // G204: host is the user's target; localPath is dew-home-local
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("keyxfer: scp %s:%s → local: %w: %s", host, remotePath, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// RemotePublicKey returns the public key of the identity on host, erroring if
// host has no usable identity or can't be reached.
func RemotePublicKey(host string) (string, error) {
	pub, hasKey, err := inspectTarget(host)
	switch {
	case err != nil:
		return "", err
	case pub != "":
		return pub, nil
	case hasKey:
		return "", fmt.Errorf("keyxfer: %s has a private key but no readable public key (run 'dew key status' there)", host)
	default:
		return "", fmt.Errorf("keyxfer: no dew identity on %s", host)
	}
}

// Download copies host's private identity key into localPath (typically a temp
// file the caller verifies before installing).
func Download(host, localPath string) error {
	return scpDownload(host, remoteKeyPath, localPath)
}

// Push provisions the local identity (private key at keyFile, public key
// localPub) onto host's ~/.dew. It refuses to overwrite a different identity
// unless force is set, and verifies the target's public key matches afterward.
func Push(host, keyFile, localPub string, force bool) (Outcome, error) {
	existingPub, hasKey, err := inspectTarget(host)
	if err != nil {
		return 0, err
	}
	switch {
	case existingPub == localPub:
		return AlreadyPresent, nil
	case (existingPub != "" || hasKey) && !force:
		return 0, ErrDifferentIdentity
	}

	if err := run(host, "mkdir -p ~/.dew && chmod 700 ~/.dew", "prepare ~/.dew"); err != nil {
		return 0, err
	}
	if err := scpUpload(keyFile, host, remoteKeyPath); err != nil {
		return 0, err
	}
	// Write the public key from the recipient string (no dependency on a local
	// .pub file) and lock down the private key's permissions.
	finalize := fmt.Sprintf("printf '%%s\\n' %s > ~/.dew/identity.age.pub && chmod 600 ~/.dew/identity.age.key", shellQuote(localPub))
	if err := run(host, finalize, "install identity"); err != nil {
		return 0, err
	}

	back, _, err := sshRun(host, "cat ~/.dew/identity.age.pub 2>/dev/null")
	if err != nil {
		return 0, fmt.Errorf("keyxfer: verify on %s: %w", host, err)
	}
	if strings.TrimSpace(back) != localPub {
		return 0, fmt.Errorf("keyxfer: verification failed on %s — public key is %q, expected %q",
			host, strings.TrimSpace(back), localPub)
	}
	return Provisioned, nil
}

// inspectTarget reads the target's current public key ("" if none) and whether
// a private key file is present.
func inspectTarget(host string) (pub string, hasKey bool, err error) {
	cmd := "cat ~/.dew/identity.age.pub 2>/dev/null; echo " + inspectMarker + "; test -f ~/.dew/identity.age.key && echo yes || true"
	out, code, err := sshRun(host, cmd)
	if err != nil || code == 255 {
		return "", false, fmt.Errorf("keyxfer: %w — %s: %s", ErrUnreachable, connectHint, firstLine(out))
	}
	parts := strings.SplitN(out, inspectMarker, 2)
	pub = strings.TrimSpace(parts[0])
	if len(parts) == 2 && strings.Contains(parts[1], "yes") {
		hasKey = true
	}
	return pub, hasKey, nil
}

func run(host, command, what string) error {
	out, code, err := sshRun(host, command)
	if err != nil {
		return fmt.Errorf("keyxfer: %s on %s: %w", what, host, err)
	}
	if code != 0 {
		return fmt.Errorf("keyxfer: %s on %s failed: %s", what, host, firstLine(out))
	}
	return nil
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
