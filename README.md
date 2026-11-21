# usr.games

The site of usr.games, an arcade in your terminal. It is plain files in `site/`, served by Cloudflare, and it also answers the `go` command for `usr.games/<name>`.

## What is here

| Path | What |
|---|---|
| `games.json` | The one list of games. The pages, the drawn names and the import tags are all made from it. |
| `site/` | What is served. The pages and the drawings in it are made files: do not edit them by hand. |
| `site/style.css`, `site/floor.js` | The stylesheet and the site's one script, written by hand. |
| `internal/brand` | Draws the sign, the names, the icons and the link-preview card. |
| `internal/sitepages` | Writes the pages from `games.json` and its templates. |
| `internal/sitecheck` | Tests of the finished site: tags, links, nothing loaded from elsewhere. |
| `BRAND.md` | The brand's facts. |

## Working on it

```sh
make site    # draw the files and write the pages again
make check   # format, tests, vet: fails if a made file has drifted
```

To add a game: add its entry to `games.json`, add a sketch to `internal/brand/sketch.go` until its recording exists, run `make site`, and commit what it wrote.

## Deploys

Cloudflare builds the site from this repository.

- **A pull request gets a preview.** Cloudflare comments on it with the address.
- **A merge to `main` deploys.** Until usr.games is attached, that deployment has no address: `wrangler.jsonc` turns the Worker's own address off, and a test holds it off.
- **From opening day, a merge to `main` is a publication.**

## How `go install usr.games/<name>` finds its code

Every page, the not-found page included, carries one `go-import` tag for each module, so whichever address the `go` command asks, it reads where the code lives. A game's page is a flat file (`site/rustwake.html`), so that `usr.games/rustwake?go-get=1` answers with the page itself and not with a redirect.

## License

MIT. See [LICENSE](LICENSE). The typeface in `site/media/fonts` is under the SIL Open Font License; its text is beside it.
