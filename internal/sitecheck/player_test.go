package sitecheck

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The player a game's page plays with: glonk.js and xterm.js, its WebGL addon and its stylesheet,
// from one tag of glonk, and the licences of xterm.js and its addon beside them, since the MIT
// licence travels with the code.
var playerFiles = []string{"LICENSE-addon-webgl.txt", "LICENSE-xterm.txt", "addon-webgl.js", "glonk.js", "xterm.css", "xterm.js"}

// player returns the folder of the one version of glonk's player the site keeps, and its version.
func player(t *testing.T) (string, string) {
	t.Helper()
	found, err := filepath.Glob(filepath.Join(site, "play", "glonk", "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 {
		t.Fatalf("wanted one version of glonk's player in site/play/glonk (scripts/player.sh keeps it), got %d", len(found))
	}
	return found[0], filepath.Base(found[0])
}

func TestThePlayerIsGlonksWithItsLicences(t *testing.T) {
	dir, _ := player(t)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, e := range entries {
		got = append(got, e.Name())
	}
	if !slices.Equal(got, playerFiles) {
		t.Errorf("wanted %v in %s, got %v", playerFiles, dir, got)
	}
	for _, licence := range []string{"LICENSE-xterm.txt", "LICENSE-addon-webgl.txt"} {
		if text := read(t, "site", "play", "glonk", filepath.Base(dir), licence); !strings.Contains(text, "Permission is hereby granted, free of charge") {
			t.Errorf("%s: wanted the text of the MIT licence", licence)
		}
	}
}

// scripts/player.sh downloads the player from glonk's tag on GitHub. It is tried here on a
// folder laid out as GitHub's addresses for a tag's files are.
func TestThePlayerScript(t *testing.T) {
	tree := t.TempDir()
	at := map[string]string{
		"web/glonk.js": "glonk.js", "site/media/xterm/xterm.js": "xterm.js", "site/media/xterm/addon-webgl.js": "addon-webgl.js",
		"site/media/xterm/xterm.css": "xterm.css", "site/media/xterm/LICENSE-xterm.txt": "xterm's licence",
		"site/media/xterm/LICENSE-addon-webgl.txt": "the addon's licence",
	}
	for path, text := range at {
		write(t, filepath.Join(tree, "v0.16.2", filepath.FromSlash(path)), []byte(text), 0o644)
	}
	newRoot := func(t *testing.T) string {
		root := t.TempDir()
		write(t, filepath.Join(root, "scripts", "player.sh"), readFile(t, filepath.Join("..", "..", "scripts", "player.sh")), 0o755)
		write(t, filepath.Join(root, "games.lock.json"), []byte("{}\n"), 0o644)
		write(t, filepath.Join(root, "site", "play", "glonk", "v0.16.0", "glonk.js"), []byte("an older glonk.js"), 0o644)
		return root
	}
	run := func(t *testing.T, root string, args ...string) (int, string) {
		cmd := exec.Command("sh", append([]string{filepath.Join(root, "scripts", "player.sh")}, args...)...)
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "TMPDIR=" + t.TempDir(), "PLAYER_SOURCE=file://" + tree}
		out, err := cmd.CombinedOutput()
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode(), string(out)
		} else if err != nil {
			t.Fatal(err)
		}
		return 0, string(out)
	}

	t.Run("it keeps the tag's player in place of the one before", func(t *testing.T) {
		root := newRoot(t)
		if status, out := run(t, root, "v0.16.2"); status != 0 {
			t.Fatalf("wanted it kept, got %d: %s", status, out)
		}
		for path, text := range at {
			if got := string(readFile(t, filepath.Join(root, "site", "play", "glonk", "v0.16.2", filepath.Base(path)))); got != text {
				t.Errorf("%s: wanted %q, got %q", path, text, got)
			}
		}
		if _, err := os.Stat(filepath.Join(root, "site", "play", "glonk", "v0.16.0")); !os.IsNotExist(err) {
			t.Errorf("wanted the older player gone, got %v", err)
		}
	})

	for _, c := range []struct{ what, version, says string }{
		{"a tag that is not there", "v0.9.9", "could not be downloaded"},
		{"a version that is not one", "main", "is not a version"},
	} {
		t.Run(c.what, func(t *testing.T) {
			root := newRoot(t)
			if status, out := run(t, root, c.version); status != 1 || !strings.Contains(out, c.says) {
				t.Errorf("wanted status 1 and to be told %q, got %d: %s", c.says, status, out)
			}
			if got := string(readFile(t, filepath.Join(root, "site", "play", "glonk", "v0.16.0", "glonk.js"))); got != "an older glonk.js" {
				t.Error("wanted the player left as it was")
			}
		})
	}
}
