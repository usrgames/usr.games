package brand

// The drawn files of the site are made here. To write them again after a change:
//
//	go test ./internal/brand -run TestTheFilesAreWhatTheSourceDraws -update
//
// or `make site`, which also writes the pages.

import (
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"flag"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/usrgames/usr.games/internal/games"
)

var update = flag.Bool("update", false, "write the drawn files again from this source")

func root(path string) string { return filepath.Join("..", "..", filepath.FromSlash(path)) }

func list(t *testing.T) []games.Game {
	t.Helper()
	gs, err := games.Load(root("games.json"))
	if err != nil {
		t.Fatal(err)
	}
	return gs
}

// A drawn file is text (an SVG), one picture (a PNG), or an icon holding several.
type drawn struct {
	path string
	text string
	pic  image.Image
	ico  []int
}

func files(t *testing.T) []drawn {
	t.Helper()
	out := []drawn{
		{path: "site/sign.svg", text: Sign()},
		{path: "site/favicon.svg", text: Mark()},
		{path: "site/favicon.ico", ico: []int{16, 32}},
		{path: "site/apple-touch-icon.png", pic: MarkPNG(180)},
		{path: "site/icon-512.png", pic: MarkPNG(512)},
		{path: "site/card.png", pic: Card()},
	}
	for _, g := range list(t) {
		sketch, ok := Sketch(g.Name)
		if !ok {
			t.Fatalf("wanted a sketch of %s until its recording arrives; add one to sketch.go", g.Name)
		}
		out = append(out,
			drawn{path: "site/media/" + g.Name + "/name.svg", text: Name(g.Name + "(6)")},
			drawn{path: "site/media/" + g.Name + "/sketch.svg", text: sketch})
	}
	for _, tool := range games.Tools {
		out = append(out, drawn{path: "site/media/" + tool + "/name.svg", text: Name(tool)})
	}
	return out
}

func samePicture(t *testing.T, what string, got, want image.Image) {
	t.Helper()
	if got.Bounds() != want.Bounds() {
		t.Errorf("%s: wanted %v, got %v (run with -update to draw it again)", what, want.Bounds(), got.Bounds())
		return
	}
	b := want.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gr, gg, gb, ga := got.At(x, y).RGBA()
			wr, wg, wb, wa := want.At(x, y).RGBA()
			if gr != wr || gg != wg || gb != wb || ga != wa {
				t.Errorf("%s: the pixel at %d,%d is not what the source draws (run with -update to draw it again)", what, x, y)
				return
			}
		}
	}
}

// icons returns the pictures inside an icon file, by their size in pixels.
func icons(t *testing.T, data []byte) map[int]image.Image {
	t.Helper()
	var head struct{ Reserved, Type, Count uint16 }
	r := bytes.NewReader(data)
	if err := binary.Read(r, binary.LittleEndian, &head); err != nil || head.Reserved != 0 || head.Type != 1 {
		t.Fatalf("wanted an icon file, got a header of %+v (%v)", head, err)
	}
	out := map[int]image.Image{}
	for range head.Count {
		var e struct {
			W, H, Colors, Reserved uint8
			Planes, Bits           uint16
			Size, Offset           uint32
		}
		if err := binary.Read(r, binary.LittleEndian, &e); err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(io.NewSectionReader(bytes.NewReader(data), int64(e.Offset), int64(e.Size)))
		if err != nil {
			t.Fatalf("the %d pixel icon: %v", e.W, err)
		}
		out[int(e.W)] = img
	}
	return out
}

