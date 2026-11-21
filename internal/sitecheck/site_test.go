// Package sitecheck holds the tests of the finished site in ../../site: what is served, as it
// is served. It has no code of its own. The tests of how the pages and the drawings are made
// are beside what makes them, in internal/sitepages and internal/brand.
package sitecheck

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/usrgames/usr.games/internal/brand"
	"github.com/usrgames/usr.games/internal/games"
)

var site = filepath.Join("..", "..", "site")

// pages are the HTML files of the site, by their paths in site/.
var pages = func() []string {
	var found []string
	err := filepath.WalkDir(site, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == "media":
			return filepath.SkipDir
		case !d.IsDir() && filepath.Ext(path) == ".html":
			rel, err := filepath.Rel(site, path)
			if err != nil {
				return err
			}
			found = append(found, filepath.ToSlash(rel))
		}
		return nil
	})
	if err != nil {
		panic(err)
	}
	return found
}()

// read returns the text of a file of the repository, given its path from the root.
func read(t *testing.T, path ...string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(append([]string{"..", ".."}, path...)...))
	if err != nil {
		t.Fatalf("unable to read %s: %v", filepath.Join(path...), err)
	}
	return string(b)
}

func list(t *testing.T) []games.Game {
	t.Helper()
	gs, err := games.Load(filepath.Join("..", "..", "games.json"))
	if err != nil {
		t.Fatal(err)
	}
	return gs
}

// metas returns the content of every <meta name="name"> tag of a page.
func metas(page, name string) []string {
	re := regexp.MustCompile(`<meta\s+name="` + regexp.QuoteMeta(name) + `"\s+content="([^"]*)"\s*>`)
	var out []string
	for _, m := range re.FindAllStringSubmatch(page, -1) {
		out = append(out, m[1])
	}
	return out
}

// The tests below go over every page that is found. This one fails if a page was not.
func TestTheSiteHasItsPages(t *testing.T) {
	want := []string{"index.html", "404.html", "terminals.html"}
	for _, m := range games.Modules(list(t)) {
		want = append(want, m+".html")
	}
	for _, name := range want {
		if !slices.Contains(pages, name) {
			t.Errorf("wanted %s among the pages of the site, got %v", name, pages)
		}
	}
}

// `go install usr.games/rustwake@latest` asks https://usr.games/rustwake?go-get=1 where the code
// is, and reads the answer whatever its status. For a sub-path, or a name that is not a page,
// the answer is the not-found page. So every page carries one go-import tag for every module,
// and the command picks the one whose prefix fits. A module's own page also says where its
// source can be read.
func TestEveryPageCarriesTheTagsTheGoCommandReads(t *testing.T) {
	modules := games.Modules(list(t))
	var want []string
	for _, m := range modules {
		want = append(want, fmt.Sprintf("usr.games/%s git https://github.com/usrgames/%s", m, m))
	}
	for _, name := range pages {
		page := read(t, "site", name)
		if got := metas(page, "go-import"); !slices.Equal(got, want) {
			t.Errorf("%s: wanted the go-import tags %q, got %q", name, want, got)
		}

		var source []string
		if m := strings.TrimSuffix(name, ".html"); slices.Contains(modules, m) {
			repo := "https://github.com/usrgames/" + m
			source = []string{fmt.Sprintf("usr.games/%s %s %s/tree/main{/dir} %s/blob/main{/dir}/{file}#L{line}", m, repo, repo, repo)}
		}
		if got := metas(page, "go-source"); !slices.Equal(got, source) {
			t.Errorf("%s: wanted the go-source tags %q, got %q", name, source, got)
		}
	}
}

// The go command reads the head with a restricted parser and the reference asks for the tags
// early, before any CSS or script.
func TestTheTagsComeBeforeAnythingTheParserMightStopAt(t *testing.T) {
	for _, name := range pages {
		page := read(t, "site", name)
		limit := len(page)
		for _, marker := range []string{"<link", "<style", "<script", "<body"} {
			if i := strings.Index(page, marker); i >= 0 {
				limit = min(limit, i)
			}
		}
		if i := strings.LastIndex(page, `name="go-import"`); i < 0 || i > limit {
			t.Errorf("%s: wanted every go-import tag before the first link, style, script or body (at %d), found one at %d", name, limit, i)
		}
	}
}

// A module's page is a flat file, site/rustwake.html. As a folder with an index.html, Cloudflare
// would answer usr.games/rustwake?go-get=1 with a redirect to add the slash.
func TestAModulesPageIsAFlatFile(t *testing.T) {
	for _, m := range games.Modules(list(t)) {
		if _, err := os.Stat(filepath.Join(site, m, "index.html")); err == nil {
			t.Errorf("wanted site/%s.html and no site/%s/index.html", m, m)
		}
	}
}

