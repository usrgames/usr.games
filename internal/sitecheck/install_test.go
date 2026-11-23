package sitecheck

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// The installer, site/install.sh, is what the front page's one line pipes to sh. It carries the
// root's key in itself, so that no download and no variable can change whose signature it
// accepts, and that key has to be the one the site publishes.
func TestTheInstallerTrustsTheSitesRootKeyAlone(t *testing.T) {
	var published, carried []string
	for _, line := range strings.Split(read(t, "site", "etc", "allowed_signers"), "\n") {
		if line != "" && !strings.HasPrefix(line, "#") {
			published = append(published, line)
		}
	}
	script := read(t, "site", "install.sh")
	for _, line := range strings.Split(script, "\n") {
		if key, ok := strings.CutPrefix(line, "root='"); ok {
			carried = append(carried, strings.TrimSuffix(key, "'"))
		}
	}
	if len(published) != 1 || !slices.Equal(carried, published) {
		t.Errorf("install.sh: wanted the one key of allowed_signers, %q, got %q", published, carried)
	}
	// A download cut short must run nothing, so the one line that runs anything is the last.
	lines := strings.Split(strings.TrimRight(script, "\n"), "\n")
	if last := lines[len(lines)-1]; last != `main "$@"` {
		t.Errorf(`install.sh: wanted its last line to be main "$@", got %q`, last)
	}
	// Someone careful reads it before piping it to sh, so a browser shows it.
	if got := headers(t, "/install.sh")["Content-Type"]; !strings.HasPrefix(got, "text/plain") {
		t.Errorf("_headers: wanted /install.sh served as text/plain, got %q", got)
	}
}

// What the installer does, run as a player runs it, piped to sh. It is tried on copies of the
// latest real releases, signed with the studio's real key, in the folders TTY1_RELEASE and
// RELAY_RELEASE name; CI downloads them and runs this by name. Every sh and dash the machine has
// runs every case.
func TestTheInstaller(t *testing.T) {
	tty1, relay := os.Getenv("TTY1_RELEASE"), os.Getenv("RELAY_RELEASE")
	if tty1 == "" || relay == "" {
		t.Skip("TTY1_RELEASE and RELAY_RELEASE name no copies of the latest releases of tty1 and the relay")
	}
	seen := map[string]bool{}
	for _, name := range []string{"sh", "dash"} {
		path, err := exec.LookPath(name)
		if err != nil {
			continue
		}
		if real, err := filepath.EvalSymlinks(path); err == nil && !seen[real] {
			seen[real] = true
			t.Run(name, func(t *testing.T) { installerCases(t, path, tty1, relay) })
		}
	}
	if len(seen) == 0 {
		t.Fatal("wanted a sh to run the installer with")
	}
}

