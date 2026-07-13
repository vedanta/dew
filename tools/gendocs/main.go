// Command gendocs renders dew's command reference (site/reference.html) by
// walking the real Cobra command tree — a javadoc/phpdoc-style reference that
// is generated from the source of truth, so it can never drift from
// 'dew --help'. Run via `make docs`; CI fails if the committed page is stale.
package main

import (
	"fmt"
	"html/template"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/vedanta/dew/cmd"
)

// sinceByPath records the release each command first shipped in. Every listed
// command must have an entry — main() fails loudly on a gap, so a new command
// can't be added without recording its version (the completeness guard).
var sinceByPath = map[string]string{
	"dew keygen": "v0.1.0", "dew key": "v0.1.0", "dew key status": "v0.1.0",
	"dew key push": "v0.4.0", "dew key pull": "v0.4.0", "dew key devices": "v0.4.0",
	"dew init": "v0.1.0", "dew scan": "v0.1.0", "dew add": "v0.1.0",
	"dew remove": "v0.1.0", "dew list": "v0.1.0", "dew rules": "v0.1.0",
	"dew pack": "v0.1.0", "dew restore": "v0.1.0", "dew hydrate": "v0.2.0",
	"dew remote": "v0.3.0", "dew remote set": "v0.3.0", "dew remote unset": "v0.3.0",
	"dew remote test": "v0.3.0", "dew remote images": "v0.3.0",
	"dew sync": "v0.1.0", "dew sync pull": "v0.1.0",
	"dew status": "v0.1.0", "dew doctor": "v0.1.0",
	"dew images": "v0.1.0", "dew images rm": "v0.5.0",
	"dew clean": "v0.5.0", "dew upgrade": "v0.6.1", "dew version": "v0.1.0",
}

// flagSince tags individual flags introduced after their command. Keyed
// "<command path>\x00<flag>"; absent means "same as the command".
var flagSince = map[string]string{
	"dew pack\x00all":      "v0.6.0",
	"dew restore\x00image": "v0.6.0",
	"dew hydrate\x00image": "v0.6.0",
}

// seeAlsoByPath cross-links related commands (rendered as anchor links).
var seeAlsoByPath = map[string][]string{
	"dew pack":     {"dew restore", "dew sync", "dew rules"},
	"dew restore":  {"dew pack", "dew hydrate", "dew sync"},
	"dew hydrate":  {"dew restore", "dew pack"},
	"dew sync":     {"dew remote", "dew pack", "dew restore"},
	"dew add":      {"dew scan", "dew remove", "dew list", "dew rules"},
	"dew scan":     {"dew add", "dew rules"},
	"dew rules":    {"dew add", "dew pack"},
	"dew remove":   {"dew add", "dew list"},
	"dew list":     {"dew add", "dew rules"},
	"dew remote":   {"dew sync"},
	"dew clean":    {"dew images", "dew pack"},
	"dew images":   {"dew clean", "dew status"},
	"dew status":   {"dew doctor", "dew images"},
	"dew doctor":   {"dew status"},
	"dew keygen":   {"dew key push", "dew key pull"},
	"dew key push": {"dew key pull", "dew key devices"},
	"dew key pull": {"dew key push", "dew key devices"},
	"dew upgrade":  {"dew version"},
}

type flagDoc struct {
	Name, Short, Type, Default, Usage, Since string
}

type cmdDoc struct {
	Path, Name, Short string
	Anchor            string
	Synopsis          string
	Aliases           string
	Since             string
	LongBlocks        []block
	ExampleLines      []string
	Flags             []flagDoc
	SeeAlso           []seeRef
	Group             string
}

type block struct {
	Pre  bool
	Text string
}

type seeRef struct{ Label, Anchor string }

type groupDoc struct {
	Title    string
	Commands []cmdDoc
}

func anchorFor(path string) string {
	return strings.ReplaceAll(strings.TrimPrefix(path, "dew "), " ", "-")
}

// longToBlocks splits a Cobra Long into paragraphs; an indented block (embedded
// command example) becomes a <pre>, prose becomes a <p>.
func longToBlocks(long string) []block {
	long = strings.TrimSpace(long)
	if long == "" {
		return nil
	}
	var out []block
	for _, para := range strings.Split(long, "\n\n") {
		lines := strings.Split(para, "\n")
		indented := false
		for _, l := range lines {
			if strings.HasPrefix(l, "  ") {
				indented = true
				break
			}
		}
		if indented {
			out = append(out, block{Pre: true, Text: strings.TrimRight(para, "\n")})
		} else {
			out = append(out, block{Text: strings.Join(lines, " ")})
		}
	}
	return out
}

