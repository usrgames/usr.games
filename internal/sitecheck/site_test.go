// Package sitecheck holds the tests of the finished site in ../../site: what is served, as it
// is served. It has no code of its own. The tests of how the pages and the drawings are made
// are beside what makes them, in internal/sitepages and internal/brand.
package sitecheck

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/usrgames/usr.games/internal/brand"
	"github.com/usrgames/usr.games/internal/games"
	"github.com/usrgames/usr.games/internal/plan"
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
	want := []string{"index.html", "404.html", "terminals.html", "plan.html", "privacy.html", "terms.html"}
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
		Previews    map[string]any `json:"previews"`
		WorkersDev  *bool          `json:"workers_dev"`
		PreviewURLs *bool          `json:"preview_urls"`
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
	// With no domain attached, Cloudflare would serve the Worker at an address of its own, under
	// the account's name: the site would be public before it opens. Previews are a separate switch.
	if cfg.WorkersDev == nil || *cfg.WorkersDev {
		t.Errorf(`wanted "workers_dev": false, so the site has no address until its domain is attached`)
	}
	if cfg.PreviewURLs == nil || !*cfg.PreviewURLs {
		t.Errorf(`wanted "preview_urls": true, so each pull request still gets a preview`)
	}
	if cfg.Assets.NotFound != "404-page" {
		t.Errorf(`wanted not_found_handling "404-page", so unknown paths get 404.html, got %q`, cfg.Assets.NotFound)
	}
}

// contact is the one address the site gives, and fenced is how a page must write it.
const (
	contact = "hello@usr.games"
	fenced  = "<!--email_off-->" + contact + "<!--/email_off-->"
)

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
		// Cloudflare rewrites an email address it finds in a page into a script that prints it.
		// The notices' contact is fenced by the two comments that tell it not to.
		if m := email.FindString(strings.ReplaceAll(page, fenced, "")); m != "" {
			t.Errorf("%s: wanted no email address but the fenced contact, found %q", name, m)
		}
	}
}

// A page runs at most one script of the site's, and nothing is written inside a page, so that
// what runs is a file anyone can read: floor.js on the front page, which asks no one for
// anything, and play.js on a page that plays its game, which loads the game and glonk's player
// from this site alone, when asked.
func TestTheScriptsAreTheListingAndPlay(t *testing.T) {
	for _, name := range pages {
		page := read(t, "site", name)
		var tag string
		switch {
		case name == "index.html":
			tag = `<script src="/floor.js" defer></script>`
		case plays(page):
			tag = `<script src="/play.js" defer></script>`
		}
		want := map[bool]int{true: 1, false: 0}[tag != ""]
		if got := strings.Count(strings.ToLower(page), "<script"); got != want || (tag != "" && strings.Count(page, tag) != 1) {
			t.Errorf("%s: wanted %d script, %s, found %d", name, want, tag, got)
		}
		if regexp.MustCompile(`(?i)\son[a-z]+="`).MatchString(page) {
			t.Errorf("%s: wanted no script written into a tag", name)
		}
	}
	_, version := player(t)
	for _, file := range []string{"floor.js", "play.js", filepath.Join("play", "glonk", version, "glonk.js")} {
		script := read(t, "site", file)
		asks := []string{"XMLHttpRequest", "import(", "://", "sendBeacon", "WebSocket", "eval(", "Function("}
		if file == "floor.js" || file == "play.js" {
			asks = append(asks, "fetch(") // glonk.js fetches the game it is given, from this site
		}
		for _, a := range asks {
			if strings.Contains(script, a) {
				t.Errorf("%s: wanted it to ask no other site for anything, found %q", file, a)
			}
		}
	}
}

