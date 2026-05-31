package manifest

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestNewDefaults(t *testing.T) {
	m := New("liway")
	if m.Version != CurrentVersion {
		t.Errorf("version = %d, want %d", m.Version, CurrentVersion)
	}
	if m.Project != "liway" {
		t.Errorf("project = %q, want %q", m.Project, "liway")
	}
	if m.Image != "liway.dew.age" {
		t.Errorf("image = %q, want %q", m.Image, "liway.dew.age")
	}
	if len(m.Allow) != 0 {
		t.Errorf("allow = %v, want empty", m.Allow)
	}
	if err := m.Validate(); err != nil {
		t.Errorf("New() manifest should validate: %v", err)
	}
}

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := Path(dir)

	want := New("liway")
	want.Allow = []string{".env.local", "certs/dev.pem"}
	want.Deny = []string{"*.log"}

	if err := Save(path, want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	// Save must create the .dew directory and place the manifest there.
	if got := Path(dir); !strings.HasSuffix(got, filepath.Join(Dir, File)) {
		t.Errorf("Path = %q, missing %q suffix", got, filepath.Join(Dir, File))
	}

	got, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("round-trip mismatch:\n got  %+v\n want %+v", got, want)
	}
}

func TestValidateProjectName(t *testing.T) {
	valid := []string{"acme-api", "my_project", "a", "v1.2", "App.Name-2"}
	for _, n := range valid {
		if err := ValidateProjectName(n); err != nil {
			t.Errorf("ValidateProjectName(%q) = %v, want nil", n, err)
		}
	}
	invalid := []string{"", ".", "..", "...", "a/b", `a\b`, "my project", "a:b", "tab\tname", strings.Repeat("x", 65)}
	for _, n := range invalid {
		if err := ValidateProjectName(n); err == nil {
			t.Errorf("ValidateProjectName(%q) = nil, want error", n)
		}
	}
}

func TestAddRemoveAllow(t *testing.T) {
	m := New("p")

	if !m.AddAllow(".env.local") {
		t.Error("AddAllow should report a change on first add")
	}
	if m.AddAllow(".env.local") {
		t.Error("AddAllow should be a no-op (no change) on duplicate")
	}
	if len(m.Allow) != 1 {
		t.Fatalf("allow = %v, want one entry", m.Allow)
	}

	if !m.RemoveAllow(".env.local") {
		t.Error("RemoveAllow should report a change when present")
	}
	if m.RemoveAllow(".env.local") {
		t.Error("RemoveAllow should be a no-op (no change) when absent")
	}
	if len(m.Allow) != 0 {
		t.Errorf("allow = %v, want empty", m.Allow)
	}
}

func TestLoadMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	path := Path(dir)
	if err := writeFile(t, path, "this: : not: valid: yaml:\n  - ]["); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("expected error on malformed yaml, got nil")
	}
}

func TestLoadMissingRequiredFields(t *testing.T) {
	cases := map[string]string{
		"missing project": "version: 1\nimage: x.dew.age\n",
		"missing image":   "version: 1\nproject: liway\n",
		"missing version": "project: liway\nimage: x.dew.age\n",
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := Path(dir)
			if err := writeFile(t, path, body); err != nil {
				t.Fatal(err)
			}
			_, err := Load(path)
			if err == nil {
				t.Fatalf("expected validation error for %q, got nil", name)
			}
			// The error should name the missing field.
			field := strings.TrimPrefix(name, "missing ")
			if !strings.Contains(err.Error(), field) {
				t.Errorf("error %q should mention %q", err.Error(), field)
			}
		})
	}
}

func TestLoadNonexistent(t *testing.T) {
	if _, err := Load(Path(t.TempDir())); err == nil {
		t.Fatal("expected error loading a nonexistent manifest, got nil")
	}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		m       Manifest
		wantErr bool
	}{
		{"valid", Manifest{Version: 1, Project: "p", Image: "p.dew.age"}, false},
		{"missing version", Manifest{Project: "p", Image: "p.dew.age"}, true},
		{"unsupported version", Manifest{Version: 99, Project: "p", Image: "p.dew.age"}, true},
		{"missing project", Manifest{Version: 1, Image: "p.dew.age"}, true},
		{"missing image", Manifest{Version: 1, Project: "p"}, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := c.m.Validate()
			if (err != nil) != c.wantErr {
				t.Errorf("Validate() error = %v, wantErr = %v", err, c.wantErr)
			}
		})
	}
}

// writeFile creates the .dew dir and writes body to path for test fixtures.
func writeFile(t *testing.T, path, body string) error {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o600)
}
