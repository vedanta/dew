package cmd

import (
	"bytes"
	"runtime"
	"strings"
	"testing"
)

func TestWriteVersion(t *testing.T) {
	var out bytes.Buffer
	if err := writeVersion(&out); err != nil {
		t.Fatalf("writeVersion: %v", err)
	}
	s := out.String()
	for _, want := range []string{"dew " + version, "commit:", "built:", "go:", runtime.Version()} {
		if !strings.Contains(s, want) {
			t.Errorf("version output missing %q:\n%s", want, s)
		}
	}
}
