package sitecheck

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// The releases the tests read, kept here because the repositories of the games and of the arcade
// are private (usrgames/usr.games#33): of tty1 v0.1.0, the relay v0.1.0 and rustwake v0.1.0,
// each release's manifest and signature, and the files of it the site uses. The files are the
// same bytes as the pinned ones, so git keeps one of each.
//
// shortcut: their signatures stop checking out when the release certificate ends, on
// 2027-11-21; sign them again at its renewal, as KEYS.md says.
func kept(name string) string { return filepath.Join("testdata", "releases", name) }

// scripts/pin.sh is how a release's manual page, a game's recording and its build for a web
// page come onto the site: each file is kept only once the studio's signature on the release's
// manifest checks out, and the file against it. It is not served, but it decides what is, so it
// is tried here as the installer is, on real releases signed with the studio's real key: the
// copies kept in testdata.
func TestThePinScript(t *testing.T) {
	tty1, relay, rustwake := kept("tty1"), kept("relay"), kept("rustwake")
	version := manifestOf(t, tty1).Version
	page, sum := manualOf(t, tty1)
	a, b := strings.Repeat("a", 64), strings.Repeat("b", 64)

	t.Run("it pins the page the studio signed", func(t *testing.T) {
		p := newPin(t, "{}\n")
		p.serve(tty1, "tty1", version)
		p.pinned("tty1", version)
		if got := p.files(); len(got) != 1 || got["man/"+page] != string(readFile(t, filepath.Join(tty1, page))) {
			t.Errorf("wanted man/%s as the release has it, and nothing else, got %v", page, slices.Collect(maps.Keys(got)))
		}
		if got, want := p.lock(), map[string]pin{"tty1": {Version: version, Files: map[string]string{page: sum}}}; !equalPins(got, want) {
			t.Errorf("wanted games.lock.json to be %v, got %v", want, got)
		}
	})

	t.Run("it pins a game's page, its recording and its build for a web page", func(t *testing.T) {
		m := manifestOf(t, rustwake)
		p := newPin(t, "{}\n")
		p.serve(rustwake, "rustwake", m.Version)
		p.pinned("rustwake", m.Version)
		want := map[string]string{}
		for _, f := range m.Files {
			if !strings.HasSuffix(f.Name, ".tar.gz") {
				want[f.Name] = f.SHA256
			}
		}
		got := p.files()
		kept := []string{
			"man/rustwake.6.html", "site/media/rustwake/rustwake.gif", "site/media/rustwake/rustwake.png",
			"site/play/rustwake/" + m.Version + "/rustwake.wasm.gz", "site/play/rustwake/" + m.Version + "/wasm_exec.js",
		}
		for _, k := range kept {
			if got[k] != string(readFile(t, filepath.Join(rustwake, filepath.Base(k)))) {
				t.Errorf("wanted %s as the release has it", k)
			}
		}
		if len(got) != len(kept) || len(want) != len(kept) {
			t.Errorf("wanted the page, the recording, its still, the build and its wasm_exec.js, and nothing else, got %v", slices.Collect(maps.Keys(got)))
		}
		if got := p.lock(); !equalPins(got, map[string]pin{"rustwake": {Version: m.Version, Files: want}}) {
			t.Errorf("wanted games.lock.json to name the five files, got %v", got)
		}
	})

	t.Run("it pins over an older build, and takes its folder away", func(t *testing.T) {
		m := manifestOf(t, rustwake)
		p := newPin(t, `{"rustwake": {"files": {"rustwake.6.html": "`+a+`", "rustwake.wasm.gz": "`+a+`", "wasm_exec.js": "`+a+`"}, "version": "v0.0.9"}}`+"\n")
		for _, path := range []string{"man/rustwake.6.html", "site/play/rustwake/v0.0.9/rustwake.wasm.gz", "site/play/rustwake/v0.0.9/wasm_exec.js"} {
			write(t, filepath.Join(p.root, filepath.FromSlash(path)), []byte("older"), 0o644)
		}
		p.serve(rustwake, "rustwake", m.Version)
		p.pinned("rustwake", m.Version)
		if _, err := os.Stat(filepath.Join(p.root, "site", "play", "rustwake", "v0.0.9")); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("wanted the older build's folder gone, got %v", err)
		}
		if got := p.files()["site/play/rustwake/"+m.Version+"/rustwake.wasm.gz"]; got != string(readFile(t, filepath.Join(rustwake, "rustwake.wasm.gz"))) {
			t.Error("wanted the build of the release pinned in its place")
		}
	})

	t.Run("it pins over what was pinned before, and leaves the rest", func(t *testing.T) {
		p := newPin(t, `{"rustwake": {"files": {"rustwake.6.html": "`+a+`"}, "version": "v0.1.0"}, "tty1": {"files": {"tty1.8.html": "`+b+`", "tty1.gif": "`+b+`", "tty1.png": "`+b+`"}, "version": "v0.0.1"}}`+"\n")
		for path, text := range map[string]string{
			"man/rustwake.6.html": "rustwake's", "man/tty1.8.html": "an older tty1's",
			"site/media/tty1/tty1.gif": "an older recording", "site/media/tty1/tty1.png": "an older still", "site/media/tty1/name.svg": "the name",
		} {
			write(t, filepath.Join(p.root, filepath.FromSlash(path)), []byte(text), 0o644)
		}
		p.serve(tty1, "tty1", version)
		p.pinned("tty1", version)
		got := p.files()
		if !maps.Equal(got, map[string]string{
			"man/rustwake.6.html": "rustwake's", "man/" + page: string(readFile(t, filepath.Join(tty1, page))), "site/media/tty1/name.svg": "the name",
		}) {
			t.Errorf("wanted tty1's older page and recording gone and the rest left, got %v", slices.Collect(maps.Keys(got)))
		}
		want := map[string]pin{
			"rustwake": {Version: "v0.1.0", Files: map[string]string{"rustwake.6.html": a}},
			"tty1":     {Version: version, Files: map[string]string{page: sum}},
		}
		if got := p.lock(); !equalPins(got, want) {
			t.Errorf("wanted games.lock.json to be %v, got %v", want, got)
		}
	})

	t.Run("a page other than the one signed for", func(t *testing.T) {
		p := newPin(t, "{}\n")
		spoil(t, filepath.Join(p.serve(tty1, "tty1", version), page))
		p.refused("is not the one the studio signed for", "tty1", version)
	})

	t.Run("a recording other than the one signed for", func(t *testing.T) {
		v := manifestOf(t, rustwake).Version
		p := newPin(t, "{}\n")
		spoil(t, filepath.Join(p.serve(rustwake, "rustwake", v), "rustwake.gif"))
		p.refused("The rustwake.gif downloaded is not the one the studio signed for", "rustwake", v)
	})

	t.Run("a build other than the one signed for", func(t *testing.T) {
		v := manifestOf(t, rustwake).Version
		p := newPin(t, "{}\n")
		spoil(t, filepath.Join(p.serve(rustwake, "rustwake", v), "rustwake.wasm.gz"))
		p.refused("The rustwake.wasm.gz downloaded is not the one the studio signed for", "rustwake", v)
	})

	t.Run("a manifest with one byte changed", func(t *testing.T) {
		p := newPin(t, "{}\n")
		spoil(t, filepath.Join(p.serve(tty1, "tty1", version), "manifest.json"))
		p.refused("is not the studio's", "tty1", version)
	})

	t.Run("a release at another version than the one asked for", func(t *testing.T) {
		p := newPin(t, "{}\n")
		p.serve(tty1, "tty1", "v9.9.9")
		p.refused("not as a release of tty1 at v9.9.9", "tty1", "v9.9.9")
	})

	t.Run("the relay's release, which the studio signed, in the place of tty1's", func(t *testing.T) {
		v := manifestOf(t, relay).Version
		p := newPin(t, "{}\n")
		p.serve(relay, "tty1", v)
		p.refused("not as a release of tty1", "tty1", v)
	})

	t.Run("a release with no manual page", func(t *testing.T) {
		v := manifestOf(t, relay).Version
		p := newPin(t, "{}\n")
		p.serve(relay, "relay", v)
		p.refused("holds no manual page", "relay", v)
	})

	t.Run("a release that is not there", func(t *testing.T) {
		p := newPin(t, "{}\n")
		p.refused("could not be downloaded", "tty1", version)
	})

	for _, args := range [][]string{{"../tty1", version}, {"tty1", "latest"}, {"tty1", "v1.2"}, {"tty1"}} {
		t.Run("asked for "+strings.Join(args, " "), func(t *testing.T) {
			newPin(t, "{}\n").refused("", args...)
		})
	}
}