// A page that plays its game names the game's build, its wasm_exec.js and glonk's player, and
// each has to be a file of the site, at the address that carries its version.
func TestAPageThatPlaysNamesFilesOfTheSite(t *testing.T) {
	found := 0
	data := regexp.MustCompile(` data-(game|exec|player)="([^"]*)"`)
	for _, name := range pages {
		page := read(t, "site", name)
		if !plays(page) {
			continue
		}
		found++
		got := map[string]string{}
		for _, m := range data.FindAllStringSubmatch(page, -1) {
			got[m[1]] = m[2]
		}
		if !regexp.MustCompile(` data-cols="[0-9]+" data-rows="[0-9]+"`).MatchString(page) {
			t.Errorf("%s: wanted the size the game plays at, data-cols and data-rows", name)
		}
		_, version := player(t)
		if got["player"] != "/play/glonk/"+version+"/" {
			t.Errorf("%s: wanted the player /play/glonk/%s/, got %q", name, version, got["player"])
		}
		for _, path := range []string{got["game"], got["exec"], got["player"] + "glonk.js"} {
			if !strings.HasPrefix(path, "/play/") {
				t.Errorf("%s: wanted %q under /play/", name, path)
				continue
			}
			if _, err := os.Stat(filepath.Join(site, filepath.FromSlash(path))); err != nil {
				t.Errorf("%s: wanted %q to be a file of the site, got %v", name, path, err)
			}
		}
	}
	if found == 0 {
		t.Error("wanted at least one page that plays its game")
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

// headers returns the header lines of one block of site/_headers, by the address pattern that
// opens the block.
func headers(t *testing.T, pattern string) map[string]string {
	t.Helper()
	out := rule(t, pattern)
	if len(out) == 0 {
		t.Fatalf("_headers: wanted headers for %s", pattern)
	}
	return out
}

// rule returns the headers _headers gives a pattern, none if it has no rule for it. A header the
// rule takes off, written "! Name", is the key "! Name".
func rule(t *testing.T, pattern string) map[string]string {
	t.Helper()
	out, in := map[string]string{}, false
	for _, line := range strings.Split(read(t, "site", "_headers"), "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "#"):
		case !strings.HasPrefix(line, " "):
			in = line == pattern
		case in && strings.HasPrefix(strings.TrimSpace(line), "! "):
			out[strings.TrimSpace(line)] = ""
		case in:
			name, value, ok := strings.Cut(strings.TrimSpace(line), ": ")
			if !ok {
				t.Fatalf("_headers: wanted \"Name: value\", got %q", line)
			}
			out[name] = value
		}
	}
	return out
}

// plays says whether a page plays its game: play.js starts it from the screen's data.
func plays(page string) bool { return strings.Contains(page, ` data-game="`) }

// The browser is told what a page may load, and refuses the rest. So the policy has to allow
// every kind of thing the pages do load, the pages may hold nothing a policy without
// 'unsafe-inline' refuses, and the policy may not be loosened to make a page work.
func TestThePolicyAllowsWhatThePagesLoad(t *testing.T) {
	all := headers(t, "/*")
	policy := map[string][]string{}
	for _, d := range strings.Split(all["Content-Security-Policy"], ";") {
		if f := strings.Fields(d); len(f) > 0 {
			policy[f[0]] = f[1:]
		}
	}
	if got := policy["default-src"]; !slices.Equal(got, []string{"'none'"}) {
		t.Errorf("_headers: wanted default-src 'none', so that what is not named is refused, got %q", got)
	}
	for name, sources := range policy {
		for _, s := range sources {
			if strings.HasPrefix(s, "'unsafe-") || s == "*" || s == "https:" || s == "data:" {
				t.Errorf("_headers: wanted %s to name its sources, got %s", name, s)
			}
		}
	}
	// The visit counter: Cloudflare's script, added at the edge, which reports to this site.
	if !slices.Contains(policy["script-src"], "https://static.cloudflareinsights.com/beacon.min.js/") || !slices.Contains(policy["connect-src"], "'self'") {
		t.Errorf("_headers: wanted Cloudflare's analytics script allowed, and its report to this site")
	}
	if all["X-Content-Type-Options"] != "nosniff" || all["Referrer-Policy"] == "" {
		t.Errorf("_headers: wanted X-Content-Type-Options: nosniff and a Referrer-Policy on every address, got %v", all)
	}
	// Only a preview is kept out of search engines; a rule for every address would hide the site.
	if _, hidden := all["X-Robots-Tag"]; hidden || headers(t, "https://:preview.:account.workers.dev/*")["X-Robots-Tag"] != "noindex" {
		t.Errorf("_headers: wanted X-Robots-Tag: noindex on workers.dev addresses and on no other")
	}

	// A page that plays takes the policy off and sets it again with one word more, so that the
	// browser compiles the game's WebAssembly. No other page has a rule of its own.
	game := strings.Replace(all["Content-Security-Policy"], "script-src 'self' ", "script-src 'self' 'wasm-unsafe-eval' ", 1)
	for _, name := range pages {
		path := "/" + strings.TrimSuffix(name, ".html")
		got := rule(t, path)
		switch {
		case plays(read(t, "site", name)):
			if _, off := got["! Content-Security-Policy"]; !off || got["Content-Security-Policy"] != game || len(got) != 2 {
				t.Errorf("_headers: %s plays its game, so wanted it to take the policy off and set it again with 'wasm-unsafe-eval' and nothing else, got %v", path, got)
			}
		case len(got) != 0 && path != "/install.sh":
			t.Errorf("_headers: %s does not play a game, so wanted no rule of its own, got %v", path, got)
		}
	}
	if got := headers(t, "/play/*")["Cache-Control"]; got != "public, max-age=31536000, immutable" {
		t.Errorf("_headers: wanted /play/*, whose addresses carry a version, kept for good, got %q", got)
	}

	loads := map[string]*regexp.Regexp{
		"script-src": regexp.MustCompile(`<script\b[^>]*\bsrc="`),
		"style-src":  regexp.MustCompile(`<link\b[^>]*\brel="stylesheet"`),
		"img-src":    regexp.MustCompile(`<img\b|<link\b[^>]*\brel="(?:icon|apple-touch-icon)"`),
		"font-src":   regexp.MustCompile(`<link\b[^>]*\bas="font"`),
	}
	refused := map[string]*regexp.Regexp{
		"a script written in the page": regexp.MustCompile(`<script(?:\s[^>]*)?>\s*[^<\s]`),
		"a style block":                regexp.MustCompile(`<style\b`),
		"a style attribute":            regexp.MustCompile(`<[^>]*\sstyle="`),
		"an event handler":             regexp.MustCompile(`<[^>]*\son[a-z]+="`),
		"a frame":                      regexp.MustCompile(`<i?frame\b|<object\b|<embed\b`),
		"a form":                       regexp.MustCompile(`<form\b`),
	}
	for _, name := range pages {
		page := read(t, "site", name)
		for directive, re := range loads {
			if re.MatchString(page) && !slices.Contains(policy[directive], "'self'") {
				t.Errorf("%s: the page loads what %s 'self' allows, and the policy has %q", name, directive, policy[directive])
			}
		}
		for what, re := range refused {
			if m := re.FindString(page); m != "" {
				t.Errorf("%s: wanted nothing the policy refuses, found %s: %s", name, what, m)
			}
		}
	}
}

// The notices say who runs the site and where to write, and security.txt says where to report
// a hole. A file whose date has passed tells a researcher nobody is listening, so this fails a
// month before it does: move Expires on, by less than a year, and it passes again.
func TestTheNoticesAndSecurityTxtGiveTheContact(t *testing.T) {
	for _, name := range []string{"privacy.html", "terms.html"} {
		page := read(t, "site", name)
		if strings.Count(page, fenced) != 1 || !strings.Contains(page, "operated by KOHANTIC LLC") {
			t.Errorf("%s: wanted the operator named and the contact once, as %s", name, fenced)
		}
	}
	fields := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(read(t, "site", ".well-known", "security.txt")), "\n") {
		key, value, ok := strings.Cut(line, ": ")
		if !ok || fields[key] != "" {
			t.Fatalf("security.txt: wanted one \"Field: value\" a line, each field once, got %q", line)
		}
		fields[key] = value
	}
	if got := fields["Contact"]; got != "mailto:"+contact {
		t.Errorf("security.txt: wanted Contact: mailto:%s, got %q", contact, got)
	}
	if got := fields["Canonical"]; got != "https://usr.games/.well-known/security.txt" {
		t.Errorf("security.txt: wanted Canonical to be its own address, got %q", got)
	}
	expires, err := time.Parse(time.RFC3339, fields["Expires"])
	if err != nil {
		t.Fatalf("security.txt: wanted Expires as a time like 2027-10-21T00:00:00Z, got %q", fields["Expires"])
	}
	switch left := time.Until(expires); {
	case left < 30*24*time.Hour:
		t.Errorf("security.txt: it expires on %s: move Expires on, by less than a year", expires.Format(time.DateOnly))
	case left > 366*24*time.Hour:
		t.Errorf("security.txt: wanted an expiry less than a year away, got %s", expires.Format(time.DateOnly))
	}
}

