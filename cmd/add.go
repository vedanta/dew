package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/manifest"
	"github.com/vedanta/dew/internal/scanner"
)

var addYes bool

var addCmd = &cobra.Command{
	Use:   "add <path>...",
	Short: "Add a file or directory to the manifest allow-list",
	Long:  "Add one or more paths to the allow-list. 'dew add .' adds discovered candidates (from 'dew scan') — not every file in the repo; use -y to accept all without prompting.",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAdd,
}

func runAdd(cmd *cobra.Command, args []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("add: %w", err)
	}
	// 'dew add .' means "add discovered candidates" — not every file in the repo.
	if len(args) == 1 && args[0] == "." {
		if err := loadGlobalDeny(); err != nil {
			return fmt.Errorf("add: %w", err)
		}
		return doAddDiscovered(root, cmd.InOrStdin(), cmd.OutOrStdout(), addYes)
	}
	return doAdd(root, args, cmd.OutOrStdout())
}

// doAddDiscovered adds scan candidates not already tracked, prompting Y/n for
// each (default yes) unless assumeYes is set. It never adds noise or
// repo-everything — only what scanner.Scan surfaces.
func doAddDiscovered(root string, in io.Reader, out io.Writer, assumeYes bool) error {
	m, err := manifest.Load(manifest.Path(root))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("add: no manifest found — run 'dew init' first")
		}
		return err
	}

	res, err := scanner.Scan(root, mergeDeny(globalDenyPatterns, m.Deny))
	if err != nil {
		return err
	}

	tracked := make(map[string]bool, len(m.Allow))
	for _, p := range m.Allow {
		tracked[p] = true
	}
	var pending []string
	for _, c := range res.Candidates {
		if !tracked[c] {
			pending = append(pending, c)
		}
	}
	if len(pending) == 0 {
		_, err := io.WriteString(out, "No new candidates to add.\n")
		return err
	}

	reader := bufio.NewReader(in)
	added := 0
	for _, c := range pending {
		if !confirmAdd(reader, out, c, assumeYes) {
			outf(out, "skipped %s\n", c)
			continue
		}
		m.AddAllow(c)
		added++
		outf(out, "added %s\n", c)
	}

	if added > 0 {
		if err := manifest.Save(manifest.Path(root), m); err != nil {
			return err
		}
	}
	outf(out, "Added %d of %d candidate(s).\n", added, len(pending))
	return nil
}

// outf writes interactive output, ignoring write errors (a failed write to
// stdout/stderr is neither recoverable nor actionable here).
func outf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

func confirmAdd(reader *bufio.Reader, out io.Writer, candidate string, assumeYes bool) bool {
	if assumeYes {
		return true
	}
	outf(out, "Add %s? [Y/n] ", candidate)
	line, rerr := reader.ReadString('\n')
	// No input available (EOF on an empty read): decline rather than default-add.
	if rerr != nil && strings.TrimSpace(line) == "" {
		return false
	}
	return isYes(line)
}

func isYes(line string) bool {
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "", "y", "yes":
		return true
	default:
		return false
	}
}

func doAdd(root string, args []string, out io.Writer) error {
	path := manifest.Path(root)
	m, err := manifest.Load(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("add: no manifest found — run 'dew init' first")
		}
		return err
	}

	var b strings.Builder
	changed := false
	for _, arg := range args {
		rel, relErr := repoRelPath(root, arg)
		if relErr != nil {
			return fmt.Errorf("add: %w", relErr)
		}
		if _, statErr := os.Stat(filepath.Join(root, rel)); errors.Is(statErr, os.ErrNotExist) {
			fmt.Fprintf(&b, "warning: %s does not exist yet\n", rel)
		}
		if m.AddAllow(rel) {
			fmt.Fprintf(&b, "added %s\n", rel)
			changed = true
		} else {
			fmt.Fprintf(&b, "already tracked: %s\n", rel)
		}
	}

	if changed {
		if err := manifest.Save(path, m); err != nil {
			return err
		}
	}
	_, err = io.WriteString(out, b.String())
	return err
}

func init() {
	addCmd.Flags().BoolVarP(&addYes, "yes", "y", false, "with 'add .', accept all discovered candidates without prompting")
	rootCmd.AddCommand(addCmd)
}
