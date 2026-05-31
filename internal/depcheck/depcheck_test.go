package depcheck

import (
	"strings"
	"testing"
)

func TestRequireToolMissing(t *testing.T) {
	err := RequireTool("dew-definitely-not-a-real-tool", "install it somehow")
	if err == nil {
		t.Fatal("expected error for a missing tool, got nil")
	}
	if !strings.Contains(err.Error(), "install it somehow") {
		t.Errorf("error should include the hint: %v", err)
	}
}

func TestRequireToolPresent(t *testing.T) {
	// `go` is always on PATH when the tests run.
	if err := RequireTool("go", "hint"); err != nil {
		t.Errorf("RequireTool(go) = %v, want nil", err)
	}
}