// wrangler.jsonc tells Cloudflare which folder to serve and what to send for a path with no
// file. Without the 404 page a request for usr.games/arcade/relay?go-get=1 would get an empty
// 404 with no tags. Cloudflare builds every pull request branch with `wrangler preview`, which
// stops with an error unless the file has a previews block, even an empty one. The file must
// stay plain JSON, with no comments, for this test to read it.
func TestCloudflareServesTheSiteFolderWithItsNotFoundPage(t *testing.T) {
	var cfg struct {
		Name   string `json:"name"`
		Assets struct {
			Directory string `json:"directory"`
			NotFound  string `json:"not_found_handling"`
		} `json:"assets"`
		Previews map[string]any `json:"previews"`
	}
	if err := json.Unmarshal([]byte(read(t, "wrangler.jsonc")), &cfg); err != nil {
		t.Fatalf("wanted wrangler.jsonc to be plain JSON, got %v", err)
	}
	if cfg.Name == "" {
		t.Errorf("wanted the Worker to have a name")
	}
	if got := filepath.Clean(cfg.Assets.Directory); got != "site" {
		t.Errorf("wanted the served directory to be site, got %q", cfg.Assets.Directory)
	}
	if cfg.Previews == nil {
		t.Errorf(`wanted a "previews" block (it may be empty), without which Cloudflare's builds of pull request branches fail`)
	}
	if cfg.Assets.NotFound != "404-page" {
		t.Errorf(`wanted not_found_handling "404-page", so unknown paths get 404.html, got %q`, cfg.Assets.NotFound)
	}
}

func TestPagesAreSelfContainedAndPlain(t *testing.T) {
	email := regexp.MustCompile(`[\w.+-]+@[\w-]+\.[\w.-]+`)
	for _, name := range pages {
		page := read(t, "site", name)
		for _, want := range []string{`<html lang="`, `<meta name="viewport"`, `<meta charset="utf-8">`, `<main id="main"`, `<h1`} {
			if !strings.Contains(page, want) {
				t.Errorf("%s: wanted %s", name, want)
			}
		}
		if m := regexp.MustCompile(`<title>([^<]+)</title>`).FindStringSubmatch(page); m == nil || strings.TrimSpace(m[1]) == "" {
			t.Errorf("%s: wanted a title", name)
		}
		if got := metas(page, "description"); len(got) != 1 || got[0] == "" {
			t.Errorf("%s: wanted one description, got %q", name, got)
		}
		// html/template writes this in place of a value it will not print.
		if strings.Contains(page, "ZgotmplZ") {
			t.Errorf("%s: a value was refused by the template and is missing from the page", name)
		}
		// Cloudflare rewrites email addresses it finds in a page.
		if m := email.FindString(page); m != "" {
			t.Errorf("%s: wanted no email address, found %q", name, m)
		}
	}
}

// The site has one script, on the front page, and nothing is written inside a page, so that
// what runs is one file anyone can read. It asks no one for anything.
func TestTheOneScriptIsTheListing(t *testing.T) {
	const tag = `<script src="/floor.js" defer></script>`
	for _, name := range pages {
		page := read(t, "site", name)
		want := map[bool]int{true: 1, false: 0}[name == "index.html"]
		if got := strings.Count(strings.ToLower(page), "<script"); got != want {
			t.Errorf("%s: wanted %d script, found %d", name, want, got)
		}
		if name == "index.html" && strings.Count(page, tag) != 1 {
			t.Errorf("index.html: wanted its one script to be %s", tag)
		}
		if regexp.MustCompile(`(?i)\son[a-z]+="`).MatchString(page) {
			t.Errorf("%s: wanted no script written into a tag", name)
		}
	}
	script := read(t, "site", "floor.js")
	for _, asks := range []string{"fetch(", "XMLHttpRequest", "import(", "://", "sendBeacon", "WebSocket"} {
		if strings.Contains(script, asks) {
			t.Errorf("floor.js: wanted it to ask no one for anything, found %q", asks)
		}
	}
}

// Every page names the three icons, which internal/brand draws: one for browsers that take an
// SVG, one for those that do not, and one for a phone's home screen.
func TestEveryPageLinksTheIcons(t *testing.T) {
	for _, name := range pages {
		page := read(t, "site", name)
		for _, link := range []string{
			`<link rel="icon" href="/favicon.ico" sizes="32x32">`,
			`<link rel="icon" href="/favicon.svg" type="image/svg+xml">`,
			`<link rel="apple-touch-icon" href="/apple-touch-icon.png">`,
		} {
			if !strings.Contains(page, link) {
				t.Errorf("%s: wanted %s", name, link)
			}
		}
	}
}

