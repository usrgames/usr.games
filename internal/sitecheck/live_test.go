package sitecheck

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/usrgames/usr.games/internal/games"
)

// The site as it is served, at the address USRGAMES_LIVE names, such as https://usr.games: each
// page answers with the policy _headers gives it, the go command finds every module, the files a
// player is told to read are text, and nothing sets a cookie. It skips itself with no address;
// the live workflow runs it by name.
func TestTheLiveSite(t *testing.T) {
	base := strings.TrimSuffix(os.Getenv("USRGAMES_LIVE"), "/")
	if base == "" {
		t.Skip("USRGAMES_LIVE names no live site")
	}
	client := &http.Client{
		Timeout:       30 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	get := func(t *testing.T, url string) (*http.Response, string) {
		t.Helper()
		res, err := client.Get(url)
		if err != nil {
			t.Fatal(err)
		}
		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			t.Fatal(err)
		}
		if cookies := res.Header.Values("Set-Cookie"); len(cookies) != 0 {
			t.Errorf("%s: wanted no cookie set, as /privacy says, got %q", url, cookies)
		}
		return res, string(body)
	}
	site := headers(t, "/*")
	game := strings.Replace(site["Content-Security-Policy"], "script-src 'self' ", "script-src 'self' 'wasm-unsafe-eval' ", 1)
	policed := func(t *testing.T, url string, res *http.Response, want string) {
		t.Helper()
		if got := res.Header.Values("Content-Security-Policy"); len(got) != 1 || got[0] != want {
			t.Errorf("%s: wanted the one policy %q, got %q", url, want, got)
		}
		for _, name := range []string{"X-Content-Type-Options", "Referrer-Policy"} {
			if res.Header.Get(name) != site[name] {
				t.Errorf("%s: wanted %s: %s, got %q", url, name, site[name], res.Header.Get(name))
			}
		}
	}

	t.Run("every page, with its policy", func(t *testing.T) {
		for _, name := range pages {
			if name == "404.html" {
				continue
			}
			path := "/" + strings.TrimSuffix(strings.TrimSuffix(name, ".html"), "index")
			url := base + strings.TrimSuffix(path, "/")
			if path == "/" {
				url = base + "/"
			}
			res, body := get(t, url)
			if res.StatusCode != http.StatusOK {
				t.Errorf("%s: wanted 200, got %d", url, res.StatusCode)
				continue
			}
			made := read(t, "site", name)
			want := site["Content-Security-Policy"]
			if plays(made) {
				want = game
			}
			policed(t, url, res, want)
			// Cloudflare adds its Web Analytics script at the end of a page; all before it is the file.
			if end := strings.Index(made, "</main>"); end < 0 || !strings.HasPrefix(body, made[:end]) {
				t.Errorf("%s: wanted the page site/%s, got another", url, name)
			}
		}
	})

	t.Run("the go command finds every module, from its page or from the not-found page", func(t *testing.T) {
		var tags []string
		modules := games.GoModules
		for _, m := range modules {
			tags = append(tags, fmt.Sprintf(`<meta name="go-import" content="usr.games/%s git https://github.com/usrgames/%s">`, m, m))
		}
		for _, m := range modules {
			for _, c := range []struct {
				path   string
				status int
			}{{"/" + m, http.StatusOK}, {"/" + m + "/a/package?go-get=1", http.StatusNotFound}} {
				url := base + c.path
				if !strings.Contains(c.path, "?") {
					url += "?go-get=1"
				}
				res, body := get(t, url)
				if res.StatusCode != c.status {
					t.Errorf("%s: wanted %d, got %d", url, c.status, res.StatusCode)
				}
				policed(t, url, res, site["Content-Security-Policy"])
				for _, tag := range tags {
					if !strings.Contains(body, tag) {
						t.Errorf("%s: wanted %s", url, tag)
					}
				}
			}
		}
	})

	t.Run("an address that is nothing gets the not-found page", func(t *testing.T) {
		url := base + "/no/such/file"
		res, body := get(t, url)
		made := read(t, "site", "404.html")
		if res.StatusCode != http.StatusNotFound || !strings.HasPrefix(body, made[:strings.Index(made, "</main>")]) {
			t.Errorf("%s: wanted 404 and the not-found page, got %d", url, res.StatusCode)
		}
		policed(t, url, res, site["Content-Security-Policy"])
	})

	t.Run("files to read are text, and play files are kept for good", func(t *testing.T) {
		_, version := player(t)
		for path, want := range map[string]string{
			"/etc/allowed_signers": "text/plain", "/etc/release-cert.pub": "text/plain", "/install.sh": "text/plain",
			"/.well-known/security.txt": "text/plain", "/etc/games.json": "application/json",
			"/play/glonk/" + version + "/glonk.js": "text/javascript",
		} {
			url := base + path
			res, body := get(t, url)
			if res.StatusCode != http.StatusOK || !strings.HasPrefix(res.Header.Get("Content-Type"), want) {
				t.Errorf("%s: wanted 200 and %s, got %d and %q", url, want, res.StatusCode, res.Header.Get("Content-Type"))
			}
			if made := read(t, filepath.Join("site", filepath.FromSlash(path))); body != made {
				t.Errorf("%s: wanted site%s as it is in the repository", url, path)
			}
		}
		url := base + "/play/glonk/" + version + "/glonk.js"
		if res, _ := get(t, url); res.Header.Get("Cache-Control") != headers(t, "/play/*")["Cache-Control"] {
			t.Errorf("%s: wanted Cache-Control %q, got %q", url, headers(t, "/play/*")["Cache-Control"], res.Header.Get("Cache-Control"))
		}
	})

	// The studio's other addresses send a visitor to the same page of usr.games. A preview has
	// none of them.
	t.Run("the other addresses lead to usr.games", func(t *testing.T) {
		if base != "https://usr.games" {
			t.Skip("only usr.games itself has the other addresses")
		}
		for _, host := range []string{"usrgames.com", "www.usrgames.com", "tty1.games", "www.tty1.games", "www.usr.games"} {
			url := "https://" + host + "/plan?from=" + host
			res, err := client.Get(url)
			if err != nil {
				t.Errorf("%s: %v", url, err)
				continue
			}
			res.Body.Close()
			switch {
			case res.StatusCode != http.StatusMovedPermanently && res.StatusCode != http.StatusPermanentRedirect:
				t.Errorf("%s: wanted a permanent redirect, got %d", url, res.StatusCode)
			case res.Header.Get("Location") != "https://usr.games/plan?from="+host:
				t.Errorf("%s: wanted it sent to https://usr.games/plan?from=%s, got %q", url, host, res.Header.Get("Location"))
			}
		}
	})
}
