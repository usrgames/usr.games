#!/bin/sh
# Keeps glonk's player on the site, as glonk's tag VERSION has it: glonk.js, the page's half of
# glonk, which starts a game and hands it the keys, and xterm.js, its WebGL addon and its
# stylesheet, which draw the terminal, with their licences. A game's page plays with them. Run it
# from anywhere in the repository:
#
#   scripts/player.sh v0.16.2
#
# The files go in site/play/glonk/VERSION/, in place of any other version, and are downloaded
# from GitHub over https at that tag. glonk.js and the game talk to each other, so a game built
# with a glonk whose page protocol changed needs the player of that glonk: move both together.
#
# PLAYER_SOURCE points it elsewhere, for its test.

set -eu

fail() {
	printf '%s\n' "$@" >&2
	exit 1
}

[ $# -eq 1 ] || fail "Usage: scripts/player.sh VERSION, such as: scripts/player.sh v0.16.2"
version=$1
case $version in
'' | *[!v0-9.]*) fail "\"$version\" is not a version such as v0.16.2." ;;
esac
printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' ||
	fail "\"$version\" is not a version such as v0.16.2."
command -v curl > /dev/null 2>&1 || fail "Keeping the player takes curl, and this machine has none."

cd "$(dirname "$0")/.."
[ -f games.lock.json ] && [ -d site ] || fail "$(pwd) is not the site's repository."

from=${PLAYER_SOURCE:-https://raw.githubusercontent.com/usrgames/glonk}
at="${from%/}/$version"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/player.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
trap 'exit 1' HUP INT TERM

for f in web/glonk.js site/media/xterm/xterm.js site/media/xterm/addon-webgl.js site/media/xterm/xterm.css \
	site/media/xterm/LICENSE-xterm.txt site/media/xterm/LICENSE-addon-webgl.txt; do
	curl -fsSL --proto '=https,file' --proto-redir '=https' -o "$tmp/${f##*/}" "$at/$f" ||
		fail "$at/$f could not be downloaded, so the player was left as it was."
	[ -s "$tmp/${f##*/}" ] || fail "$at/$f is empty, so the player was left as it was."
done

rm -rf site/play/glonk
mkdir -p "site/play/glonk/$version"
cp "$tmp"/* "site/play/glonk/$version/"
printf '%s\n' "Kept glonk's player at $version: site/play/glonk/$version/." "Now run make site, and commit what it wrote."