func synopsis(c *cobra.Command) string {
	// Argument spec is whatever follows the command name in Use ("add <path>...").
	args := ""
	if parts := strings.SplitN(c.Use, " ", 2); len(parts) == 2 {
		args = parts[1]
	}
	var flags []string
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		flags = append(flags, "["+flagToken(f)+"]")
	})
	sort.Strings(flags)
	syn := c.CommandPath()
	if args != "" {
		syn += " " + args
	}
	if len(flags) > 0 {
		syn += " " + strings.Join(flags, " ")
	}
	return syn
}

func flagToken(f *pflag.Flag) string {
	tok := "--" + f.Name
	if f.Value.Type() != "bool" {
		tok += " <" + f.Value.Type() + ">"
	}
	return tok
}

func buildCmd(c *cobra.Command, groupTitle string) cmdDoc {
	path := c.CommandPath()
	var flags []flagDoc
	c.LocalFlags().VisitAll(func(f *pflag.Flag) {
		if f.Name == "help" {
			return
		}
		def := f.DefValue
		if def == "false" || def == "" {
			def = ""
		}
		flags = append(flags, flagDoc{
			Name: f.Name, Short: f.Shorthand, Type: f.Value.Type(),
			Default: def, Usage: f.Usage, Since: flagSince[path+"\x00"+f.Name],
		})
	})
	sort.Slice(flags, func(i, j int) bool { return flags[i].Name < flags[j].Name })

	var see []seeRef
	for _, ref := range seeAlsoByPath[path] {
		see = append(see, seeRef{Label: strings.TrimPrefix(ref, "dew "), Anchor: anchorFor(ref)})
	}

	var exLines []string
	if ex := strings.TrimSpace(c.Example); ex != "" {
		exLines = strings.Split(ex, "\n")
	}

	return cmdDoc{
		Path: path, Name: c.Name(), Short: c.Short, Anchor: anchorFor(path),
		Synopsis: synopsis(c), Aliases: strings.Join(c.Aliases, ", "),
		Since: sinceByPath[path], LongBlocks: longToBlocks(c.Long),
		ExampleLines: exLines, Flags: flags, SeeAlso: see, Group: groupTitle,
	}
}

// walk collects documentable commands under root, grouped, recursing into
// parents (key, remote, images) so subcommands appear beside their parent.
func walk(root *cobra.Command) ([]groupDoc, []string) {
	groupTitles := map[string]string{}
	for _, g := range root.Groups() {
		groupTitles[g.ID] = strings.TrimSuffix(g.Title, ":")
	}
	order := []string{"Identity", "Repository", "Image", "Sync", "Health & inventory", "Global"}
	byTitle := map[string]*groupDoc{}
	var missing []string

	var visit func(c *cobra.Command, groupID string)
	visit = func(c *cobra.Command, groupID string) {
		if c.Hidden || c.Name() == "help" || c.Name() == "completion" {
			return
		}
		gid := groupID
		if c.GroupID != "" {
			gid = c.GroupID
		}
		title := groupTitles[gid]
		if title == "" {
			title = "Global" // version, upgrade — no Cobra group
		}
		if _, ok := byTitle[title]; !ok {
			byTitle[title] = &groupDoc{Title: title}
		}
		if _, ok := sinceByPath[c.CommandPath()]; !ok {
			missing = append(missing, c.CommandPath())
		}
		byTitle[title].Commands = append(byTitle[title].Commands, buildCmd(c, title))
		subs := c.Commands()
		sort.Slice(subs, func(i, j int) bool { return subs[i].Name() < subs[j].Name() })
		for _, sub := range subs {
			visit(sub, gid)
		}
	}
	for _, c := range root.Commands() {
		visit(c, "")
	}

	var groups []groupDoc
	for _, t := range order {
		if g, ok := byTitle[t]; ok {
			groups = append(groups, *g)
		}
	}
	return groups, missing
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "gendocs:", err)
		os.Exit(1)
	}
	fmt.Println("wrote site/reference.html")
}

func run() error {
	groups, missing := walk(cmd.Root())
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("missing 'since' for: %s\n  add each to sinceByPath in tools/gendocs/main.go",
			strings.Join(missing, ", "))
	}
	f, err := os.Create("site/reference.html")
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return page.Execute(f, groups)
}

var page = template.Must(template.New("ref").Parse(pageTmpl))

