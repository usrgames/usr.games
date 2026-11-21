// Package games reads games.json, the one list of what is in /usr/games. The pages, the drawn
// names and the tags the go command reads are all made from it, so a new game is one new entry.
package games

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
)

// Game is one entry of games.json, in the order the games were added.
type Game struct {
	Name    string      `json:"name"`
	Players string      `json:"players"`
	Blurb   string      `json:"blurb"`
	About   string      `json:"about"`
	Facts   string      `json:"facts"`
	Picture string      `json:"picture"`
	Keys    [][2]string `json:"keys"`
}

// Tools are the studio's modules that are not games: the launcher and the server.
var Tools = []string{"tty1", "arcade"}

// Load reads the list at path and refuses one a page could not be made from.
func Load(path string) ([]Game, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var list []Game
	if err := json.Unmarshal(data, &list); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	seen := map[string]bool{}
	for _, g := range list {
		switch {
		case g.Name == "" || strings.Trim(g.Name, "abcdefghijklmnopqrstuvwxyz") != "":
			return nil, fmt.Errorf("%s: a name is lowercase letters only, got %q", path, g.Name)
		case seen[g.Name] || slices.Contains(Tools, g.Name):
			return nil, fmt.Errorf("%s: the name %q is used twice", path, g.Name)
		case !slices.Contains([]string{"1p", "2p", "2p net", "net"}, g.Players):
			return nil, fmt.Errorf("%s: %s: players is 1p, 2p, 2p net or net, got %q", path, g.Name, g.Players)
		case g.Blurb == "" || g.About == "" || g.Facts == "" || g.Picture == "" || len(g.Keys) == 0:
			return nil, fmt.Errorf("%s: %s: a blurb, an about, facts, a picture and keys are all needed", path, g.Name)
		}
		seen[g.Name] = true
	}
	return list, nil
}

// ByName returns the games from A to Z, the order a listing prints them in.
func ByName(list []Game) []Game {
	out := slices.Clone(list)
	slices.SortFunc(out, func(a, b Game) int { return strings.Compare(a.Name, b.Name) })
	return out
}

// Modules returns every module the site answers the go command for: the games in the order
// they were added, then the tools.
func Modules(list []Game) []string {
	var out []string
	for _, g := range list {
		out = append(out, g.Name)
	}
	return append(out, Tools...)
}
