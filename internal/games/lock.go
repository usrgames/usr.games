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
// not among modules, a version that is not vX.Y.Z, a hash that is not one, and anything but
// one manual page.
func LoadLock(path string, modules []string) ([]Pin, error) {
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
		if !slices.Contains(modules, name) {
			return nil, fmt.Errorf("%s: %q is neither a game of games.json nor one of the tools", path, name)
		}
		if !versionText.MatchString(pin.Version) {
			return nil, fmt.Errorf("%s: %s: wanted a version such as v0.1.0, got %q", path, name, pin.Version)
		}
		pages := 0
		for file, sum := range pin.Files {
			if !hashText.MatchString(sum) {
				return nil, fmt.Errorf("%s: %s: %s: wanted a SHA-256 in hex, got %q", path, name, file, sum)
			}
			if _, ok := section(name, file); !ok {
				return nil, fmt.Errorf("%s: %s pins %s, which the site has no use for", path, name, file)
			}
			pages++
		}
		if pages != 1 {
			return nil, fmt.Errorf("%s: %s: wanted one manual page, got %d", path, name, pages)
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

// section returns the section of the manual a file is the page of, if it is the program's
// page as mandoc writes it: NAME.6.html.
func section(name, file string) (string, bool) {
	rest, ok := strings.CutPrefix(file, name+".")
	if !ok || len(rest) != len("6.html") || rest[1:] != ".html" || rest[0] < '1' || rest[0] > '9' {
		return "", false
	}
	return rest[:1], true
}