const pageTmpl = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>dew — command reference</title>
<meta name="description" content="The complete dew command reference — synopsis, parameters, exit status, since-version, and cross-links for every command. Generated from the CLI itself.">
<meta property="og:title" content="dew — command reference">
<meta property="og:description" content="Every dew command: synopsis, parameters, exit status, and cross-links. Generated from the CLI.">
<meta property="og:image" content="assets/comic.png">
<meta property="og:type" content="website">
<link rel="icon" href="assets/dew.png">
<link rel="stylesheet" href="style.css">
</head>
<body>

<header class="nav">
  <a class="brand" href="index.html">
    <img src="assets/dew.png" alt="dew" width="28" height="28">
    <span>dew</span>
  </a>
  <nav class="nav-links">
    <a href="index.html#how">How it works</a>
    <a href="index.html#features">Features</a>
    <a href="guide.html">Guide</a>
    <a href="reference.html">Reference</a>
    <a href="faq.html">FAQ</a>
    <a href="index.html#install">Install</a>
    <a class="ghost" href="https://github.com/vedanta/dew">GitHub ↗</a>
  </nav>
</header>

<div class="ref-layout">
  <aside class="ref-nav" aria-label="Command index">
    {{range .}}<div class="ref-nav-group"><h4>{{.Title}}</h4><ul>
      {{range .Commands}}<li><a href="#{{.Anchor}}"><code>{{.Path}}</code></a></li>{{end}}
    </ul></div>{{end}}
  </aside>

  <main class="ref-main">
    <section class="ref-intro">
      <h1>Command reference</h1>
      <p class="lede">Every command, with its synopsis, parameters, exit status, and the release it first shipped in — generated directly from the dew CLI, so it always matches <code>dew --help</code>.</p>
    </section>

    {{range .}}
    <section class="ref-group">
      <h2>{{.Title}}</h2>
      {{range .Commands}}
      <article class="ref-cmd" id="{{.Anchor}}">
        <h3><code>{{.Path}}</code>{{if .Since}} <span class="ver">since {{.Since}}</span>{{end}}</h3>
        <p class="ref-short">{{.Short}}</p>
        <div class="ref-field"><span class="ref-label">Synopsis</span><pre class="ref-syn">{{.Synopsis}}</pre></div>
        {{if .Aliases}}<div class="ref-field"><span class="ref-label">Alias</span><span class="ref-val"><code>{{.Aliases}}</code></span></div>{{end}}
        {{if .LongBlocks}}<div class="ref-desc">{{range .LongBlocks}}{{if .Pre}}<pre>{{.Text}}</pre>{{else}}<p>{{.Text}}</p>{{end}}{{end}}</div>{{end}}
        {{if .Flags}}<div class="ref-field"><span class="ref-label">Parameters</span>
          <table class="ref-params"><thead><tr><th>Flag</th><th>Type</th><th>Default</th><th>Description</th></tr></thead><tbody>
          {{range .Flags}}<tr>
            <td><code>{{if .Short}}-{{.Short}}, {{end}}--{{.Name}}</code>{{if .Since}} <span class="ver">{{.Since}}</span>{{end}}</td>
            <td><code>{{.Type}}</code></td>
            <td>{{if .Default}}<code>{{.Default}}</code>{{else}}—{{end}}</td>
            <td>{{.Usage}}</td>
          </tr>{{end}}
          </tbody></table></div>{{end}}
        {{if .ExampleLines}}<div class="ref-field"><span class="ref-label">Examples</span><pre class="ref-ex">{{range .ExampleLines}}{{.}}
{{end}}</pre></div>{{end}}
        <div class="ref-field"><span class="ref-label">Exit</span><span class="ref-val"><code>0</code> success · non-zero on error (<code>dew: error: …</code>)</span></div>
        {{if .SeeAlso}}<div class="ref-field"><span class="ref-label">See also</span><span class="ref-val">{{range $i, $s := .SeeAlso}}{{if $i}} · {{end}}<a href="#{{$s.Anchor}}"><code>{{$s.Label}}</code></a>{{end}}</span></div>{{end}}
      </article>
      {{end}}
    </section>
    {{end}}
  </main>
</div>

<footer class="footer">
  <img src="assets/dew.png" alt="" width="22" height="22">
  <span>dew — the local half of your repo, restored after every clone.</span>
  <span class="dot">·</span>
  <a href="index.html">Home</a>
  <span class="dot">·</span>
  <a href="guide.html">Guide</a>
  <span class="dot">·</span>
  <a href="https://github.com/vedanta/dew">GitHub</a>
</footer>

</body>
</html>
`
