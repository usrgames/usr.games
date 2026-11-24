# usr.games

The site of usr.games, an arcade in your terminal. It is plain files in `site/`, served by Cloudflare, and it also answers the `go` command for `usr.games/<name>`.

## What is here

| Path | What |
|---|---|
| `games.json` | The one list of games. The pages, the drawn names and the import tags are all made from it. |
| `games.lock.json` | The release of each program the site pins, with the hash of each of its files the site uses. Written by `scripts/pin.sh`. |
| `man/` | The manual page of each pinned release, byte for byte as the release ships it. The pages at `/man/<name>` are made from them. Written by `scripts/pin.sh`. |
| `site/media/<game>/<game>.gif`, `.png` | A game's recording and its still, byte for byte as its release ships them. Its screens show them. Written by `scripts/pin.sh`. |
| `site/play/<game>/<version>/` | A game's build for a web page and the `wasm_exec.js` of the Go that built it, byte for byte as its release ships them, at an address that carries the version. Written by `scripts/pin.sh`. |
| `scripts/pin.sh` | Pins a release: it checks the studio's signature on the release's manifest and each file against it, then writes the manual page, a game's recording, its build for a web page and their hashes. CI tries it against the latest real releases, and against copies spoiled in each way it must refuse. |
| `site/play/glonk/<version>/` | glonk's player, from that tag of glonk: `glonk.js`, which starts a game in a page, and xterm.js, its WebGL addon and its stylesheet, which draw it, with their licences. Written by `scripts/player.sh`. |
| `scripts/player.sh` | Keeps glonk's player at one tag. A game built with a glonk whose page protocol changed needs the player of that glonk, so move the two together. |
| `plan/` | The devlog's notes, one text file each. The page at `/plan` and its feed are made from them. |
| `site/` | What is served. The pages and the drawings in it are made files: do not edit them by hand. |
| `site/style.css`, `site/floor.js`, `site/play.js` | The stylesheet, the front page's script and the script that plays a game in its page, written by hand. `play.js` loads nothing until a visitor asks to play, and offers Play only with a fine pointer, room and WebGL2: without WebGL2, xterm.js draws with style attributes, which the content policy refuses. |
| `site/_headers` | What a browser may load on a page, sent by Cloudflare with every answer. Not served as a file. A page that plays its game has the same policy with `'wasm-unsafe-eval'`, and nothing else, more; a test holds every such page to it. |
| `internal/brand` | Draws the sign, the names, the icons and the link-preview card. |
| `internal/plan` | Reads the notes in `plan/`. |
| `internal/sitepages` | Writes the pages, and the devlog's feed, from `games.json`, the notes and its templates. |
| `internal/sitecheck` | Tests of the finished site: tags, links, nothing loaded from elsewhere. |
| `BRAND.md` | The brand's facts. |
| `KEYS.md` | What signs a release, how to check one, and how a key is renewed or replaced. |
| `site/etc/games.json` | The list of games as the launcher, `tty1`, reads it. A made file, written from `games.json`. |
| `site/etc/allowed_signers`, `site/etc/release-cert.pub` | The root's public key, and the certificate in which it vouches for the key that signs releases. A test holds the two together. |
| `site/install.sh` | The installer the front page's line pipes to `sh`. It keeps `tty1` only once the studio's signature on it checks out, with the root's key written into it; a test holds that key to `allowed_signers`. Written by hand. CI tries it on macOS and Linux against the latest release, and against copies spoiled in each way it must refuse. |

## Working on it

```sh
make site    # draw the files and write the pages again
make check   # format, tests, vet: fails if a made file has drifted
```

To add a game: add its entry to `games.json`, add a sketch to `internal/brand/sketch.go` until its recording is pinned, run `make site`, and commit what it wrote. When a game's recording is first pinned, give it a `recording` in `games.json`, what the recording shows for anyone who cannot see it; `make site` then removes its sketch.

After a release of a game or of `tty1`, pin it, so that the site shows the manual page the release ships, and a game's recording and its build for a web page:

```sh
scripts/pin.sh rustwake v0.1.0
make site
```

A game's releases come from its release repository, `usrgames/NAME.6`, since its source is private; tty1's from `usrgames/tty1`. So a game can't be pinned again until its release repository has a release. It needs `curl`, `ssh-keygen` and `jq`. Nothing checks that a pin is the newest release, so until it is run the site shows the page of the release pinned before.

## The devlog

A note is a file in `plan/` named for its day and its title, like `2026-11-21-the-sign-is-up.txt`. Its first line is the title. The paragraphs follow, a blank line apart, and one that starts with `$ ` is shown as a command. A note is plain text: it holds no links.

`make site` writes the page and the feed again. A note's file name is its address in the feed, so do not rename one after it is published.

## The notices

`/privacy` says what the site and the relay keep, and `/terms` holds the house rules. Both are in `internal/sitepages/templates/pages.tmpl`. When what is kept changes, `/privacy` changes in the same pull request, with its date.

The one address the site gives is in `site/.well-known/security.txt` and, fenced so Cloudflare does not rewrite it, on the two notices. A test holds them to the same address, and fails a month before `security.txt` expires.

Cloudflare adds its Web Analytics script to each page of usr.games at the edge. No file here names it, and `/privacy` says so.

## Deploys

Cloudflare builds the site from this repository.

- **A pull request gets a preview.** Cloudflare comments on it with the address.
- **A merge to `main` deploys.** Until usr.games is attached, that deployment has no address: `wrangler.jsonc` turns the Worker's own address off, and a test holds it off.
- **From opening day, a merge to `main` is a publication.**

The live site is checked by hand, once it answers at usr.games: `gh workflow run live.yml` runs `TestTheLiveSite` against what is served, every page's policy, the go command's answers, the not-found page and the other addresses, and then every command the pages print, on fresh Linux and macOS machines. `USRGAMES_LIVE=<an address> go test ./internal/sitecheck -run TestTheLiveSite -v` runs the test alone, against usr.games or a version's own preview.

## How `go install usr.games/<name>` finds its code

Every page, the not-found page included, carries one `go-import` tag for each module, so whichever address the `go` command asks, it reads where the code lives. A game's page is a flat file (`site/rustwake.html`), so that `usr.games/rustwake?go-get=1` answers with the page itself and not with a redirect.

## License

MIT. See [LICENSE](LICENSE). The typeface in `site/media/fonts` is under the SIL Open Font License; its text is beside it.