// The pages ask no other site for anything: the typeface, the stylesheet, the script and the
// pictures are files of the site. What a page loads is what its link tags, its pictures and its
// stylesheet name; a link to follow is not loaded. One thing is added that no file here names:
// on usr.games Cloudflare puts its Web Analytics script into each page at the edge, to count
// visits. That was decided on 2026-11-21, as on glonk.dev, and the privacy notice says so.
// Previews and this test do not see it.
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

// Nothing shows a mistake in a feed: a reader that cannot parse it just goes quiet. So the feed
// is held to the notes in plan/: one entry each, newest first, each leading to its note on the
// page.
func TestTheFeedHoldsEveryNoteAndIsWellFormed(t *testing.T) {
	var feed struct {
		XMLName xml.Name `xml:"http://www.w3.org/2005/Atom feed"`
		Title   string   `xml:"title"`
		ID      string   `xml:"id"`
		Updated string   `xml:"updated"`
		Author  string   `xml:"author>name"`
		Links   []struct {
			Rel  string `xml:"rel,attr"`
			Href string `xml:"href,attr"`
		} `xml:"link"`
		Entries []struct {
			Title   string `xml:"title"`
			ID      string `xml:"id"`
			Updated string `xml:"updated"`
			Link    struct {
				Href string `xml:"href,attr"`
			} `xml:"link"`
			Content string `xml:"content"`
		} `xml:"entry"`
	}
	if err := xml.Unmarshal([]byte(read(t, "site", "plan.xml")), &feed); err != nil {
		t.Fatalf("plan.xml: wanted an Atom feed, got %v", err)
	}
	notes, err := plan.Load(filepath.Join("..", "..", "plan"))
	if err != nil {
		t.Fatal(err)
	}
	// Atom asks for all four of a feed.
	if feed.Title == "" || feed.ID == "" || feed.Author == "" || len(feed.Entries) == 0 {
		t.Fatalf("plan.xml: wanted a title, an id, an author and entries, got %q, %q, %q and %d entries", feed.Title, feed.ID, feed.Author, len(feed.Entries))
	}
	self := false
	for _, l := range feed.Links {
		self = self || (l.Rel == "self" && l.Href == "https://usr.games/plan.xml")
	}
	if !self {
		t.Errorf("plan.xml: wanted a link to itself at https://usr.games/plan.xml, got %+v", feed.Links)
	}
	if len(feed.Entries) != len(notes) {
		t.Fatalf("plan.xml: wanted one entry for each of the %d notes, got %d", len(notes), len(feed.Entries))
	}
	page := read(t, "site", "plan.html")
	for i, e := range feed.Entries {
		n := notes[i]
		at := "https://usr.games/plan#" + n.ID
		if e.Title != n.Title || e.ID != at || e.Link.Href != at {
			t.Errorf("plan.xml: entry %d: wanted %q at %s, got %q with id %s and link %s", i+1, n.Title, at, e.Title, e.ID, e.Link.Href)
		}
		if !strings.Contains(page, `id="`+n.ID+`"`) {
			t.Errorf("plan.html: wanted the note %s under its own id, where the feed sends a reader", n.ID)
		}
		if when, err := time.Parse(time.RFC3339, e.Updated); err != nil || !when.Equal(n.Date) {
			t.Errorf("plan.xml: entry %d: wanted the time %s, got %q", i+1, n.Date.Format(time.RFC3339), e.Updated)
		}
		if strings.TrimSpace(e.Content) == "" {
			t.Errorf("plan.xml: entry %d: wanted the note's text", i+1)
		}
	}
	if feed.Updated != feed.Entries[0].Updated {
		t.Errorf("plan.xml: wanted the feed as old as its newest note, %s, got %s", feed.Entries[0].Updated, feed.Updated)
	}
}

