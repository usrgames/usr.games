// Package sitepages writes the pages of the site from games.json and the templates beside this
// file. It has no code of its own outside its test: the pages are committed, Cloudflare serves
// them as they are, and no Go runs when the site is deployed.
//
// To write them again after a change to games.json or a template:
//
//	go test ./internal/sitepages -run TestThePagesAreWhatTheSourceSays -update
//
// or `make site`, which also draws the brand's files.
package sitepages

import (
	"bytes"
	"flag"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/usrgames/usr.games/internal/games"
)

var update = flag.Bool("update", false, "write the pages again from games.json and the templates")

func root(path string) string { return filepath.Join("..", "..", filepath.FromSlash(path)) }

// page is what a template is given.
type page struct {
	Title    string
	GoTags   template.HTML // the tags the go command reads, one go-import for every module
	Describe template.HTML // the description, and on the front page the link-preview tags
	Home     bool
	NoIndex  bool
	Script   bool
	Count    string       // "3 games."
	Games    []games.Game // A to Z, as a listing prints them
	Game     *games.Game
	Others   []games.Game
	Example  string // a game to name in the launcher's examples
}

// goTags returns one go-import tag for every module and, on a module's own page, its go-source.
// They are built here and not in a template because html/template will not fill in the content
// of a meta tag.
func goTags(modules []string, source string) template.HTML {
	var b strings.Builder
	for _, m := range modules {
		fmt.Fprintf(&b, "<meta name=\"go-import\" content=\"usr.games/%s git https://github.com/usrgames/%s\">\n", m, m)
	}
	if source != "" {
		repo := "https://github.com/usrgames/" + source
		fmt.Fprintf(&b, "<meta name=\"go-source\" content=\"usr.games/%s %s %s/tree/main{/dir} %s/blob/main{/dir}/{file}#L{line}\">\n", source, repo, repo, repo)
	}
	return template.HTML(b.String())
}

func describe(title, text string, card bool) template.HTML {
	meta := func(key, name, content string) string {
		return "<meta " + key + "=\"" + name + "\" content=\"" + html.EscapeString(content) + "\">\n"
	}
	out := meta("name", "description", text)
	if card {
		out += meta("property", "og:type", "website") +
			meta("property", "og:url", "https://usr.games/") +
			meta("property", "og:title", title) +
			meta("property", "og:description", text) +
			meta("property", "og:image", "https://usr.games/card.png") +
			meta("property", "og:image:width", "1280") +
			meta("property", "og:image:height", "640") +
			meta("property", "og:image:alt", "The sign #!/usr/games in green cells on a dark screen, and under it: an arcade in your terminal. free play.") +
			meta("name", "twitter:card", "summary_large_image")
	}
	return template.HTML(out)
}

// pages returns every page of the site by its path in site/.
func pages(t *testing.T) map[string]string {
	t.Helper()
	list, err := games.Load(root("games.json"))
	if err != nil {
		t.Fatal(err)
	}
	tmpl, err := template.New("").Funcs(template.FuncMap{
		// The width a drawn name is given in a page: ten units a cell, six cells a letter.
		"namew": func(name string, more int) int { return (len(name) + more) * 60 },
	}).ParseGlob(filepath.Join("templates", "*.tmpl"))
	if err != nil {
		t.Fatal(err)
	}

	modules, sorted := games.Modules(list), games.ByName(list)
	count := fmt.Sprintf("%d games.", len(list))
	if len(list) == 1 {
		count = "1 game."
	}
	base := func(title, text, source string) page {
		return page{Title: title, GoTags: goTags(modules, source), Describe: describe(title, text, false), Games: sorted, Count: count, Example: list[0].Name}
	}

	out := map[string]string{}
	render := func(path, name string, p page) {
		var b bytes.Buffer
		if err := tmpl.ExecuteTemplate(&b, name, p); err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		out[path] = b.String()
	}

	const pitch = "Games that run in your terminal. Play them here, or take them home. Free play."
	home := base("usr.games: an arcade in your terminal", pitch, "")
	home.Home, home.Script = true, true
	home.Describe = describe(home.Title, pitch, true)
	render("index.html", "index", home)

	for i := range sorted {
		g := sorted[i]
		p := base(g.Name+"(6): usr.games", g.Blurb, g.Name)
		p.Game = &g
		for _, other := range sorted {
			if other.Name != g.Name {
				p.Others = append(p.Others, other)
			}
		}
		render(g.Name+".html", "game", p)
	}
	render("tty1.html", "tty1", base("tty1: the usr.games launcher", "One command that installs the games, keeps them current, and starts them.", "tty1"))
	render("arcade.html", "arcade", base("arcade: the usr.games server", "What the arcade's server does, and what it keeps.", "arcade"))
	missing := base("No such file or directory: usr.games", "Nothing lives at this address.", "")
	missing.NoIndex = true
	render("404.html", "404", missing)
	return out
}

func TestThePagesAreWhatTheSourceSays(t *testing.T) {
	for path, want := range pages(t) {
		file := root("site/" + path)
		if *update {
			if err := os.WriteFile(file, []byte(want), 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(file)
		if err != nil {
			t.Errorf("wanted site/%s (run with -update to write it): %v", path, err)
			continue
		}
		if string(got) != want {
			t.Errorf("site/%s is not what games.json and the templates say (run with -update, or `make site`, to write it again)", path)
		}
	}
}

// A page in site/ that nothing here writes was made by hand or left behind by a game that has
// gone: either way it would be served, and stale.
func TestEveryPageOfTheSiteIsWrittenHere(t *testing.T) {
	made := pages(t)
	found, err := filepath.Glob(root("site/*.html"))
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range found {
		if _, ok := made[filepath.Base(file)]; !ok {
			t.Errorf("site/%s is written by nothing here: remove it, or add it to the pages", filepath.Base(file))
		}
	}
}
