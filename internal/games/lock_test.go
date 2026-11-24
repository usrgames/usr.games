package games

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The pages and the drawings are made from what the lock pins, so a lock that names what the
// site cannot use, or names it wrongly, is refused before anything is made from it.
func TestTheLockHoldsOnlyWhatTheSiteUses(t *testing.T) {
	hash := strings.Repeat("a", 64)
	names := []string{"rustwake", "tty1"}
	for _, c := range []struct {
		what, lock, refused string
	}{
		{"a manual page", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `"}, "version": "v0.1.0"}}`, ""},
		{"a section 1 page", `{"tty1": {"files": {"tty1.1.html": "` + hash + `"}, "version": "v10.20.30"}}`, ""},
		{"nothing pinned", `{}`, ""},
		{"a recording", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "rustwake.gif": "` + hash + `", "rustwake.png": "` + hash + `"}, "version": "v0.1.0"}}`, ""},
		{"a recording with no still", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "rustwake.gif": "` + hash + `"}, "version": "v0.1.0"}}`, "got one of them"},
		{"a still with no recording", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "rustwake.png": "` + hash + `"}, "version": "v0.1.0"}}`, "got one of them"},
		{"a build for a web page", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "rustwake.wasm.gz": "` + hash + `", "wasm_exec.js": "` + hash + `"}, "version": "v0.1.0"}}`, ""},
		{"a build with no wasm_exec.js", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "rustwake.wasm.gz": "` + hash + `"}, "version": "v0.1.0"}}`, "got one of them"},
		{"a wasm_exec.js with no build", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "wasm_exec.js": "` + hash + `"}, "version": "v0.1.0"}}`, "got one of them"},
		{"another game's recording", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "tty1.gif": "` + hash + `", "tty1.png": "` + hash + `"}, "version": "v0.1.0"}}`, "has no use for"},
		{"a program the site does not list", `{"pong": {"files": {"pong.6.html": "` + hash + `"}, "version": "v0.1.0"}}`, `"pong" is neither`},
		{"a version with a suffix", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `"}, "version": "v0.1.0-rc.1"}}`, "wanted a version"},
		{"no manual page", `{"rustwake": {"files": {}, "version": "v0.1.0"}}`, "wanted one manual page, got 0"},
		{"another program's page", `{"rustwake": {"files": {"tty1.1.html": "` + hash + `"}, "version": "v0.1.0"}}`, "has no use for"},
		{"a file the site has no use for", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "rustwake_linux_amd64.tar.gz": "` + hash + `"}, "version": "v0.1.0"}}`, "has no use for"},
		{"two manual pages", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `", "rustwake.1.html": "` + hash + `"}, "version": "v0.1.0"}}`, "got 2"},
		{"a hash that is not one", `{"rustwake": {"files": {"rustwake.6.html": "AA"}, "version": "v0.1.0"}}`, "wanted a SHA-256"},
		{"a field it does not know", `{"rustwake": {"files": {"rustwake.6.html": "` + hash + `"}, "version": "v0.1.0", "url": "x"}}`, "unknown field"},
	} {
		t.Run(c.what, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "games.lock.json")
			if err := os.WriteFile(path, []byte(c.lock), 0o644); err != nil {
				t.Fatal(err)
			}
			pins, err := LoadLock(path, names)
			switch {
			case c.refused == "" && err != nil:
				t.Fatalf("wanted it read, got %v", err)
			case c.refused != "" && (err == nil || !strings.Contains(err.Error(), c.refused)):
				t.Fatalf("wanted it refused with %q, got %v", c.refused, err)
			case c.refused == "" && len(pins) == 1:
				if file, sect := pins[0].Manual(); !strings.HasPrefix(file, pins[0].Name+"."+sect+".") {
					t.Errorf("wanted the manual page and its section, got %q and %q", file, sect)
				}
				_, gif := pins[0].Files[pins[0].Name+".gif"]
				if pins[0].Recorded() != gif {
					t.Errorf("wanted Recorded to say %v", gif)
				}
				_, wasm := pins[0].Files[pins[0].Name+".wasm.gz"]
				if pins[0].Playable() != wasm {
					t.Errorf("wanted Playable to say %v", wasm)
				}
			}
		})
	}
}
