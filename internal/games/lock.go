package games

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

// Pin is one release the site pins in games.lock.json: the program, its version, and the
// SHA-256 of each of the release's files the site uses. scripts/pin.sh writes it, once the
// studio's signature on the release checks out.
type Pin struct {
	Name    string            `json:"-"`
	Version string            `json:"version"`
	Files   map[string]string `json:"files"`
}

var (
	versionText = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`)
	hashText    = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// LoadLock reads the lock at path and returns its pins, A to Z. It refuses a pin of a program
// not among names, a version that is not vX.Y.Z, a hash that is not one, and anything but
// one manual page, a game's recording (NAME.gif and its still, NAME.png) and a game's build for
// a web page (NAME.wasm.gz and the wasm_exec.js of the Go that built it), each both or neither.
func LoadLock(path string, names []string) ([]Pin, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var lock map[string]Pin
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&lock); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	var out []Pin
	for name, pin := range lock {
		pin.Name = name
		if !slices.Contains(names, name) {
			return nil, fmt.Errorf("%s: %q is neither a game of games.json nor one of the tools", path, name)
		}
		if !versionText.MatchString(pin.Version) {
			return nil, fmt.Errorf("%s: %s: wanted a version such as v0.1.0, got %q", path, name, pin.Version)
		}
		pages, recording, build := 0, 0, 0
		for file, sum := range pin.Files {
			if !hashText.MatchString(sum) {
				return nil, fmt.Errorf("%s: %s: %s: wanted a SHA-256 in hex, got %q", path, name, file, sum)
			}
			switch _, ok := section(name, file); {
			case ok:
				pages++
			case file == name+".gif" || file == name+".png":
				recording++
			case file == name+".wasm.gz" || file == "wasm_exec.js":
				build++
			default:
				return nil, fmt.Errorf("%s: %s pins %s, which the site has no use for", path, name, file)
			}
		}
		if pages != 1 {
			return nil, fmt.Errorf("%s: %s: wanted one manual page, got %d", path, name, pages)
		}
		if recording == 1 {
			return nil, fmt.Errorf("%s: %s: wanted a recording as both %s.gif and its still, %s.png, got one of them", path, name, name, name)
		}
		if build == 1 {
			return nil, fmt.Errorf("%s: %s: wanted a build for a web page as both %s.wasm.gz and wasm_exec.js, got one of them", path, name, name)
		}
		out = append(out, pin)
	}
	slices.SortFunc(out, func(a, b Pin) int { return strings.Compare(a.Name, b.Name) })
	return out, nil
}

// Manual returns the pin's manual page and the section of the manual it is in.
func (p Pin) Manual() (file, sect string) {
	for f := range p.Files {
		if s, ok := section(p.Name, f); ok {
			return f, s
		}
	}
	return "", ""
}

// Recorded says whether the pin holds a recording of the game: NAME.gif, and NAME.png, its still.
func (p Pin) Recorded() bool {
	_, ok := p.Files[p.Name+".gif"]
	return ok
}

// Playable says whether the pin holds a build of the game for a web page, so that it can be
// played there.
func (p Pin) Playable() bool {
	_, ok := p.Files[p.Name+".wasm.gz"]
	return ok
}

// Path returns where the site keeps a pinned file of a program's release, from the repository's
// root: a manual page in man/, which its page on the site is made from, a recording where the
// site serves it, and a build for a web page at an address that carries its version, so that it
// can be kept as long as a browser likes.
func Path(name, version, file string) string {
	switch {
	case strings.HasSuffix(file, ".html"):
		return "man/" + file
	case file == name+".wasm.gz" || file == "wasm_exec.js":
		return "site/play/" + name + "/" + version + "/" + file
	}
	return "site/media/" + name + "/" + file
}

// section returns the section of the manual a file is the page of, if it is the program's
// page as mandoc writes it: NAME.6.html.
func section(name, file string) (string, bool) {
	rest, ok := strings.CutPrefix(file, name+".")
	if !ok || len(rest) != len("6.html") || rest[1:] != ".html" || rest[0] < '1' || rest[0] > '9' {
		return "", false
	}
	return rest[:1], true
}
