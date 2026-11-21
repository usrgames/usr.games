package brand

import "math"

// Sketch returns a stand-in picture of a game, 32 by 16 cells, and whether there is one.
//
// These are placeholders. A game's page is meant to show a recording of the game itself, which
// arrives with the game's release; until then the page has a sketch, so it is not empty.
func Sketch(game string) (string, bool) {
	const w, h = 32, 16
	var cells []Cell
	lit := map[Cell]bool{}
	dot := func(x, y int) {
		if c := (Cell{x, y}); x >= 0 && x < w && y >= 0 && y < h && !lit[c] {
			lit[c] = true
			cells = append(cells, c)
		}
	}
	cut := func(x, y int) {
		for i, c := range cells {
			if c == (Cell{x, y}) {
				cells = append(cells[:i], cells[i+1:]...)
				delete(lit, c)
				return
			}
		}
	}
	line := func(x0, y0, x1, y1, step int) {
		n := max(abs(x1-x0), abs(y1-y0), 1)
		for i := 0; i <= n; i += step {
			t := float64(i) / float64(n)
			dot(int(math.Floor(float64(x0)+float64(x1-x0)*t+0.5)), int(math.Floor(float64(y0)+float64(y1-y0)*t+0.5)))
		}
	}
	box := func(x0, y0, x1, y1 int, solid bool) {
		for y := y0; y <= y1; y++ {
			for x := x0; x <= x1; x++ {
				if solid || x == x0 || x == x1 || y == y0 || y == y1 {
					dot(x, y)
				}
			}
		}
	}

	switch game {
	case "rustwake": // a corridor, a door at its end, a drone, your gun
		line(0, 0, 11, 5, 1)
		line(31, 0, 20, 5, 1)
		line(0, 15, 11, 10, 1)
		line(31, 15, 20, 10, 1)
		box(11, 5, 20, 10, false)
		box(14, 7, 17, 10, true)
		dot(5, 7)
		dot(6, 7)
		dot(7, 7)
		dot(6, 6)
		dot(6, 8)
		box(15, 13, 16, 15, true)
	case "skyspear": // stepped land, a tree, a hunter with a spear, a falling star
		top := func(x int) int {
			switch {
			case x < 10:
				return 11
			case x < 16:
				return 10
			case x < 22:
				return 9
			}
			return 11
		}
		for x := range w {
			dot(x, top(x))
			for y := top(x) + 1; y < h; y++ {
				if (x+y)%2 == 0 {
					dot(x, y)
				}
			}
		}
		box(3, 6, 7, 8, true)
		line(5, 9, 5, 10, 1)
		dot(18, 5)
		line(18, 6, 18, 8, 1)
		line(20, 3, 20, 8, 1)
		line(30, 1, 25, 4, 1)
		box(23, 4, 24, 5, true)
	case "breakshot": // a pool table, the rack, the cue ball and where it is aimed
		box(2, 3, 29, 13, false)
		for _, p := range [][2]int{{2, 3}, {29, 3}, {2, 13}, {29, 13}, {15, 3}, {16, 3}, {15, 13}, {16, 13}} {
			cut(p[0], p[1])
		}
		for c := range 4 {
			for r := 0; r <= c; r++ {
				dot(21+2*c, 8-c+2*r)
			}
		}
		dot(9, 8)
		line(11, 8, 19, 8, 2)
		line(4, 8, 7, 8, 1)
	default:
		return "", false
	}
	return svg(cells, w, h, "a sketch of "+game, false), true
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
