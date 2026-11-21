// Package plan reads the devlog's notes in plan/, one text file each. The page at /plan and its
// feed are made from them, so a new note is one new file.
package plan

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
)

// Note is one file of plan/: its first line is the title, and the paragraphs follow, a blank
// line apart.
type Note struct {
	ID    string    // the file's name without .txt, and the note's anchor on the page
	Date  time.Time // the day in the file's name, at midnight UTC
	Title string
	Paras []Para
}

// Para is a paragraph of a note. One that starts with "$ " in the file is a command.
type Para struct {
	Text    string
	Command bool
}

// Day returns the note's date as 2026-11-21.
func (n Note) Day() string { return n.Date.Format(time.DateOnly) }

var (
	name  = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2})-[a-z0-9]+(-[a-z0-9]+)*\.txt$`)
	blank = regexp.MustCompile(`\n\s*\n`)
)

// Load reads every note in dir, newest first, and refuses a folder a page could not be made from.
//
// shortcut: a note is plain text, with no links and no page of its own; upgrade when a note needs either.
func Load(dir string) ([]Note, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var notes []Note
	for _, f := range files {
		m := name.FindStringSubmatch(f.Name())
		if m == nil {
			return nil, fmt.Errorf("%s: a note is named like 2026-11-21-the-sign-is-up.txt, got %q", dir, f.Name())
		}
		date, err := time.Parse(time.DateOnly, m[1])
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", dir, f.Name(), err)
		}
		data, err := os.ReadFile(filepath.Join(dir, f.Name()))
		if err != nil {
			return nil, err
		}
		text := strings.TrimSpace(strings.ReplaceAll(string(data), "\r\n", "\n"))
		blocks := blank.Split(text, -1)
		n := Note{ID: strings.TrimSuffix(f.Name(), ".txt"), Date: date, Title: blocks[0]}
		for _, b := range blocks[1:] {
			// A paragraph may be wrapped in the file; on the page it is one run of text.
			p := Para{Text: strings.Join(strings.Fields(b), " ")}
			if cmd, ok := strings.CutPrefix(p.Text, "$ "); ok {
				p = Para{Text: cmd, Command: true}
			}
			n.Paras = append(n.Paras, p)
		}
		if n.Title == "" || strings.Contains(n.Title, "\n") || len(n.Paras) == 0 {
			return nil, fmt.Errorf("%s: %s: wanted a title on the first line, a blank line, then at least one paragraph", dir, f.Name())
		}
		notes = append(notes, n)
	}
	if len(notes) == 0 {
		return nil, fmt.Errorf("%s: wanted at least one note", dir)
	}
	// The date leads the name, so the names in reverse are the notes newest first.
	slices.SortFunc(notes, func(a, b Note) int { return strings.Compare(b.ID, a.ID) })
	return notes, nil
}