// What a chat or a social site shows beside a link to usr.games comes from these tags. The
// picture is named by its full address, which the test for local links does not follow, so it
// is checked here that the address is a file of the site.
func TestTheLinkPreviewNamesAFileOfTheSite(t *testing.T) {
	index := read(t, "site", "index.html")
	for _, tag := range []string{`<meta property="og:title" content="`, `<meta property="og:description" content="`, `<meta property="og:image:alt" content="`, `<meta name="twitter:card" content="summary_large_image">`} {
		if !strings.Contains(index, tag) {
			t.Errorf("index.html: wanted %s", tag)
		}
	}
	m := regexp.MustCompile(`<meta property="og:image" content="https://usr\.games/([^"]+)">`).FindStringSubmatch(index)
	if m == nil {
		t.Fatal(`index.html: wanted <meta property="og:image"> with an address on usr.games`)
	}
	if _, err := os.Stat(filepath.Join(site, m[1])); err != nil {
		t.Errorf("index.html: wanted the picture of the link preview, %q, to be a file of the site, got %v", m[1], err)
	}
}

// The pages ask no other site for anything: the typeface, the stylesheet, the script and the
// pictures are files of the site. What a page loads is what its link tags, its pictures and its
// stylesheet name; a link to follow is not loaded.
func TestNothingIsLoadedFromAnotherSite(t *testing.T) {
	local := func(target string) bool {
		return strings.HasPrefix(target, "/") && !strings.HasPrefix(target, "//")
	}
	loads := regexp.MustCompile(`<link\b[^>]*\bhref="([^"]*)"|\bsrc(?:set)?="([^"]*)"`)
	for _, name := range pages {
		for _, m := range loads.FindAllStringSubmatch(read(t, "site", name), -1) {
			if target := m[1] + m[2]; !local(target) {
				t.Errorf("%s: wanted %q to be loaded from this site", name, target)
			}
		}
	}

	css := read(t, "site", "style.css")
	if strings.Contains(css, "@import") {
		t.Error("style.css: wanted no @import")
	}
	urls := regexp.MustCompile(`url\(\s*["']?([^"')]+)["']?\s*\)`).FindAllStringSubmatch(css, -1)
	if len(urls) == 0 {
		t.Error("style.css: wanted it to name the typeface of BRAND.md")
	}
	for _, m := range urls {
		if !local(m[1]) {
			t.Errorf("style.css: wanted %q to be loaded from this site", m[1])
			continue
		}
		if _, err := os.Stat(filepath.Join(site, m[1])); err != nil {
			t.Errorf("style.css: wanted %q to be a file of the site, got %v", m[1], err)
		}
		// A typeface under the Open Font License travels with its licence.
		if strings.HasPrefix(m[1], "/media/fonts/") {
			if _, err := os.Stat(filepath.Join(site, "media", "fonts", "OFL.txt")); err != nil {
				t.Errorf("wanted the typeface's licence beside it, got %v", err)
			}
		}
	}
}

// file returns the file of the site that Cloudflare serves for a local address: the front page
// for /, and for a name with no extension the page of that name.
func file(target string) string {
	target = strings.TrimPrefix(target, "/")
	switch {
	case target == "":
		return "index.html"
	case strings.HasSuffix(target, "/"):
		return target + "index.html"
	case filepath.Ext(target) == "":
		return target + ".html"
	}
	return target
}

// A link to this site must name a file that is in it, and a link to a part of a page an id that
// the page has.
func TestLocalLinksNameFilesOfTheSite(t *testing.T) {
	link := regexp.MustCompile(`(?:href|src|srcset)="([^"]*)"`)
	for _, name := range pages {
		page := read(t, "site", name)
		found := 0
		for _, m := range link.FindAllStringSubmatch(page, -1) {
			if regexp.MustCompile(`^[a-z][a-z0-9+.-]*:`).MatchString(m[1]) {
				continue // another site
			}
			target, part, _ := strings.Cut(m[1], "#")
			target, _, _ = strings.Cut(target, "?")
			in := page
			if target != "" {
				found++
				data, err := os.ReadFile(filepath.Join(site, filepath.FromSlash(file(target))))
				if err != nil {
					t.Errorf("%s: wanted %q to be a file of the site, got %v", name, m[1], err)
					continue
				}
				in = string(data)
			}
			if part != "" && !strings.Contains(in, `id="`+part+`"`) {
				t.Errorf("%s: wanted %q to name an id of the page it leads to", name, m[1])
			}
		}
		if found == 0 {
			t.Errorf("%s: wanted at least the stylesheet among local links", name)
		}

		// Of two parts with one id, a link reaches the first only.
		seen := map[string]bool{}
		for _, m := range regexp.MustCompile(` id="([^"]*)"`).FindAllStringSubmatch(page, -1) {
			if seen[m[1]] {
				t.Errorf("%s: wanted the id %q once", name, m[1])
			}
			seen[m[1]] = true
		}
	}
}

