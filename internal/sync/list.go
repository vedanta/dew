package sync

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vedanta/dew/internal/depcheck"
)

// imageSuffix is the extension every dew image carries.
const imageSuffix = ".dew.age"

// DestImage is one image found at a sync destination.
type DestImage struct {
	Name     string
	Size     int64     // bytes; -1 if unknown (remote, unparsable)
	Modified time.Time // zero if unknown (remote)
}

// List returns the dew images present at destination, sorted by name. Local and
// mounted paths are read in-process; remote host:path destinations are listed
// over ssh. A missing/empty destination yields an empty list, not an error.
func List(destination string) ([]DestImage, error) {
	if IsRemote(destination) {
		return listRemote(destination)
	}
	return listLocal(destination)
}

func listLocal(dir string) ([]DestImage, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil // not created yet — treat as empty
		}
		return nil, fmt.Errorf("sync: read %s: %w", dir, err)
	}
	var imgs []DestImage
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, imageSuffix) {
			continue
		}
		info, ierr := e.Info()
		if ierr != nil {
			return nil, fmt.Errorf("sync: stat %s: %w", name, ierr)
		}
		imgs = append(imgs, DestImage{Name: name, Size: info.Size(), Modified: info.ModTime()})
	}
	sortImages(imgs)
	return imgs, nil
}

func listRemote(destination string) ([]DestImage, error) {
	if err := depcheck.RequireTool("ssh", sshHint); err != nil {
		return nil, err
	}
	host, path, ok := splitRemote(destination)
	if !ok {
		return nil, fmt.Errorf("sync: malformed remote destination %q (want host:path)", destination)
	}
	// Quote the path (spaces/specials) but leave the glob unquoted so the remote
	// shell expands it. No matches → ls errors to stderr (suppressed) and a
	// non-zero exit; we treat empty output as "no images".
	remoteCmd := fmt.Sprintf("ls -l %s/*%s 2>/dev/null", shellQuote(path), imageSuffix)
	out, _, err := runSSH("-o", "BatchMode=yes", "-o", "ConnectTimeout=10", host, remoteCmd)
	if err != nil {
		return nil, fmt.Errorf("sync: running ssh: %w", err)
	}
	imgs := parseLSList(out)
	sortImages(imgs)
	return imgs, nil
}

// parseLSList extracts dew images from `ls -l` output. It is pure (no
// process/network) so it can be unit-tested on any platform. Only the size and
// the (base) name are read; the locale-dependent date is ignored.
func parseLSList(out string) []DestImage {
	var imgs []DestImage
	for _, line := range strings.Split(out, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue // "total N" headers, blanks, errors
		}
		name := filepath.Base(fields[len(fields)-1])
		if !strings.HasSuffix(name, imageSuffix) {
			continue
		}
		size := int64(-1)
		if n, err := strconv.ParseInt(fields[4], 10, 64); err == nil {
			size = n
		}
		imgs = append(imgs, DestImage{Name: name, Size: size})
	}
	return imgs
}

func sortImages(imgs []DestImage) {
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].Name < imgs[j].Name })
}