func installerCases(t *testing.T, shell, tty1, relay string) {
	t.Run("it installs, and installs again", func(t *testing.T) {
		tr := newTrial(t, shell, tty1)
		for range 2 {
			status, out, errs := tr.run()
			if status != 0 || !strings.Contains(out, "tty1 "+tr.version+" is installed, at "+tr.self()+".") {
				t.Fatalf("wanted tty1 %s installed, got %d: %q %q", tr.version, status, out, errs)
			}
			tr.installed()
		}
	})

	t.Run("a manifest with one byte changed", func(t *testing.T) {
		tr := newTrial(t, shell, tty1)
		before := tr.installFirst()
		manifest := filepath.Join(tr.latest(), "manifest.json")
		b, err := os.ReadFile(manifest)
		if err != nil {
			t.Fatal(err)
		}
		b[len(b)/2] ^= 1
		write(t, manifest, b, 0o644)
		tr.refused("The release's signature is not the studio's")
		if !bytes.Equal(tr.installed(), before) {
			t.Error("wanted the tty1 installed before left as it was")
		}
	})

	t.Run("an archive other than the one signed for, that would leave a mark if run", func(t *testing.T) {
		tr := newTrial(t, shell, tty1)
		before := tr.installFirst()
		mark := filepath.Join(t.TempDir(), "ran")
		write(t, tr.archive(), gzipped(t, "#!/bin/sh\ntouch '"+mark+"'\necho tty1 v9.9.9\n"), 0o644)
		tr.refused("The archive downloaded is not the one the studio signed for")
		if _, err := os.Stat(mark); !errors.Is(err, os.ErrNotExist) {
			t.Error("wanted nothing of the archive run, and it was")
		}
		if !bytes.Equal(tr.installed(), before) {
			t.Error("wanted the tty1 installed before left as it was")
		}
	})

	t.Run("the relay's release, which the studio signed, in the place of tty1's", func(t *testing.T) {
		tr := newTrial(t, shell, relay)
		tr.refused("not as a release of tty1")
		tr.nothing()
	})

	t.Run("an ssh-keygen that cannot check signatures", func(t *testing.T) {
		tr := newTrial(t, shell, tty1)
		tr.path = tool(t, "ssh-keygen", "echo 'unknown option -- Y' >&2\nexit 1\n") + ":" + tr.path
		if errs := tr.refused("this ssh-keygen cannot check it"); !strings.Contains(errs, "unknown option -- Y") {
			t.Errorf("wanted what ssh-keygen said shown, got %q", errs)
		}
		tr.nothing()
	})

	for _, m := range []struct{ system, processor, says string }{
		{"FreeBSD", "amd64", "There is no build of tty1 for FreeBSD."},
		{"Linux", "riscv64", "There is no build of tty1 for riscv64 processors."},
	} {
		t.Run("a machine there is no build for: "+m.system+" on "+m.processor, func(t *testing.T) {
			tr := newTrial(t, shell, tty1)
			tr.path = tool(t, "uname", "case $1 in -s) echo "+m.system+" ;; -m) echo "+m.processor+" ;; esac\n") + ":" + tr.path
			tr.refused(m.says)
			tr.nothing()
		})
	}

	bash := ".bashrc"
	if runtime.GOOS == "darwin" {
		bash = ".bash_profile"
	}
	for _, s := range []struct{ shell, file, line string }{
		{"/bin/zsh", ".zshrc", `export PATH="$HOME/.local/bin:$PATH"`},
		{"/bin/bash", bash, `export PATH="$HOME/.local/bin:$PATH"`},
		{"/usr/local/bin/fish", filepath.Join(".config", "fish", "config.fish"), "fish_add_path ~/.local/bin"},
		{"/bin/sh", ".profile", `export PATH="$HOME/.local/bin:$PATH"`},
	} {
		t.Run("not on the PATH, in the shell "+s.shell, func(t *testing.T) {
			tr := newTrial(t, shell, tty1)
			tr.shellVar = s.shell
			_, out, _ := tr.run()
			want := filepath.Join(tr.home, ".local", "bin") + " is not on your PATH yet. Add this line to " + filepath.Join(tr.home, s.file)
			if !strings.Contains(out, want) || !strings.Contains(out, "\n    "+s.line+"\n") {
				t.Errorf("wanted to be told to add %s to %s, got %q", s.line, s.file, out)
			}
			tr.installed()
		})
	}

	t.Run("on the PATH", func(t *testing.T) {
		tr := newTrial(t, shell, tty1)
		tr.path = filepath.Join(tr.home, ".local", "bin") + ":" + tr.path
		if _, out, _ := tr.run(); strings.Contains(out, "on your PATH") || !strings.Contains(out, "Start it with: tty1") {
			t.Errorf("wanted no word about the PATH, got %q", out)
		}
	})

	t.Run("on the PATH, after another tty1", func(t *testing.T) {
		tr := newTrial(t, shell, tty1)
		other := tool(t, "tty1", "echo tty1 dev\n")
		tr.path = other + ":" + filepath.Join(tr.home, ".local", "bin") + ":" + tr.path
		if _, out, _ := tr.run(); !strings.Contains(out, "Another tty1 comes first on your PATH, at "+filepath.Join(other, "tty1")+".") {
			t.Errorf("wanted the other tty1 named, got %q", out)
		}
	})
}

// trial is one player's machine: a home of its own, a temporary folder that has to be empty
// again after each run, and a folder laid out as GitHub's addresses for tty1's releases are.
type trial struct {
	t               *testing.T
	shell           string
	home, tmp, tree string
	path, shellVar  string
	version         string
}

