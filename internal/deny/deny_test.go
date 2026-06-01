package deny

import "testing"

func TestBuiltinRules(t *testing.T) {
	m := New(nil)
	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{"node_modules", true, true},
		{"sub/node_modules", true, true},
		{".DS_Store", false, true},
		{"logs/app.log", false, true},
		{".env.local", false, false},
		{"src", true, false},
		{"certs/dev.pem", false, false},
	}
	for _, c := range cases {
		if got := m.Match(c.path, c.isDir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestBuiltinListMatchesRules(t *testing.T) {
	got := Builtin()
	want := map[string]bool{
		"node_modules/": true, "dist/": true, "build/": true,
		"target/": true, ".venv/": true, "__pycache__/": true,
		".DS_Store": true, "*.log": true,
	}
	if len(got) != len(want) {
		t.Fatalf("Builtin() = %v, want %d entries", got, len(want))
	}
	for _, g := range got {
		if !want[g] {
			t.Errorf("unexpected built-in pattern %q", g)
		}
	}
}

func TestExtraPatterns(t *testing.T) {
	m := New([]string{"*.tmp", ".next/"})

	if !m.Match("scratch.tmp", false) {
		t.Error("*.tmp should match scratch.tmp")
	}
	if !m.Match(".next/cache.bin", false) {
		t.Error(".next/ should match .next/cache.bin")
	}
	if m.Match("keep.txt", false) {
		t.Error("keep.txt should not be denied")
	}
}