// The front page lists every game once, in a row and on a screen, and the line under the
// listing counts them, so the line under the sign never has to.
func TestTheListingHoldsEveryGameAndCountsThem(t *testing.T) {
	index, gs := read(t, "site", "index.html"), list(t)
	if got := strings.Count(index, `<a class="row"`); got != len(gs) {
		t.Errorf("wanted %d rows in the listing, got %d", len(gs), got)
	}
	if got := strings.Count(index, `<div class="att"`); got != len(gs) {
		t.Errorf("wanted %d screens beside the listing, got %d", len(gs), got)
	}
	if got := strings.Count(index, `<div class="att" hidden>`); got != len(gs)-1 {
		t.Errorf("wanted every screen but the first hidden until its row is chosen, got %d hidden", got)
	}
	want := fmt.Sprintf("%d games.", len(gs))
	if len(gs) == 1 {
		want = "1 game."
	}
	if !strings.Contains(index, want+" Up and down to choose") {
		t.Errorf("wanted the line under the listing to say %q", want)
	}
	last := -1
	for _, g := range games.ByName(gs) {
		i := strings.Index(index, `<a class="row" href="/`+g.Name+`">`)
		if i < 0 {
			t.Errorf("wanted a row for %s", g.Name)
		}
		if i < last {
			t.Errorf("wanted the rows from A to Z, found %s out of place", g.Name)
		}
		last = i
	}
}

// The front page names a few good terminals in one line, and the terminals page has the whole
// list. A name on the first must be on the second.
func TestTheFrontPagesTerminalsAreOnTheTerminalsPage(t *testing.T) {
	m := regexp.MustCompile(`Best in ([^.]+)\.`).FindStringSubmatch(read(t, "site", "index.html"))
	if m == nil {
		t.Fatal(`index.html: wanted the line that starts "Best in"`)
	}
	full := read(t, "site", "terminals.html")
	for _, name := range regexp.MustCompile(`,| or `).Split(m[1], -1) {
		if name = strings.TrimSpace(name); name != "" && !strings.Contains(full, "<li>"+name+"</li>") {
			t.Errorf("wanted %q, named on the front page, in the list on the terminals page", name)
		}
	}
}

// The site names no person. The company that runs it is named once, in the foot of each page.
func TestTheSiteNamesItsOperatorAndNoPerson(t *testing.T) {
	person := regexp.MustCompile(`(?i)kyro|\bkohan\b`)
	err := filepath.WalkDir(site, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !slices.Contains([]string{".html", ".css", ".js", ".json", ".svg"}, filepath.Ext(path)) {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if m := person.Find(data); m != nil {
			t.Errorf("%s: wanted no person named, found %q", path, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range pages {
		if got := strings.Count(read(t, "site", name), "Operated by KOHANTIC LLC."); got != 1 {
			t.Errorf("%s: wanted the operator named once in the foot, got %d", name, got)
		}
	}
}

// The stylesheet's four colours are the brand's, by the names the pages use for them.
func TestTheStylesheetWearsTheBrandsColours(t *testing.T) {
	css := read(t, "site", "style.css")
	for name, hex := range map[string]string{"--glass": brand.Glass, "--p": brand.Phosphor, "--hot": brand.Bright, "--dim": brand.Dim} {
		if want := name + ": " + hex + ";"; !strings.Contains(css, want) {
			t.Errorf("style.css: wanted %s", want)
		}
	}
	if !strings.Contains(css, "prefers-reduced-motion") {
		t.Error("style.css: wanted the blink to stop for someone who asks for less motion")
	}
	// Blinking that lasts more than five seconds needs a way to stop it, so it stops by itself.
	if strings.Contains(css, "infinite") {
		t.Error("style.css: wanted nothing to move without end")
	}
}

// Online clients read this file for notices. It must stay plain JSON they can parse.
func TestTheNoticeFileIsPlainJSON(t *testing.T) {
	var notice struct {
		Schema  int     `json:"schema"`
		Message *string `json:"message"`
	}
	if err := json.Unmarshal([]byte(read(t, "site", "etc", "arcade.json")), &notice); err != nil {
		t.Fatalf("site/etc/arcade.json: %v", err)
	}
	if notice.Schema != 1 || notice.Message == nil {
		t.Errorf("site/etc/arcade.json: wanted schema 1 and a message, even an empty one, got %+v", notice)
	}
}