// PNGs are compared pixel by pixel, not byte by byte: another Go may pack the same picture
// into different bytes.
func TestTheFilesAreWhatTheSourceDraws(t *testing.T) {
	for _, f := range files(t) {
		if *update {
			data := []byte(f.text)
			switch {
			case f.pic != nil:
				data = PNG(f.pic)
			case f.ico != nil:
				data = ICO(f.ico...)
			}
			if err := os.MkdirAll(filepath.Dir(root(f.path)), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(root(f.path), data, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		got, err := os.ReadFile(root(f.path))
		if err != nil {
			t.Errorf("wanted %s (run with -update to draw it): %v", f.path, err)
			continue
		}
		switch {
		case f.pic != nil:
			img, err := png.Decode(bytes.NewReader(got))
			if err != nil {
				t.Errorf("%s: %v", f.path, err)
				continue
			}
			samePicture(t, f.path, img, f.pic)
		case f.ico != nil:
			inside := icons(t, got)
			if len(inside) != len(f.ico) {
				t.Errorf("%s: wanted %d pictures inside, got %d", f.path, len(f.ico), len(inside))
			}
			for _, size := range f.ico {
				if inside[size] == nil {
					t.Errorf("%s: wanted a %d pixel picture inside", f.path, size)
					continue
				}
				samePicture(t, f.path, inside[size], MarkPNG(size))
			}
		case string(got) != f.text:
			t.Errorf("%s is not what the source draws (run with -update to draw it again)", f.path)
		}
	}
}

func TestEveryLetterIsFiveBySeven(t *testing.T) {
	for r, rows := range letters {
		for y, row := range rows {
			if len(row) != Cols || strings.Trim(row, "#.") != "" {
				t.Errorf("%q, row %d: wanted %d cells of # or ., got %q", r, y, Cols, row)
			}
		}
	}
	for _, text := range []string{SignText, CardLine} {
		if !Has(text) {
			t.Errorf("wanted a drawing for every character of %q", text)
		}
	}
}

func TestEveryNameCanBeDrawn(t *testing.T) {
	for _, m := range games.Modules(list(t)) {
		if !Has(m + "(6)") {
			t.Errorf("wanted a drawing for every letter of %q; add the missing ones to letters.go", m)
		}
	}
}

// The sign is twelve letters, six cells apart, and lit only inside its own box.
func TestTheSignIsTwelveLettersOfCells(t *testing.T) {
	cells, w := Cells(SignText)
	if want := 12*(Cols+1) - 1; w != want {
		t.Errorf("wanted the sign %d cells wide, got %d", want, w)
	}
	if len(cells) == 0 {
		t.Fatal("wanted the sign to have lit cells")
	}
	for _, c := range cells {
		if c.X < 0 || c.X >= w || c.Y < 0 || c.Y >= Rows {
			t.Errorf("wanted every cell inside %d by %d, got %v", w, Rows, c)
		}
	}
}

// At 16 pixels the small mark has no room for blur: each of the letter's cells must be two
// whole pixels, and the rest must be glass.
func TestTheSmallMarkIsSharpAtSixteenPixels(t *testing.T) {
	img := MarkPNG(16)
	ink := map[Cell]bool{}
	for _, c := range markInk() {
		ink[c] = true
	}
	for y := range 16 {
		for x := range 16 {
			want := rgb(Glass)
			if x >= 3 && x < 13 && y >= 3 && y < 13 && ink[Cell{(x - 3) / 2, (y - 3) / 2}] {
				want = rgb(Phosphor)
			}
			if got := img.RGBAAt(x, y); got != want {
				t.Fatalf("the pixel at %d,%d: wanted %v, got %v", x, y, want, got)
			}
		}
	}
}

func luminance(hex string) float64 {
	c := rgb(hex)
	lin := func(v uint8) float64 {
		f := float64(v) / 255
		if f <= 0.03928 {
			return f / 12.92
		}
		return math.Pow((f+0.055)/1.055, 2.4)
	}
	return 0.2126*lin(c.R) + 0.7152*lin(c.G) + 0.0722*lin(c.B)
}

// Every voice of the one colour is used for words, so each must be readable on the glass, and
// the glass on plain phosphor, which is how a reversed row is drawn.
func TestEveryVoiceCanBeReadOnTheGlass(t *testing.T) {
	for _, hex := range []string{Phosphor, Bright, Dim} {
		hi, lo := max(luminance(hex), luminance(Glass)), min(luminance(hex), luminance(Glass))
		if ratio := (hi + 0.05) / (lo + 0.05); ratio < 4.5 {
			t.Errorf("wanted %s on the glass at 4.5 to 1 or more, got %.2f", hex, ratio)
		}
	}
}

func TestBrandMdNamesTheColoursTheSourceUses(t *testing.T) {
	doc, err := os.ReadFile(root("BRAND.md"))
	if err != nil {
		t.Fatal(err)
	}
	for _, hex := range []string{Glass, Phosphor, Bright, Dim} {
		if !strings.Contains(string(doc), "`"+hex+"`") {
			t.Errorf("BRAND.md: wanted it to name %s", hex)
		}
	}
	if !strings.Contains(string(doc), "`"+SignText+"`") {
		t.Errorf("BRAND.md: wanted it to show the sign, %s", SignText)
	}
}

func TestTheCardIsWhatALinkPreviewWants(t *testing.T) {
	img := Card()
	if got := img.Bounds().Size(); got != (image.Point{1280, 640}) {
		t.Errorf("wanted the card 1280 by 640, got %v", got)
	}
	if n := len(PNG(img)); n > 1<<20 {
		t.Errorf("wanted the card under 1 MB, got %d bytes", n)
	}
	// Nothing lit within 80 pixels of an edge: some sites crop the picture.
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if (x < 80 || x >= b.Max.X-80 || y < 80 || y >= b.Max.Y-80) && img.RGBAAt(x, y) != rgb(Glass) {
				t.Fatalf("wanted only glass near the edge, found ink at %d,%d", x, y)
			}
		}
	}
}

func TestTheSVGsAreWellFormed(t *testing.T) {
	for _, f := range files(t) {
		if f.text == "" {
			continue
		}
		d := xml.NewDecoder(strings.NewReader(f.text))
		for {
			if _, err := d.Token(); err == io.EOF {
				break
			} else if err != nil {
				t.Errorf("%s: %v", f.path, err)
				break
			}
		}
	}
}
