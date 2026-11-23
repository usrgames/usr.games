// Package sitepages writes the pages of the site, and the devlog's feed, from games.json, the
// notes in plan/, the manual pages games.lock.json pins and the templates beside this file. It has no code of its own outside its test: the pages are committed, Cloudflare serves
// them as they are, and no Go runs when the site is deployed.
//
// To write them again after a change to games.json, a note, a pin or a template:
//
//	go test ./internal/sitepages -run TestThePagesAreWhatTheSourceSays -update
//
// or `make site`, which also draws the brand's files.
package sitepages

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"html"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/usrgames/usr.games/internal/games"
	"github.com/usrgames/usr.games/internal/plan"
)

var update = flag.Bool("update", false, "write the pages again from games.json, the notes and the templates")

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
	Example  string            // a game to name in the launcher's examples
	Notes    []plan.Note       // the devlog, newest first
	Contact  template.HTML     // where the notices say to write
	Man      map[string]string // the section of each program with a manual page on the site
	Manuals  []manual          // every manual page, A to Z
	Manual   *manual           // on a manual's own page, that manual
}

// manual is one pinned manual page: its program, its section, the release it came in, and its
// text, ready to be set in a page of the site.
type manual struct {
	Name, Section, Version string
	Body                   template.HTML
}

// manuals reads games.lock.json and returns the manual page of each release it pins, A to Z. A
// page goes on the site only as its release shipped it, and scripts/pin.sh checked that release:
// its file in man/ has to match the lock's hash, and man/ may hold nothing the lock does not name.
func manuals(t *testing.T, modules []string) []manual {
	t.Helper()
	data, err := os.ReadFile(root("games.lock.json"))
	if err != nil {
		t.Fatal(err)
	}
	var lock map[string]struct {
		Version string            `json:"version"`
		Files   map[string]string `json:"files"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&lock); err != nil {
		t.Fatalf("games.lock.json: %v", err)
	}
	named := map[string]bool{}
	var out []manual
	for name, pin := range lock {
		if !slices.Contains(modules, name) {
			t.Fatalf("games.lock.json: %q is neither a game of games.json nor one of the tools", name)
		}
		if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(pin.Version) {
			t.Fatalf("games.lock.json: %s: wanted a version such as v0.1.0, got %q", name, pin.Version)
		}
		page := regexp.MustCompile(`^` + regexp.QuoteMeta(name) + `\.([1-9])\.html$`)
		if len(pin.Files) != 1 {
			t.Fatalf("games.lock.json: %s: wanted one file, its manual page, got %d", name, len(pin.Files))
		}
		for file, sum := range pin.Files {
			named[file] = true
			section := page.FindStringSubmatch(file)
			if section == nil {
				t.Fatalf("games.lock.json: %s pins %s, which is not its manual page", name, file)
			}
			raw, err := os.ReadFile(root("man/" + file))
			if err != nil {
				t.Fatalf("games.lock.json pins man/%s (scripts/pin.sh %s %s writes it): %v", file, name, pin.Version, err)
			}
			if got := sha256.Sum256(raw); hex.EncodeToString(got[:]) != sum {
				t.Fatalf("man/%s is not the page games.lock.json pins: run scripts/pin.sh %s %s to pin it again", file, name, pin.Version)
			}
			out = append(out, manual{Name: name, Section: section[1], Version: pin.Version, Body: body(t, file, raw)})
		}
	}
	files, err := os.ReadDir(root("man"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	for _, f := range files {
		if !named[f.Name()] {
			t.Errorf("man/%s is pinned by nothing in games.lock.json: remove it, or pin it with scripts/pin.sh", f.Name())
		}
	}
	slices.SortFunc(out, func(a, b manual) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// body returns the text of a manual page as mandoc writes it, to be set in a page of the site:
// what is between mandoc's head, with its style block the site's policy refuses, and its foot,
// which names the machine mandoc ran on. Each heading goes one level down, under the page's
// own h1.
func body(t *testing.T, file string, raw []byte) template.HTML {
	t.Helper()
	const start, end = `<div class="manual-text">`, "</div>\n<table class=\"foot\">"
	s := string(raw)
	i, j := strings.Index(s, start), strings.LastIndex(s, end)
	if i < 0 || j < i || strings.Count(s, start) != 1 {
		t.Fatalf("man/%s: wanted a page as mandoc writes it, with one %s and its foot after it", file, start)
	}
	down := strings.NewReplacer("<h1", "<h2", "</h1>", "</h2>", "<h2", "<h3", "</h2>", "</h3>")
	return template.HTML(down.Replace(s[i+len(start) : j]))
}

// contact is the address the notices give. Cloudflare rewrites an email address it finds in a
// page into a script that prints it; the two comments tell it to leave this one as written. It
// is passed as a value because html/template drops a comment written in a template.
const contact = template.HTML("<!--email_off-->hello@usr.games<!--/email_off-->")

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

// feed returns the devlog as an Atom feed. A note has a day and no hour, so its time is midnight
// UTC, and the feed is as old as its newest note: nothing in it changes until a note does.
func feed(t *testing.T, notes []plan.Note) string {
	t.Helper()
	type link struct {
		Rel  string `xml:"rel,attr"`
		Type string `xml:"type,attr"`
		Href string `xml:"href,attr"`
	}
	type content struct {
		Type string `xml:"type,attr"`
		HTML string `xml:",chardata"`
	}
	type entry struct {
		Title   string  `xml:"title"`
		ID      string  `xml:"id"`
		Updated string  `xml:"updated"`
		Link    link    `xml:"link"`
		Content content `xml:"content"`
	}
	doc := struct {
		XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
		Title   string   `xml:"title"`
		ID      string   `xml:"id"`
		Updated string   `xml:"updated"`
		Author  string   `xml:"author>name"`
		Links   []link   `xml:"link"`
		Entries []entry  `xml:"entry"`
	}{
		Title:   "usr.games: the plan",
		ID:      "https://usr.games/plan",
		Updated: notes[0].Date.Format(time.RFC3339),
		Author:  "usr.games",
		Links: []link{
			{Rel: "self", Type: "application/atom+xml", Href: "https://usr.games/plan.xml"},
			{Rel: "alternate", Type: "text/html", Href: "https://usr.games/plan"},
		},
	}
	for _, n := range notes {
		var body strings.Builder
		for _, p := range n.Paras {
			if p.Command {
				body.WriteString("<pre>$ " + html.EscapeString(p.Text) + "</pre>")
			} else {
				body.WriteString("<p>" + html.EscapeString(p.Text) + "</p>")
			}
		}
		at := "https://usr.games/plan#" + n.ID
		doc.Entries = append(doc.Entries, entry{
			Title: n.Title, ID: at, Updated: n.Date.Format(time.RFC3339),
			Link:    link{Rel: "alternate", Type: "text/html", Href: at},
			Content: content{Type: "html", HTML: body.String()},
		})
	}
	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return xml.Header + string(out) + "\n"
}

// listing returns the list of games as tty1 reads it, at /etc/games.json: every game from A to
// Z, with what a listing shows of it. The launcher's `tty1 ls` is this file, so a game is on the
// floor and in the launcher from the same line of games.json.
func listing(t *testing.T, sorted []games.Game) string {
	t.Helper()
	type game struct {
		Name    string `json:"name"`
		Players string `json:"players"`
		Blurb   string `json:"blurb"`
	}
	doc := struct {
		Schema int    `json:"schema"`
		Games  []game `json:"games"`
	}{Schema: 1}
	for _, g := range sorted {
		doc.Games = append(doc.Games, game{g.Name, g.Players, g.Blurb})
	}
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return string(out) + "\n"
}

// pages returns every made text file of the site, the pages, the feed and the list of games, by
// its path in site/.
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

	notes, err := plan.Load(root("plan"))
	if err != nil {
		t.Fatal(err)
	}

	modules, sorted := games.Modules(list), games.ByName(list)
	count := fmt.Sprintf("%d games.", len(list))
	if len(list) == 1 {
		count = "1 game."
	}
	mans := manuals(t, modules)
	man := map[string]string{}
	for _, m := range mans {
		man[m.Name] = m.Section
	}
	base := func(title, text, source string) page {
		return page{Title: title, GoTags: goTags(modules, source), Describe: describe(title, text, false), Games: sorted, Count: count, Example: list[0].Name, Man: man}
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
	render("terminals.html", "terminals", base("Bring a good terminal: usr.games", "Which terminals show the games at their best, and what you see in the others.", ""))
	index := base("The manual: usr.games", "The manual pages of the games and of tty1, as each release ships them.", "")
	index.Manuals = mans
	render("man.html", "manindex", index)
	for i := range mans {
		m := &mans[i]
		name := m.Name + "(" + m.Section + ")"
		p := base("The manual of "+name+": usr.games", "The manual page of "+name+", as its release "+m.Version+" ships it.", "")
		p.Manual = m
		render("man/"+m.Name+".html", "man", p)
	}
	devlog := base("The plan: usr.games", "What we are building at usr.games, one part at a time. Newest first.", "")
	devlog.Notes = notes
	render("plan.html", "plan", devlog)
	out["plan.xml"] = feed(t, notes)
	out["etc/games.json"] = listing(t, sorted)
	privacy := base("Privacy: usr.games", "What usr.games keeps about its visitors and players: almost nothing.", "")
	privacy.Contact = contact
	render("privacy.html", "privacy", privacy)
	terms := base("Terms: usr.games", "The house rules of usr.games: free games, open code, and a relay for playing matches.", "")
	terms.Contact = contact
	render("terms.html", "terms", terms)
	missing := base("No such file or directory: usr.games", "Nothing lives at this address.", "")
	missing.NoIndex = true
	render("404.html", "404", missing)
	return out
}

func TestThePagesAreWhatTheSourceSays(t *testing.T) {
	for path, want := range pages(t) {
		file := root("site/" + path)
		if *update {
			if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
				t.Fatal(err)
			}
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
			t.Errorf("site/%s is not what its source says (run with -update, or `make site`, to write it again)", path)
		}
	}
}

// A page or a feed in site/ that nothing here writes was made by hand or left behind by a game
// that has gone: either way it would be served, and stale.
func TestEveryPageOfTheSiteIsWrittenHere(t *testing.T) {
	made := pages(t)
	var found []string
	for _, pattern := range []string{"site/*.html", "site/*.xml", "site/man/*.html"} {
		files, err := filepath.Glob(root(pattern))
		if err != nil {
			t.Fatal(err)
		}
		found = append(found, files...)
	}
	for _, file := range found {
		rel, err := filepath.Rel(root("site"), file)
		if err != nil {
			t.Fatal(err)
		}
		if _, ok := made[filepath.ToSlash(rel)]; !ok {
			t.Errorf("site/%s is written by nothing here: remove it, or add it to the pages", filepath.ToSlash(rel))
		}
	}
}