// A feed reader, given any address of the site, finds the feed from the page's head.
func TestEveryPageNamesTheFeed(t *testing.T) {
	const want = `<link rel="alternate" type="application/atom+xml" title="usr.games: the plan" href="/plan.xml">`
	for _, name := range pages {
		if head, _, _ := strings.Cut(read(t, "site", name), "</head>"); !strings.Contains(head, want) {
			t.Errorf("%s: wanted the feed named in the head: %s", name, want)
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
		i := strings.Index(index, `<a class="row" href="/`+g.Name)
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
		if err != nil || d.IsDir() || !slices.Contains([]string{".html", ".css", ".js", ".json", ".svg", ".xml", ".txt", ".sh"}, filepath.Ext(path)) {
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

// fields reads what SSH writes one after another in a key or a certificate: a text or a blob
// as four bytes of length and then that many bytes, and a number as eight bytes or four.
type fields struct {
	t    *testing.T
	what string
	rest []byte
}

func (f *fields) take(n int) []byte {
	f.t.Helper()
	if n < 0 || n > len(f.rest) {
		f.t.Fatalf("%s: cut short", f.what)
	}
	b := f.rest[:n]
	f.rest = f.rest[n:]
	return b
}
func (f *fields) blob() []byte  { return f.take(int(binary.BigEndian.Uint32(f.take(4)))) }
func (f *fields) num64() uint64 { return binary.BigEndian.Uint64(f.take(8)) }
func (f *fields) num32() uint32 { return binary.BigEndian.Uint32(f.take(4)) }

// sshKey returns the 32 bytes of an Ed25519 public key from the form SSH writes it in.
func sshKey(t *testing.T, what string, b []byte) []byte {
	t.Helper()
	f := &fields{t: t, what: what, rest: b}
	if kind := string(f.blob()); kind != "ssh-ed25519" {
		t.Fatalf("%s: wanted an ssh-ed25519 key, got %q", what, kind)
	}
	key := f.blob()
	if len(key) != ed25519.PublicKeySize || len(f.rest) != 0 {
		t.Fatalf("%s: wanted a key of %d bytes and nothing after it", what, ed25519.PublicKeySize)
	}
	return key
}

// A release is checked against two files of the site: the list that names the root key, and
// the certificate in which the root vouches, for a year, for the key that signs releases. They
// have to belong together, or nothing the studio releases can be installed. And the certificate
// is inside every signature, so a release stops checking out the day it ends: this fails a
// month before that. KEYS.md says how it is renewed.
func TestTheRootVouchesForTheReleaseKey(t *testing.T) {
	const who = "release@usr.games"
	var roots [][]byte
	for _, line := range strings.Split(read(t, "site", "etc", "allowed_signers"), "\n") {
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// One identity, one purpose: the root may vouch for the release key and for nothing else.
		f := strings.Fields(line)
		if len(f) != 4 || f[0] != who || f[1] != `cert-authority,namespaces="`+who+`"` || f[2] != "ssh-ed25519" {
			t.Fatalf("allowed_signers: wanted `%s cert-authority,namespaces=\"%s\" ssh-ed25519 KEY`, got %q", who, who, line)
		}
		raw, err := base64.StdEncoding.DecodeString(f[3])
		if err != nil {
			t.Fatalf("allowed_signers: the key is not base64: %v", err)
		}
		roots = append(roots, sshKey(t, "allowed_signers", raw))
	}
	if len(roots) != 1 {
		t.Fatalf("allowed_signers: wanted one root key, got %d", len(roots))
	}
	// Someone who opens either file in a browser should be shown it, not offered a download.
	for _, name := range []string{"/etc/allowed_signers", "/etc/release-cert.pub"} {
		if got := headers(t, name)["Content-Type"]; !strings.HasPrefix(got, "text/plain") {
			t.Errorf("_headers: wanted %s served as text/plain, got %q", name, got)
		}
	}

	text := strings.Fields(read(t, "site", "etc", "release-cert.pub"))
	if len(text) < 2 || text[0] != "ssh-ed25519-cert-v01@openssh.com" {
		t.Fatalf("release-cert.pub: wanted an ssh-ed25519 certificate")
	}
	whole, err := base64.StdEncoding.DecodeString(text[1])
	if err != nil {
		t.Fatalf("release-cert.pub: not base64: %v", err)
	}
	c := &fields{t: t, what: "release-cert.pub", rest: whole}
	c.blob()  // the kind, again
	c.blob()  // a nonce
	c.blob()  // the release key itself
	c.num64() // the serial
	kind := c.num32()
	c.blob() // the key's id
	names := c.blob()
	after, before := time.Unix(int64(c.num64()), 0), time.Unix(int64(c.num64()), 0)
	options, extensions := c.blob(), c.blob()
	c.blob() // reserved
	signer := c.blob()
	signed := whole[:len(whole)-len(c.rest)] // the root signed everything up to here
	s := &fields{t: t, what: "release-cert.pub: the signature", rest: c.blob()}
	if alg := string(s.blob()); alg != "ssh-ed25519" {
		t.Fatalf("release-cert.pub: wanted an ssh-ed25519 signature, got %q", alg)
	}
	signature := s.blob()
	if len(c.rest) != 0 {
		t.Fatalf("release-cert.pub: %d bytes after the signature", len(c.rest))
	}

	if !bytes.Equal(sshKey(t, "release-cert.pub: the signer", signer), roots[0]) || !ed25519.Verify(roots[0], signed, signature) {
		t.Errorf("release-cert.pub: wanted it signed by the root key in allowed_signers")
	}
	n := &fields{t: t, what: "release-cert.pub: the names", rest: names}
	if kind != 1 || string(n.blob()) != who || len(n.rest) != 0 {
		t.Errorf("release-cert.pub: wanted a certificate for %s alone", who)
	}
	if len(options) != 0 || len(extensions) != 0 {
		t.Errorf("release-cert.pub: wanted it to permit nothing but signing; issue it with -O clear")
	}
	switch left := time.Until(before); {
	case time.Now().Before(after):
		t.Errorf("release-cert.pub: it is not good until %s", after.UTC().Format(time.DateOnly))
	case left < 30*24*time.Hour:
		t.Errorf("release-cert.pub: it ends on %s, and every release stops checking out that day: renew it as KEYS.md says", before.UTC().Format(time.DateOnly))
	case before.Sub(after) > 366*24*time.Hour:
		t.Errorf("release-cert.pub: wanted it good for a year at most, got %s to %s", after.UTC().Format(time.DateOnly), before.UTC().Format(time.DateOnly))
	}
}

// tty1 reads this file to know which games there are. It holds each game of games.json once,
// from A to Z, in a shape the launcher refuses if it does not know it.
func TestTheLauncherIsServedEveryGame(t *testing.T) {
	var doc struct {
		Schema int `json:"schema"`
		Games  []struct {
			Name    string `json:"name"`
			Players string `json:"players"`
			Blurb   string `json:"blurb"`
		} `json:"games"`
	}
	if err := json.Unmarshal([]byte(read(t, "site", "etc", "games.json")), &doc); err != nil {
		t.Fatalf("site/etc/games.json: %v", err)
	}
	want := games.ByName(list(t))
	if doc.Schema != 1 || len(doc.Games) != len(want) {
		t.Fatalf("site/etc/games.json: wanted schema 1 and %d games, got schema %d and %d", len(want), doc.Schema, len(doc.Games))
	}
	for i, g := range want {
		if got := doc.Games[i]; got.Name != g.Name || got.Players != g.Players || got.Blurb != g.Blurb {
			t.Errorf("site/etc/games.json: wanted %s (%s) at %d, as games.json has it, got %+v", g.Name, g.Players, i, got)
		}
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
