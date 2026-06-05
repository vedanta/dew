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

var imagesRmYes bool

var imagesRmCmd = &cobra.Command{
	Use:     "rm <project>...",
	Aliases: []string{"remove"},
	Short:   "Delete packed image(s) from ~/.dew/images by project name",
	Long: `Delete one or more encrypted images from ~/.dew/images, named by project (the
PROJECT column 'dew images' prints; the trailing .dew.age is optional). Use it to
garbage-collect images whose repo is gone, or any image you no longer want — its
.id owner marker is removed alongside it.

This only deletes the local image file; it never touches your identity key or any
copy synced elsewhere, and it leaves repo manifests alone (to tear down the
current repo's manifest and image together, use 'dew clean'). You are asked to
confirm unless you pass --yes. Naming a project with no image is a harmless no-op.`,
	Example: `  dew images rm oldproject
  dew images rm a b c --yes`,
	Args: cobra.MinimumNArgs(1),
	RunE: runImagesRemove,
}

func runImagesRemove(cmd *cobra.Command, args []string) error {
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("images rm: %w", err)
	}
	return doImagesRemove(identity.NewPaths(home).ImagesDir, args, imagesRmYes, cmd.InOrStdin(), cmd.OutOrStdout())
}

func doImagesRemove(imagesDir string, projects []string, assumeYes bool, in io.Reader, out io.Writer) error {
	type target struct {
		project string
		path    string
	}
	var present []target
	var b strings.Builder
	for _, raw := range projects {
		name := strings.TrimSuffix(raw, ".dew.age")
		// Reject anything that isn't a bare project name — no path traversal out
		// of ~/.dew/images.
		if name == "" || strings.ContainsAny(name, `/\`) || strings.Contains(name, "..") {
			return fmt.Errorf("images rm: invalid project name %q", raw)
		}
		path := filepath.Join(imagesDir, name+".dew.age")
		if !fileExists(path) {
			outf(out, "no image for %q\n", name)
			continue
		}
		present = append(present, target{project: name, path: path})
	}
	if len(present) == 0 {
		return nil
	}

	if !assumeYes {
		outf(out, "This will permanently remove:\n")
		for _, t := range present {
			outf(out, "  - %s\n", t.path)
		}
		if !confirm(in, out, "Remove these image(s)?") {
			return errors.New("images rm: cancelled")
		}
	}

	for _, t := range present {
		if _, err := removeImageFile(t.path); err != nil {
			return fmt.Errorf("images rm: %w", err)
		}
		fmt.Fprintf(&b, "removed %s\n", t.path)
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
	imagesRmCmd.Flags().BoolVarP(&imagesRmYes, "yes", "y", false, "skip the confirmation prompt")
	imagesCmd.AddCommand(imagesRmCmd)
	rootCmd.AddCommand(imagesCmd)
}