// pin is one program's entry in games.lock.json.
type pin struct {
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

func equalPins(a, b map[string]pin) bool {
	return maps.EqualFunc(a, b, func(x, y pin) bool { return x.Version == y.Version && maps.Equal(x.Files, y.Files) })
}

type releaseManifest struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Files   []struct {
		Name   string `json:"name"`
		SHA256 string `json:"sha256"`
	} `json:"files"`
}

func manifestOf(t *testing.T, release string) releaseManifest {
	t.Helper()
	var m releaseManifest
	if err := json.Unmarshal(readFile(t, filepath.Join(release, "manifest.json")), &m); err != nil || m.Version == "" {
		t.Fatalf("%s: wanted a manifest with a version, got %v", release, err)
	}
	return m
}

// manualOf returns the name of the release's manual page and its hash, which the page in the
// folder has to match.
func manualOf(t *testing.T, release string) (string, string) {
	t.Helper()
	m := manifestOf(t, release)
	for _, f := range m.Files {
		if strings.HasPrefix(f.Name, m.Name+".") && strings.HasSuffix(f.Name, ".html") {
			sum := sha256.Sum256(readFile(t, filepath.Join(release, f.Name)))
			if hex.EncodeToString(sum[:]) != f.SHA256 {
				t.Fatalf("%s: wanted %s as its manifest has it", release, f.Name)
			}
			return f.Name, f.SHA256
		}
	}
	t.Fatalf("%s: wanted a manual page in the release", release)
	return "", ""
}

