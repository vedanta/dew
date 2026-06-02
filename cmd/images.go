package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/identity"
)

var imagesCmd = &cobra.Command{
	Use:     "images",
	GroupID: groupHealth,
	Short:   "List every image dew manages, across all repos",
	Long: `Show a global inventory of the encrypted images in ~/.dew/images — each one's
project, size, when it was last packed, and which repo owns it. Where 'dew status'
and 'dew list' describe the current repo, images spans every repo and runs from
anywhere.`,
	Example: "  dew images",
	Args:    cobra.NoArgs,
	RunE:    runImages,
}

type imageInfo struct {
	name    string
	project string
	size    int64
	modTime string
	owner   string
}

func runImages(cmd *cobra.Command, _ []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("images: %w", err)
	}
	return doImages(identity.NewPaths(home).ImagesDir, cmd.OutOrStdout())
}

func doImages(imagesDir string, out io.Writer) error {
	entries, err := os.ReadDir(imagesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			_, werr := io.WriteString(out, "No images yet.\n")
			return werr
		}
		return fmt.Errorf("images: read %s: %w", imagesDir, err)
	}

	var imgs []imageInfo
	for _, e := range entries {
		name := e.Name()
		// Skip directories and ownership markers — list only image files.
		if e.IsDir() || strings.HasSuffix(name, ".id") {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			return fmt.Errorf("images: stat %s: %w", name, infoErr)
		}
		owner, _ := readImageOwner(filepath.Join(imagesDir, name)) // "" if unmarked
		imgs = append(imgs, imageInfo{
			name:    name,
			project: strings.TrimSuffix(name, ".dew.age"),
			size:    info.Size(),
			modTime: info.ModTime().Format("2006-01-02 15:04"),
			owner:   shortID(owner),
		})
	}

	if len(imgs) == 0 {
		_, werr := io.WriteString(out, "No images yet.\n")
		return werr
	}
	sort.Slice(imgs, func(i, j int) bool { return imgs[i].name < imgs[j].name })

	var b strings.Builder
	tw := tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
	outf(tw, "IMAGE\tPROJECT\tSIZE\tLAST PACKED\tOWNER\n")
	for _, im := range imgs {
		outf(tw, "%s\t%s\t%s\t%s\t%s\n", im.name, im.project, humanBytes(im.size), im.modTime, im.owner)
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("images: %w", err)
	}
	_, werr := io.WriteString(out, b.String())
	return werr
}

func shortID(id string) string {
	if id == "" {
		return "-"
	}
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

func init() {
	rootCmd.AddCommand(imagesCmd)
}
