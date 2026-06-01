package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/config"
	"github.com/vedanta/dew/internal/deny"
	"github.com/vedanta/dew/internal/identity"
	"github.com/vedanta/dew/internal/manifest"
)

// globalDenyPatterns holds the user-level deny rules from ~/.dew/config.yaml for
// the current invocation. Entry points (run* funcs) populate it via
// loadGlobalDeny; it is merged with the per-manifest deny when building a deny
// matcher. Package-level like the cobra flag vars; tests may set it directly.
var globalDenyPatterns []string

// loadGlobalDeny reads the user-level deny-list from the global config.
func loadGlobalDeny() error {
	home, err := identity.DefaultHome()
	if err != nil {
		return err
	}
	cfg, err := config.Load(config.Path(home))
	if err != nil {
		return err
	}
	globalDenyPatterns = cfg.Deny
	return nil
}

// mergeDeny combines global and per-manifest deny patterns (global first).
func mergeDeny(global, perManifest []string) []string {
	if len(global) == 0 {
		return perManifest
	}
	out := make([]string, 0, len(global)+len(perManifest))
	out = append(out, global...)
	return append(out, perManifest...)
}

var rulesCmd = &cobra.Command{
	Use:     "rules",
	GroupID: groupRepo,
	Short:   "Show the effective allow-list and deny rules by layer",
	Long: `Show the effective configuration by layer: the repo allow-list, and the
three deny layers — built-in, global (~/.dew/config.yaml), and repo
(.dew/manifest.yaml). Useful for understanding why a path is included or skipped.`,
	Example: "  dew rules",
	Args:    cobra.NoArgs,
	RunE:    runRules,
}

func runRules(cmd *cobra.Command, _ []string) error {
	root, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("rules: %w", err)
	}
	if err := loadGlobalDeny(); err != nil {
		return fmt.Errorf("rules: %w", err)
	}
	return doRules(root, globalDenyPatterns, cmd.OutOrStdout())
}

func doRules(root string, globalDeny []string, out io.Writer) error {
	var b strings.Builder

	m, mErr := manifest.Load(manifest.Path(root))
	switch {
	case mErr == nil:
		writeRuleList(&b, "Allow-list (repo)", m.Allow)
	case errors.Is(mErr, os.ErrNotExist):
		fmt.Fprintln(&b, "Allow-list (repo): no manifest (run 'dew init')")
	default:
		return mErr
	}

	fmt.Fprintln(&b)
	writeRuleList(&b, "Deny — built-in", deny.Builtin())
	fmt.Fprintln(&b)
	writeRuleList(&b, "Deny — global (~/.dew/config.yaml)", globalDeny)
	fmt.Fprintln(&b)
	var repoDeny []string
	if mErr == nil {
		repoDeny = m.Deny
	}
	writeRuleList(&b, "Deny — repo (.dew/manifest.yaml)", repoDeny)

	_, err := io.WriteString(out, b.String())
	return err
}

func writeRuleList(b *strings.Builder, label string, items []string) {
	if len(items) == 0 {
		fmt.Fprintf(b, "%s: (none)\n", label)
		return
	}
	fmt.Fprintf(b, "%s:\n", label)
	for _, it := range items {
		fmt.Fprintf(b, "  %s\n", it)
	}
}

func init() {
	rootCmd.AddCommand(rulesCmd)
}
