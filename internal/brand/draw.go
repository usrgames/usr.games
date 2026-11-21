package brand

import (
	"bytes"
	"encoding/binary"
	"html"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"strconv"
	"strings"
)

// The colours of the one green tube. BRAND.md and site/style.css name the same four, and tests
// hold both to these.
const (
	Glass    = "#030a05" // the screen
	Phosphor = "#45ff86" // plain
	Bright   = "#d6ffe4" // numbers and headings worth looking at
	Dim      = "#2fd16a" // what can be read second
)

// SignText is the studio's sign: a shebang line that points at the Unix games directory.
const SignText = "#!/usr/games"

// CardLine is what the sign says beneath itself on the link-preview card. The site sets the same
// words in type, with capitals; the card has only the letters.
const CardLine = "an arcade in your terminal. free play."

// inset leaves a gap round each cell, so the grid the letters are made of stays visible.
const inset = 0.07

func num(v float64) string { return strconv.FormatFloat(v, 'f', -1, 64) }

// svg draws cells as squares on a transparent ground, in Phosphor. With a smear, each cell
// trails a faint copy half a cell to its left: phosphor keeps glowing after the beam moves on.
func svg(cells []Cell, w, h int, label string, smear bool) string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="-0.5 -0.5 ` + strconv.Itoa(w+1) + ` ` + strconv.Itoa(h+1) + `" role="img" aria-label="` + html.EscapeString(label) + `">`)
	b.WriteString(`<title>` + html.EscapeString(label) + `</title>`)
	side := num(1 - 2*inset)
	rect := func(x, y float64) {
		b.WriteString(`<rect x="` + num(x) + `" y="` + num(y) + `" width="` + side + `" height="` + side + `"/>`)
	}
	if smear {
		b.WriteString(`<g fill="` + Phosphor + `" opacity="0.24">`)
		for _, c := range cells {
			rect(float64(c.X)-0.5, float64(c.Y)+inset)
		}
		b.WriteString(`</g>`)
	}
	b.WriteString(`<g fill="` + Phosphor + `">`)
	for _, c := range cells {
		rect(float64(c.X)+inset, float64(c.Y)+inset)
	}
	b.WriteString("</g></svg>\n")
	return b.String()
}

// Sign returns the sign as an SVG, with the smear.
func Sign() string {
	cells, w := Cells(SignText)
	return svg(cells, w, Rows, SignText, true)
}

// Name returns a name set in the letters, as an SVG: "rustwake(6)", or "tty1".
func Name(text string) string {
	cells, w := Cells(text)
	return svg(cells, w, Rows, text, false)
}

// The small mark is the letter u alone, on the glass. Its five rows of ink sit in a square of
// eight cells, so at 16 pixels every cell is two whole pixels.
const markCells = 8

func markInk() []Cell {
	cells, _ := Cells("u")
	for i := range cells {
		cells[i].Y -= Rows - Cols // a small letter's ink starts two rows down
	}
	return cells
}

// Mark returns the small mark as an SVG, for browsers that take one as an icon.
func Mark() string {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 16 16" role="img" aria-label="usr.games"><title>usr.games</title>`)
	b.WriteString(`<rect width="16" height="16" fill="` + Glass + `"/><g fill="` + Phosphor + `">`)
	for _, c := range markInk() {
		b.WriteString(`<rect x="` + strconv.Itoa(3+2*c.X) + `" y="` + strconv.Itoa(3+2*c.Y) + `" width="2" height="2"/>`)
	}
	b.WriteString("</g></svg>\n")
	return b.String()
}

func rgb(hex string) color.RGBA {
	v, err := strconv.ParseUint(strings.TrimPrefix(hex, "#"), 16, 32)
	if err != nil {
		panic(err)
	}
	return color.RGBA{uint8(v >> 16), uint8(v >> 8), uint8(v), 0xff}
}

func fill(img *image.RGBA, r image.Rectangle, c color.RGBA) {
	draw.Draw(img, r, image.NewUniform(c), image.Point{}, draw.Src)
}

// MarkPNG returns the small mark size pixels square, whole cells with no gap between them.
func MarkPNG(size int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	fill(img, img.Bounds(), rgb(Glass))
	at := func(cell int) int { return int(math.Round(float64(size) * (1.5 + float64(cell)) / markCells)) }
	for _, c := range markInk() {
		fill(img, image.Rect(at(c.X), at(c.Y), at(c.X+1), at(c.Y+1)), rgb(Phosphor))
	}
	return img
}

// Card returns the picture a chat or a social site shows beside a link: the sign and its line,
// 1280 by 640.
func Card() *image.RGBA {
	const w, h, signUnit, lineUnit, gap = 1280, 640, 14, 4, 56
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	glass, phosphor := rgb(Glass), rgb(Phosphor)
	fill(img, img.Bounds(), glass)

	sign, signW := Cells(SignText)
	line, lineW := Cells(CardLine)
	top := (h - (Rows*signUnit + gap + Rows*lineUnit)) / 2
	left := (w - signW*signUnit) / 2
	// The smear first, mixed into the glass as the SVG's opacity would mix it.
	mix := func(a, b uint8) uint8 { return uint8(math.Round(float64(a) + (float64(b)-float64(a))*0.24)) }
	faint := color.RGBA{mix(glass.R, phosphor.R), mix(glass.G, phosphor.G), mix(glass.B, phosphor.B), 0xff}
	for pass, c := range []color.RGBA{faint, phosphor} {
		shift := (1 - pass) * signUnit / 2
		for _, cell := range sign {
			x, y := left+cell.X*signUnit+1-shift, top+cell.Y*signUnit+1
			fill(img, image.Rect(x, y, x+signUnit-2, y+signUnit-2), c)
		}
	}
	left, top = (w-lineW*lineUnit)/2, top+Rows*signUnit+gap
	for _, cell := range line {
		x, y := left+cell.X*lineUnit, top+cell.Y*lineUnit
		fill(img, image.Rect(x, y, x+lineUnit, y+lineUnit), rgb(Bright))
	}
	return img
}

// PNG encodes a picture.
func PNG(img image.Image) []byte {
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

// ICO returns the small mark as an icon file holding one PNG for each size.
func ICO(sizes ...int) []byte {
	var head, body bytes.Buffer
	binary.Write(&head, binary.LittleEndian, []uint16{0, 1, uint16(len(sizes))})
	offset := 6 + 16*len(sizes)
	for _, size := range sizes {
		data := PNG(MarkPNG(size))
		head.Write([]byte{byte(size), byte(size), 0, 0}) // width, height, no palette, reserved
		binary.Write(&head, binary.LittleEndian, []uint16{1, 32})
		binary.Write(&head, binary.LittleEndian, []uint32{uint32(len(data)), uint32(offset)})
		body.Write(data)
		offset += len(data)
	}
	return append(head.Bytes(), body.Bytes()...)
}
