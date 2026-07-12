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
		{".next", true, true},
		{"apps/web/.nuxt", true, true},
		{"coverage", true, true},
		{".cache", true, true},
		{"packages/ui/.turbo", true, true},
		{".parcel-cache", true, true},
		{"coverage", false, false},
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
		".next/": true, ".nuxt/": true, "coverage/": true,
		".cache/": true, ".turbo/": true, ".parcel-cache/": true,
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
	m := New([]string{"*.tmp", ".gradle/"})

	if !m.Match("scratch.tmp", false) {
		t.Error("*.tmp should match scratch.tmp")
	}
	if !m.Match(".gradle/cache.bin", false) {
		t.Error(".gradle/ should match .gradle/cache.bin")
	}
	if m.Match("keep.txt", false) {
		t.Error("keep.txt should not be denied")
	}
}
