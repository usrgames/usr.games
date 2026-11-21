// Package brand draws usr.games' own letters and everything made from them: the sign, the names
// of the games, the small mark, the icons and the picture for link previews. BRAND.md says what
// each is for. `make site` writes the files, and `make check` fails if one is not what this draws.
package brand

import "fmt"

// A letter is Cols cells wide and Rows tall. Small letters sit in the lower five rows.
const (
	Cols = 5
	Rows = 7
)

// letters are the studio's own drawings, a # for each lit cell. Everything is built only from
// cells, so a terminal can print the same sign.
var letters = map[rune][Rows]string{
	' ': {".....", ".....", ".....", ".....", ".....", ".....", "....."},
	'#': {".#.#.", ".#.#.", "#####", ".#.#.", "#####", ".#.#.", ".#.#."},
	'!': {"..#..", "..#..", "..#..", "..#..", "..#..", ".....", "..#.."},
	'/': {"....#", "....#", "...#.", "..#..", ".#...", "#....", "#...."},
	'(': {"...#.", "..#..", ".#...", ".#...", ".#...", "..#..", "...#."},
	')': {".#...", "..#..", "...#.", "...#.", "...#.", "..#..", ".#..."},
	'.': {".....", ".....", ".....", ".....", ".....", ".##..", ".##.."},
	'1': {"..#..", ".##..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'6': {"..##.", ".#...", "#....", "####.", "#...#", "#...#", ".###."},
	'a': {".....", ".....", ".###.", "....#", ".####", "#...#", ".####"},
	'b': {"#....", "#....", "#.##.", "##..#", "#...#", "#...#", "####."},
	'c': {".....", ".....", ".###.", "#....", "#....", "#....", ".###."},
	'd': {"....#", "....#", ".##.#", "#..##", "#...#", "#...#", ".####"},
	'e': {".....", ".....", ".###.", "#...#", "#####", "#....", ".###."},
	'f': {"..##.", ".#...", "####.", ".#...", ".#...", ".#...", ".#..."},
	'g': {".....", ".....", ".####", "#...#", ".####", "....#", ".###."},
	'h': {"#....", "#....", "#.##.", "##..#", "#...#", "#...#", "#...#"},
	'i': {"..#..", ".....", ".##..", "..#..", "..#..", "..#..", ".###."},
	'k': {"#....", "#....", "#..#.", "#.#..", "##...", "#.#..", "#..#."},
	'l': {".##..", "..#..", "..#..", "..#..", "..#..", "..#..", ".###."},
	'm': {".....", ".....", "##.#.", "#.#.#", "#.#.#", "#...#", "#...#"},
	'n': {".....", ".....", "#.##.", "##..#", "#...#", "#...#", "#...#"},
	'o': {".....", ".....", ".###.", "#...#", "#...#", "#...#", ".###."},
	'p': {".....", ".....", "####.", "#...#", "####.", "#....", "#...."},
	'r': {".....", ".....", "#.##.", "##..#", "#....", "#....", "#...."},
	's': {".....", ".....", ".####", "#....", ".###.", "....#", "####."},
	't': {".#...", ".#...", "####.", ".#...", ".#...", ".#..#", "..##."},
	'u': {".....", ".....", "#...#", "#...#", "#...#", "#..##", ".##.#"},
	'v': {".....", ".....", "#...#", "#...#", "#...#", ".#.#.", "..#.."},
	'w': {".....", ".....", "#...#", "#...#", "#.#.#", "#.#.#", ".#.#."},
	'y': {".....", ".....", "#...#", "#...#", ".####", "....#", ".###."},
}

// Cell is one lit cell of a drawing: its column and its row, from the top left.
type Cell struct{ X, Y int }

// Has reports whether every character of text has a drawing.
func Has(text string) bool {
	for _, r := range text {
		if _, ok := letters[r]; !ok {
			return false
		}
	}
	return true
}

// Cells returns the lit cells of a line of text and its width in cells, with one empty column
// between letters. It panics on a character with no drawing; check with Has first.
func Cells(text string) ([]Cell, int) {
	var on []Cell
	n := 0
	for _, r := range text {
		rows, ok := letters[r]
		if !ok {
			panic(fmt.Sprintf("brand: no drawing for %q", r))
		}
		for y, row := range rows {
			for x, c := range row {
				if c == '#' {
					on = append(on, Cell{n*(Cols+1) + x, y})
				}
			}
		}
		n++
	}
	return on, max(0, n*(Cols+1)-1)
}
