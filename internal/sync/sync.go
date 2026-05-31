package sync

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/vedanta/dew/internal/depcheck"
)

const scpHint = "install the OpenSSH client (e.g. 'brew install openssh' or 'apt-get install openssh-client')"

// runScp executes scp; overridable in tests. Arguments are passed as argv (not
// a shell line), so a destination string can never inject a command.
var runScp = func(args ...string) error {
	cmd := exec.Command("scp", args...) //nolint:gosec // G204: args are dew-controlled (image path + configured destination)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("sync: scp failed: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// IsRemote reports whether destination is an scp-style remote (host:path)
// rather than a local or mounted path. A Windows drive (C:\...) is local.
func IsRemote(destination string) bool {
	i := strings.IndexByte(destination, ':')
	if i <= 0 {
		return false
	}
	host := destination[:i]
	if strings.ContainsAny(host, "/\\") {
		return false // the colon is inside a path, not a host separator
	}
	if len(host) == 1 {
		return false // Windows drive letter
	}
	return true
}

// Push copies the local image to destination (a directory). The remote name is
// the local image's base name. Remote destinations go through scp; local ones
// use a pure-Go copy.
func Push(localImage, destination string) error {
	if err := guardNotKey(localImage); err != nil {
		return err
	}
	name := filepath.Base(localImage)
	if IsRemote(destination) {
		if err := depcheck.RequireTool("scp", scpHint); err != nil {
			return err
		}
		return runScp(localImage, remoteJoin(destination, name))
	}
	return copyFile(localImage, filepath.Join(destination, name))
}

// Pull copies the image from destination into localImage.
func Pull(localImage, destination string) error {
	if err := guardNotKey(localImage); err != nil {
		return err
	}
	name := filepath.Base(localImage)
	if IsRemote(destination) {
		if err := depcheck.RequireTool("scp", scpHint); err != nil {
			return err
		}
		return runScp(remoteJoin(destination, name), localImage)
	}
	return copyFile(filepath.Join(destination, name), localImage)
}

func remoteJoin(destination, name string) string {
	return strings.TrimRight(destination, "/") + "/" + name
}

// guardNotKey refuses to transfer anything that looks like a private key. Sync
// moves encrypted images only — never identities.
func guardNotKey(p string) error {
	base := filepath.Base(p)
	if strings.HasSuffix(base, ".key") || strings.Contains(base, "identity.age") {
		return fmt.Errorf("sync: refusing to transfer key-like file %q (images only)", base)
	}
	return nil
}

// copyFile copies src to dst atomically (temp file + rename), creating dst's
// directory.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return fmt.Errorf("sync: create dir: %w", err)
	}
	in, err := os.Open(src) //nolint:gosec // G304: src is the dew image path or configured destination
	if err != nil {
		return fmt.Errorf("sync: open %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	tmp, err := os.CreateTemp(filepath.Dir(dst), ".dew-sync-*")
	if err != nil {
		return fmt.Errorf("sync: create temp: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = os.Remove(tmpName) }() // no-op once renamed

	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync: copy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("sync: close temp: %w", err)
	}
	if err := os.Rename(tmpName, dst); err != nil {
		return fmt.Errorf("sync: install %s: %w", dst, err)
	}
	return nil
}
