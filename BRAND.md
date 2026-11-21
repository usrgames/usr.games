# The usr.games brand

usr.games is an arcade in your terminal. Its look is one green tube: a single phosphor screen, the kind a terminal had.

This file owns the brand's facts. The drawings are made by `internal/brand`, and `make check` fails if a file, a colour here, or the stylesheet has drifted from that source.

## The sign

`#!/usr/games`

A shebang line that points at the Unix games directory. It is drawn in the studio's own letters: five cells wide, seven tall, built only from cells, so a terminal can print the same sign. Each cell trails a faint smear to its left, the way phosphor keeps glowing after the beam has moved on.

- Always this line, in these letters, in phosphor on the glass.
- No other colour, no tilt, no outline, no letters from a typeface standing in for it.
- The games are written the way the Unix manual writes them, with their section: `rustwake(6)`.

## The small mark

The letter `u` from the same letters, on the glass. It is what sits in a browser tab and on an avatar. At 16 pixels each of its cells is two whole pixels.

## Colours

One colour, and the four voices a terminal gives it.

| Voice | Hex | Used for |
|---|---|---|
| Glass | `#030a05` | The screen. |
| Plain | `#45ff86` | Words, the sign, borders. |
| Bright | `#d6ffe4` | Headings and numbers worth looking at. |
| Dim | `#2fd16a` | What can be read second: prompts, notes, captions. |
| Reversed | glass on plain | What is yours, and what you can press. |

A blink asks for a key: four blinks, then it holds still. Nothing else moves by itself.

Every voice is readable on the glass at 4.5 to 1 or better; a test holds that.

## Type

Words are set in VT323, which redraws the letters of a DEC terminal. The file is `site/media/fonts/VT323-Regular.ttf`, served from the site, with its licence beside it (SIL Open Font License 1.1, Copyright 2011 The VT323 Project Authors).

## Voice

Playful and inviting, as an arcade is. The Unix words (`ls`, `who`, `/var`) are props on the stage. Never a manual's deadpan, and never jargon for its own sake.

Lines in use:

- Under the sign: "An arcade in your terminal. Free play." It carries no number, so it never changes when a game is added.
- "Play it here, or take it home."
- "See also"

## How a game is shown

- **A row of the listing.** The front page is `ls -l /usr/games`. Each game is a row; the row you are on is reversed. A column says how many can play: `1p`, `2p`, `2p net`.
- **An attract screen.** Beside the listing, one screen shows the game you are on: its picture, its name, what it is, and an invitation to play.

The listing grows downward and nothing is redesigned. The line under it counts the games. From eight games up it is meant to gain three controls that rewrite the command (A to Z, newest first, only games against someone); those are not built yet.

## Files

| File | What |
|---|---|
| `site/sign.svg` | The sign. |
| `site/media/<name>/name.svg` | A game's name, or a tool's, in the letters. |
| `site/favicon.svg`, `site/favicon.ico`, `site/apple-touch-icon.png`, `site/icon-512.png` | The small mark, for browsers, phones and avatars. |
| `site/card.png` | The picture beside a link to usr.games: the sign and its line, 1280 by 640. |
| `site/media/<game>/sketch.svg` | A stand-in picture of a game. See below. |

Draw them again with `make site`.

**The sketches are placeholders.** A game's screen is meant to show a recording of the game itself. Until each game's release brings one, a small sketch in cells keeps the screen from being empty.
