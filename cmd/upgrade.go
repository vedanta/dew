package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/vedanta/dew/internal/upgrade"
)

var (
	upgradeVersion string
	upgradeCheck   bool
	upgradeForce   bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Update dew itself to the latest release (or a chosen version)",
	Long: `Replace this dew binary with a newer release from GitHub: resolve the latest
version (or the exact tag you name with --version), download the build for this
platform, verify it against the release's checksums, and swap it in atomically.
--check only reports what's available and changes nothing.

If dew was installed with Homebrew, upgrade refuses and points you at
'brew upgrade --cask dew' instead — replacing a brew-managed binary corrupts
brew's own bookkeeping (--force overrides if you know what you're doing).
After an upgrade, 'dew version' confirms what's running.`,
	Example: `  dew upgrade                    # → the latest release
  dew upgrade --check            # what would happen; changes nothing
  dew upgrade --version v0.6.0   # → that exact release`,
	Args: cobra.NoArgs,
	RunE: runUpgrade,
}

func runUpgrade(cmd *cobra.Command, _ []string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("upgrade: locate running binary: %w", err)
	}
	return doUpgrade(exe, version, upgradeVersion, upgradeCheck, upgradeForce, cmd.OutOrStdout())
}

// doUpgrade orchestrates the self-update. Network and file mechanics live in
// internal/upgrade; this stitches them together against the running binary.
func doUpgrade(exePath, current, wantTag string, check, force bool, out io.Writer) error {
	rel, err := upgrade.FetchRelease(wantTag)
	if err != nil {
		return err
	}
	target := rel.Version()

	brew := upgrade.BrewManaged(exePath)
	if check {
		outf(out, "current:   dew %s\navailable: dew %s (%s)\n", current, target, rel.Tag)
		switch {
		case current == target:
			outf(out, "Already up to date.\n")
		case brew:
			outf(out, "Installed via Homebrew — run 'brew upgrade --cask dew' to install it.\n")
		default:
			outf(out, "Run 'dew upgrade' to install it.\n")
		}
		return nil
	}
	if brew && !force {
		return errors.New("upgrade: this dew is managed by Homebrew — run 'brew upgrade --cask dew' instead\n" +
			"  (--force replaces the binary anyway, but brew will no longer know what's installed)")
	}

	if current == target && !force {
		_, err := fmt.Fprintf(out, "dew %s is already installed — nothing to do. (--force reinstalls.)\n", target)
		return err
	}

	assetName := upgrade.AssetName(target, runtime.GOOS, runtime.GOARCH)
	asset, err := upgrade.FindAsset(rel, assetName)
	if err != nil {
		return err
	}
	sums, err := upgrade.FindAsset(rel, "checksums.txt")
	if err != nil {
		return err
	}

	outf(out, "Downloading %s …\n", assetName)
	archive, err := upgrade.Download(asset)
	if err != nil {
		return err
	}
	sumData, err := upgrade.Download(sums)
	if err != nil {
		return err
	}
	want, ok := upgrade.ParseChecksums(sumData)[assetName]
	if !ok {
		return fmt.Errorf("upgrade: checksums.txt has no entry for %s", assetName)
	}
	if err := upgrade.VerifySHA256(archive, want); err != nil {
		return err
	}

	bin, err := upgrade.ExtractBinary(archive, runtime.GOOS == "windows")
	if err != nil {
		return err
	}
	if err := upgrade.ReplaceExecutable(exePath, bin); err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "Upgraded dew %s → %s. Run 'dew version' to confirm.\n", current, target)
	return err
}

func init() {
	upgradeCmd.Flags().StringVar(&upgradeVersion, "version", "", "install this exact release tag (e.g. v0.6.0) instead of the latest")
	upgradeCmd.Flags().BoolVar(&upgradeCheck, "check", false, "report the current and latest versions; change nothing")
	upgradeCmd.Flags().BoolVar(&upgradeForce, "force", false, "reinstall even if current, or replace a brew-managed binary")
	rootCmd.AddCommand(upgradeCmd)
}
