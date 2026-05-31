// Package depcheck verifies that external tools a command needs are available,
// failing gracefully with an install hint rather than a cryptic exec error.
package depcheck

import (
	"fmt"
	"os/exec"
)

// RequireTool returns an error (including hint) if name is not found on PATH.
// Commands call this only for the external tools they actually need, so a
// missing tool never breaks an unrelated command.
func RequireTool(name, hint string) error {
	if _, err := exec.LookPath(name); err != nil {
		return fmt.Errorf("required tool %q not found on PATH — %s", name, hint)
	}
	return nil
}