// newTrial serves the release in the folder given as the newest of tty1's: its manifest and
// signature where the latest are asked for, and its archive for this machine under its version.
func newTrial(t *testing.T, shell, release string) *trial {
	t.Helper()
	tr := &trial{t: t, shell: shell, home: t.TempDir(), tmp: t.TempDir(), tree: t.TempDir(), path: os.Getenv("PATH"), shellVar: "/bin/zsh"}
	manifest, err := os.ReadFile(filepath.Join(release, "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var m struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.Unmarshal(manifest, &m); err != nil || m.Version == "" {
		t.Fatalf("%s: wanted a manifest with a version, got %v", release, err)
	}
	tr.version = m.Version
	for _, name := range []string{"manifest.json", "manifest.json.sig"} {
		write(t, filepath.Join(tr.latest(), name), readFile(t, filepath.Join(release, name)), 0o644)
	}
	// The relay's release is refused before any archive is asked for, so it needs none here.
	if m.Name == "tty1" {
		write(t, tr.archive(), readFile(t, filepath.Join(release, "tty1_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz")), 0o644)
	}
	return tr
}

func (tr *trial) latest() string {
	return filepath.Join(tr.tree, "tty1", "releases", "latest", "download")
}
func (tr *trial) archive() string {
	return filepath.Join(tr.tree, "tty1", "releases", "download", tr.version, "tty1_"+runtime.GOOS+"_"+runtime.GOARCH+".tar.gz")
}
func (tr *trial) self() string { return filepath.Join(tr.home, ".local", "bin", "tty1") }

// run pipes the installer to the trial's sh, as `curl -fsSL https://usr.games/install.sh | sh`
// does, with nothing of this machine's own environment but its PATH.
func (tr *trial) run() (status int, out, errs string) {
	tr.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, tr.shell)
	cmd.Stdin = bytes.NewReader(readFile(tr.t, filepath.Join("..", "..", "site", "install.sh")))
	cmd.Env = []string{"HOME=" + tr.home, "TMPDIR=" + tr.tmp, "PATH=" + tr.path, "SHELL=" + tr.shellVar, "TTY1_RELEASES=file://" + tr.tree}
	var o, e strings.Builder
	cmd.Stdout, cmd.Stderr = &o, &e
	var exit *exec.ExitError
	switch err := cmd.Run(); {
	case errors.As(err, &exit):
		status = exit.ExitCode()
	case err != nil:
		tr.t.Fatal(err)
	}
	if left, _ := os.ReadDir(tr.tmp); len(left) != 0 {
		tr.t.Errorf("wanted the temporary folder empty after the run, and %d entries are left in it", len(left))
	}
	return status, o.String(), e.String()
}

// installFirst installs the release as it is, and returns the tty1 it installed.
func (tr *trial) installFirst() []byte {
	tr.t.Helper()
	if status, out, errs := tr.run(); status != 0 {
		tr.t.Fatalf("wanted tty1 installed first, got %d: %q %q", status, out, errs)
	}
	return tr.installed()
}

// refused holds that the run stops with what it says and does not say tty1 is installed. It
// returns what it said.
func (tr *trial) refused(says string) string {
	tr.t.Helper()
	status, out, errs := tr.run()
	if status != 1 || out != "" || !strings.Contains(errs, says) {
		tr.t.Errorf("wanted status 1 and to be told %q, got %d: %q %q", says, status, out, errs)
	}
	return errs
}

// installed holds that the release's tty1 is installed, runs, and is alone in its folder, and
// returns it.
func (tr *trial) installed() []byte {
	tr.t.Helper()
	info, err := os.Lstat(tr.self())
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != 0o755 {
		tr.t.Fatalf("wanted tty1 a plain program of mode 755, got %v, %v", info, err)
	}
	if said, err := exec.Command(tr.self(), "--version").Output(); err != nil || string(said) != "tty1 "+tr.version+"\n" {
		tr.t.Errorf("wanted tty1 to say it is %s, got %q, %v", tr.version, said, err)
	}
	if left, _ := os.ReadDir(filepath.Dir(tr.self())); len(left) != 1 {
		tr.t.Errorf("wanted tty1 alone in its folder, got %d entries", len(left))
	}
	return readFile(tr.t, tr.self())
}

// nothing holds that the run wrote nothing in the home.
func (tr *trial) nothing() {
	tr.t.Helper()
	if left, _ := os.ReadDir(tr.home); len(left) != 0 {
		tr.t.Errorf("wanted nothing written in the home, and %d entries are in it", len(left))
	}
}

// tool puts a program of the given name in a folder of its own, for the front of a PATH.
func tool(t *testing.T, name, script string) string {
	t.Helper()
	dir := t.TempDir()
	write(t, filepath.Join(dir, name), []byte("#!/bin/sh\n"+script), 0o755)
	return dir
}

// gzipped is an archive of the shape a release's is, holding a tty1 of the given text.
func gzipped(t *testing.T, program string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: "./tty1", Mode: 0o755, Size: int64(len(program)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write([]byte(program)); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func write(t *testing.T, path string, b []byte, mode os.FileMode) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, b, mode); err != nil {
		t.Fatal(err)
	}
}
