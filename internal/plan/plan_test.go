package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func folder(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestLoadReadsNotesNewestFirst(t *testing.T) {
	notes, err := Load(folder(t, map[string]string{
		"2026-11-21-first.txt":  "First\n\nOne line\nwrapped.\n\n$ ls -l\n",
		"2026-12-02-second.txt": "Second\r\n\r\nLater.\r\n",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 2 || notes[0].ID != "2026-12-02-second" || notes[1].ID != "2026-11-21-first" {
		t.Fatalf("wanted the two notes newest first, got %+v", notes)
	}
	first := notes[1]
	if first.Title != "First" || first.Day() != "2026-11-21" {
		t.Errorf("wanted the title First on 2026-11-21, got %q on %s", first.Title, first.Day())
	}
	want := []Para{{Text: "One line wrapped."}, {Text: "ls -l", Command: true}}
	if len(first.Paras) != 2 || first.Paras[0] != want[0] || first.Paras[1] != want[1] {
		t.Errorf("wanted %+v, got %+v", want, first.Paras)
	}
}

func TestLoadRefusesWhatAPageCouldNotBeMadeFrom(t *testing.T) {
	for what, files := range map[string]map[string]string{
		"an empty folder":           {},
		"a name with no date":       {"notes.txt": "Title\n\nText.\n"},
		"a day that does not exist": {"2026-13-40-x.txt": "Title\n\nText.\n"},
		"a title and nothing else":  {"2026-11-21-x.txt": "Title\n"},
		"a title on two lines":      {"2026-11-21-x.txt": "Title\nmore title\n\nText.\n"},
		"an empty file":             {"2026-11-21-x.txt": "\n"},
	} {
		if notes, err := Load(folder(t, files)); err == nil {
			t.Errorf("%s: wanted it refused, got %+v", what, notes)
		}
	}
}
