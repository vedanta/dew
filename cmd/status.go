package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show identity, manifest, image, and hydration health",
	Args:  cobra.NoArgs,
	RunE:  runStatus,
}

func runStatus(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	return doStatus(root, identity.NewPaths(home), cmd.OutOrStdout())
}

func doStatus(root string, p identity.Paths, out io.Writer) error {
	var b strings.Builder

	st, err := identity.Inspect(p)
	if err != nil {
		return err
	}

	m, mErr := manifest.Load(manifest.Path(root))
	if mErr == nil {
		fmt.Fprintf(&b, "%-10s %s\n", "Project:", m.Project)
	} else if errors.Is(mErr, os.ErrNotExist) {
		fmt.Fprintf(&b, "%-10s %s\n", "Project:", "(none)")
	}

	writeStatusLine(&b, "Identity", boolLabel(st.Present, "Present", "Not found (run 'dew keygen')"))

	switch {
	case mErr == nil:
		writeStatusLine(&b, "Manifest", "Valid")
	case errors.Is(mErr, os.ErrNotExist):
		writeStatusLine(&b, "Manifest", "Not found (run 'dew init')")
	default:
		writeStatusLine(&b, "Manifest", "Invalid: "+mErr.Error())
	}

	if mErr == nil {
		imagePath := filepath.Join(p.ImagesDir, m.Image)
		imagePresent := fileExists(imagePath)
		writeStatusLine(&b, "Image", boolLabel(imagePresent, "Present", "Missing (run 'dew pack')"))
		fmt.Fprintf(&b, "%-10s %d\n", "Tracked:", len(m.Allow))
		writeStatusLine(&b, "Hydration", hydrationStatus(root, m.Allow, imagePresent))
	}

	// Sync configuration lands in Phase 6.
	writeStatusLine(&b, "Sync", "Not configured")

	_, err = io.WriteString(out, b.String())
	return err
}

func hydrationStatus(root string, allow []string, imagePresent bool) string {
	missing := 0
	for _, rel := range allow {
		if !fileExists(filepath.Join(root, filepath.FromSlash(rel))) {
			missing++
		}
	}
	switch {
	case len(allow) == 0:
		return "Empty (nothing tracked yet)"
	case missing == 0:
		return "Healthy"
	case imagePresent:
		return fmt.Sprintf("Incomplete: %d file(s) missing (run 'dew restore')", missing)
	default:
		return fmt.Sprintf("Incomplete: %d file(s) missing (no image — pack on a source machine)", missing)
	}
}

func writeStatusLine(b *strings.Builder, label, value string) {
	fmt.Fprintf(b, "%-10s %s\n", label+":", value)
}

func boolLabel(ok bool, yes, no string) string {
	if ok {
		return yes
	}
	return no
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func init() {
	rootCmd.AddCommand(statusCmd)
}
