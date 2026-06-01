package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/crypto"
	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
)

var doctorCmd = &cobra.Command{
	Use:     "doctor",
	GroupID: groupHealth,
	Short:   "Diagnose hydration problems and recommend next actions",
	Long: `Diagnose the highest-priority problem and recommend the exact next command
(missing identity/manifest/image, undecryptable image, missing tracked files, …),
or report "Repository fully hydrated." It verifies the image actually decrypts,
not just that it exists.`,
	Example: "  dew doctor",
	Args:    cobra.NoArgs,
	RunE:    runDoctor,
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("doctor: %w", err)
	}
	home, err := identity.DefaultHome()
	if err != nil {
		return fmt.Errorf("doctor: %w", err)
	}
	return doDoctor(root, identity.NewPaths(home), cmd.OutOrStdout())
}

func doDoctor(root string, p identity.Paths, out io.Writer) error {
	var b strings.Builder

	problem, rec, err := diagnose(root, p, &b)
	if err != nil {
		return err
	}
	if problem == "" {
		fmt.Fprintln(&b, "\nRepository fully hydrated.")
	} else {
		fmt.Fprintf(&b, "\nProblem:\n  %s\n", problem)
		fmt.Fprintf(&b, "\nRecommended action:\n  %s\n", rec)
	}

	_, werr := io.WriteString(out, b.String())
	return werr
}

// diagnose writes confirmed-good facts to b and returns the first (highest
// priority) problem and its recommended action, or empty strings if healthy.
func diagnose(root string, p identity.Paths, b *strings.Builder) (problem, rec string, err error) {
	st, err := identity.Inspect(p)
	if err != nil {
		return "", "", err
	}
	if !st.Present {
		return "No identity found.", "Run 'dew keygen' to create your identity.", nil
	}
	fmt.Fprintln(b, "Identity:  present")

	m, mErr := manifest.Load(manifest.Path(root))
	switch {
	case errors.Is(mErr, os.ErrNotExist):
		return "No manifest in this repo.", "Run 'dew init' (or 'dew init --from-gitignore').", nil
	case mErr != nil:
		return "Manifest is invalid: " + mErr.Error(), "Fix .dew/manifest.yaml.", nil
	}
	fmt.Fprintln(b, "Manifest:  valid")

	if len(m.Allow) == 0 {
		return "Manifest tracks no files.", "Add files with 'dew add <path>' or 'dew scan'.", nil
	}

	imagePath := filepath.Join(p.ImagesDir, m.Image)
	imagePresent := fileExists(imagePath)
	missing := missingTracked(root, m.Allow)

	if !imagePresent {
		if len(missing) == 0 {
			return "No image has been packed yet.", "Run 'dew pack' to create the encrypted image.", nil
		}
		return fmt.Sprintf("No image, and %d tracked file(s) missing.", len(missing)),
			"Pack on a source machine, or 'dew sync pull' then 'dew restore'.", nil
	}
	fmt.Fprintln(b, "Image:     present")

	if vErr := verifyImage(imagePath, p.KeyFile); vErr != nil {
		return "Image cannot be decrypted with this identity (wrong key or corrupt).",
			"Verify ~/.dew/identity.age.key matches the image's recipient.", nil
	}

	if len(missing) > 0 {
		return fmt.Sprintf("%d tracked file(s) missing: %s", len(missing), strings.Join(limit(missing, 5), ", ")),
			"Run 'dew restore'.", nil
	}
	return "", "", nil
}

func missingTracked(root string, allow []string) []string {
	var missing []string
	for _, rel := range allow {
		if !fileExists(filepath.Join(root, filepath.FromSlash(rel))) {
			missing = append(missing, rel)
		}
	}
	return missing
}

// verifyImage confirms the image decrypts with the identity and authenticates
// end to end (age verifies the stream as it is read).
func verifyImage(imagePath, keyFile string) error {
	f, err := os.Open(imagePath) //nolint:gosec // G304: image path is dew-home-local
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()

	r, err := crypto.DecryptReader(f, keyFile)
	if err != nil {
		return err
	}
	_, err = io.Copy(io.Discard, r) //nolint:gosec // G110: image is the user's own bounded data
	return err
}

func limit(s []string, n int) []string {
	if len(s) <= n {
		return s
	}
	return append(s[:n:n], fmt.Sprintf("(+%d more)", len(s)-n))
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}