// spoil changes one byte of a file in the middle.
func spoil(t *testing.T, path string) {
	t.Helper()
	b := readFile(t, path)
	b[len(b)/2] ^= 1
	write(t, path, b, 0o644)
}

// pinTrial is the part of the repository the script reads and writes, copied into a folder of
// its own, beside a folder laid out as GitHub's addresses for releases are.
type pinTrial struct {
	t               *testing.T
	root, tree, tmp string
}

func newPin(t *testing.T, lock string) *pinTrial {
	t.Helper()
	p := &pinTrial{t: t, root: t.TempDir(), tree: t.TempDir(), tmp: t.TempDir()}
	write(t, filepath.Join(p.root, "scripts", "pin.sh"), readFile(t, filepath.Join("..", "..", "scripts", "pin.sh")), 0o755)
	write(t, filepath.Join(p.root, "site", "etc", "allowed_signers"), readFile(t, filepath.Join(site, "etc", "allowed_signers")), 0o644)
	write(t, filepath.Join(p.root, "games.lock.json"), []byte(lock), 0o644)
	return p
}

// serve puts a release's manifest, its signature, its pages, its recording and its build for a
// web page where the script asks for NAME's VERSION, and returns that folder.
func (p *pinTrial) serve(release, name, version string) string {
	p.t.Helper()
	dir := filepath.Join(p.tree, name, "releases", "download", version)
	var files []string
	for _, pattern := range []string{"*.html", "*.gif", "*.png", "*.wasm.gz", "wasm_exec.js"} {
		found, err := filepath.Glob(filepath.Join(release, pattern))
		if err != nil {
			p.t.Fatal(err)
		}
		files = append(files, found...)
	}
	for _, f := range append(files, filepath.Join(release, "manifest.json"), filepath.Join(release, "manifest.json.sig")) {
		write(p.t, filepath.Join(dir, filepath.Base(f)), readFile(p.t, f), 0o644)
	}
	return dir
}

// run runs the script with sh, from another folder, with nothing of this machine's own
// environment but its PATH.
func (p *pinTrial) run(args ...string) (status int, out, errs string) {
	p.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "sh", append([]string{filepath.Join(p.root, "scripts", "pin.sh")}, args...)...)
	cmd.Dir = p.t.TempDir()
	cmd.Env = []string{"HOME=" + p.t.TempDir(), "TMPDIR=" + p.tmp, "PATH=" + os.Getenv("PATH"), "PIN_RELEASES=file://" + p.tree}
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	var exit *exec.ExitError
	switch err := cmd.Run(); {
	case errors.As(err, &exit):
		status = exit.ExitCode()
	case err != nil:
		p.t.Fatal(err)
	}
	if left, _ := os.ReadDir(p.tmp); len(left) != 0 {
		p.t.Errorf("wanted the temporary folder empty after the run, and %d entries are left in it", len(left))
	}
	return status, o.String(), e.String()
}

func (p *pinTrial) pinned(name, version string) {
	p.t.Helper()
	if status, out, errs := p.run(name, version); status != 0 || !strings.HasPrefix(out, "Pinned "+name+" "+version+": man/") {
		p.t.Fatalf("wanted %s %s pinned, got %d: %q %q", name, version, status, out, errs)
	}
}

// refused holds that the run stops with what it says, and leaves the lock and every file as they were.
func (p *pinTrial) refused(says string, args ...string) {
	p.t.Helper()
	lock, files := readFile(p.t, filepath.Join(p.root, "games.lock.json")), p.files()
	status, out, errs := p.run(args...)
	if status != 1 || out != "" || !strings.Contains(errs, says) || errs == "" {
		p.t.Errorf("wanted status 1 and to be told %q, got %d: %q %q", says, status, out, errs)
	}
	if string(readFile(p.t, filepath.Join(p.root, "games.lock.json"))) != string(lock) || !maps.Equal(p.files(), files) {
		p.t.Error("wanted games.lock.json, man/, site/media/ and site/play/ left as they were")
	}
}

// files returns what the script may write, man/, site/media/ and site/play/, by path from the
// root.
func (p *pinTrial) files() map[string]string {
	p.t.Helper()
	out := map[string]string{}
	for _, dir := range []string{"man", filepath.Join("site", "media"), filepath.Join("site", "play")} {
		err := filepath.WalkDir(filepath.Join(p.root, dir), func(path string, d fs.DirEntry, err error) error {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			if err != nil || d.IsDir() {
				return err
			}
			rel, err := filepath.Rel(p.root, path)
			if err != nil {
				return err
			}
			out[filepath.ToSlash(rel)] = string(readFile(p.t, path))
			return nil
		})
		if err != nil {
			p.t.Fatal(err)
		}
	}
	return out
}

func (p *pinTrial) lock() map[string]pin {
	p.t.Helper()
	var out map[string]pin
	if err := json.Unmarshal(readFile(p.t, filepath.Join(p.root, "games.lock.json")), &out); err != nil {
		p.t.Fatalf("games.lock.json: %v", err)
	}
	return out
}
