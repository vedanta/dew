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
		{"ios/Pods", true, true},
		{"DerivedData", true, true},
		{"android/.gradle", true, true},
		{"android/app/.cxx", true, true},
		{".expo", true, true},
		{"Pods", false, false},
		{"tsconfig.tsbuildinfo", false, true},
		{"packages/web/app.tsbuildinfo", false, true},
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
		"Pods/": true, "DerivedData/": true, ".gradle/": true,
		".cxx/": true, ".expo/": true,
		".DS_Store": true, "*.log": true, "*.tsbuildinfo": true,
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

func TestNegationOverridesBuiltin(t *testing.T) {
	m := New([]string{"!.next/", "!keep.log"})
	cases := []struct {
		path  string
		isDir bool
		want  bool
	}{
		{".next", true, false}, // rescued from built-in
		{"apps/web/.next", true, false},
		{".nuxt", true, true},      // other built-ins untouched
		{"keep.log", false, false}, // rescued from built-in *.log
		{"other.log", false, true},
	}
	for _, c := range cases {
		if got := m.Match(c.path, c.isDir); got != c.want {
			t.Errorf("Match(%q, dir=%v) = %v, want %v", c.path, c.isDir, got, c.want)
		}
	}
}

func TestNegationLastMatchWins(t *testing.T) {
	// Within/across layers: the last matching rule decides.
	m := New([]string{"*.tmp", "!keep.tmp"})
	if m.Match("keep.tmp", false) {
		t.Error("!keep.tmp should rescue keep.tmp from *.tmp")
	}
	if !m.Match("scratch.tmp", false) {
		t.Error("*.tmp should still deny scratch.tmp")
	}

	// A later layer can re-deny what an earlier one rescued (global "!Pods/",
	// repo "Pods/").
	redeny := New([]string{"!Pods/", "Pods/"})
	if !redeny.Match("Pods", true) {
		t.Error("later Pods/ should re-deny after !Pods/")
	}
	rescue := New([]string{"Pods/", "!Pods/"})
	if rescue.Match("Pods", true) {
		t.Error("later !Pods/ should rescue after Pods/")
	}
}

func TestNegationIgnoresBlankAndComments(t *testing.T) {
	m := New([]string{"", "  ", "# comment", "*.tmp"})
	if !m.Match("a.tmp", false) {
		t.Error("*.tmp should survive blank/comment lines")
	}
	if m.Match("readme.md", false) {
		t.Error("blank/comment lines must not deny anything")
	}
}
