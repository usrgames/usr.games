#!/bin/sh
# Pins a release of one of the studio's programs on the site: its manual page, which the site
# shows at /man/NAME. Run it after a release, from anywhere in the repository:
#
#   scripts/pin.sh rustwake v0.1.0
#
# It downloads the release's manifest and signature from GitHub and checks the signature with
# ssh-keygen against site/etc/allowed_signers, then that the manifest is NAME's at VERSION, then
# the page against the manifest. Only then does it put the page in man/ and its hash in
# games.lock.json, in place of what was pinned for NAME before. On any failure it writes
# nothing. Then `make site` writes the pages again.
#
# PIN_RELEASES points it elsewhere, for its test. Nothing changes whose signature it accepts.

set -eu

fail() {
	printf '%s\n' "$@" >&2
	exit 1
}

has() { command -v "$1" > /dev/null 2>&1; }

sha256() {
	if has sha256sum; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d ' ' -f 1
}

# get downloads an address to a file. Only https is followed, and file for the test.
get() {
	curl -fsSL --proto '=https,file' --proto-redir '=https' -o "$2" "$1" ||
		fail "$1 could not be downloaded, so nothing was pinned."
}

[ $# -eq 2 ] || fail "Usage: scripts/pin.sh NAME VERSION, such as: scripts/pin.sh rustwake v0.1.0"
name=$1
version=$2
case $name in
'' | [!a-z]* | *[!a-z0-9]*) fail "\"$name\" is not the name of one of the studio's programs." ;;
esac
case $version in
'' | *[!v0-9.]*) fail "\"$version\" is not a version such as v0.1.0." ;;
esac
printf '%s\n' "$version" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$' ||
	fail "\"$version\" is not a version such as v0.1.0."

for tool in curl ssh-keygen jq; do
	has "$tool" || fail "Pinning a release takes $tool, and this machine has none."
done
has sha256sum || has shasum || fail "Pinning a release takes sha256sum or shasum, and this machine has neither."

cd "$(dirname "$0")/.."
[ -f site/etc/allowed_signers ] && [ -f games.lock.json ] ||
	fail "$(pwd) is not the site's repository: it has no site/etc/allowed_signers or games.lock.json."

releases=${PIN_RELEASES:-https://github.com/usrgames}
at="${releases%/}/$name/releases/download/$version"

tmp=$(mktemp -d "${TMPDIR:-/tmp}/pin.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
trap 'exit 1' HUP INT TERM

get "$at/manifest.json" "$tmp/manifest.json"
get "$at/manifest.json.sig" "$tmp/manifest.json.sig"
if ! ssh-keygen -Y verify -f site/etc/allowed_signers -I release@usr.games -n release@usr.games \
	-s "$tmp/manifest.json.sig" < "$tmp/manifest.json" > "$tmp/said" 2>&1; then
	fail "The signature on the manifest of $name $version is not the studio's, so nothing was pinned. ssh-keygen said:" "$(cat "$tmp/said")"
fi
jq -e --arg n "$name" --arg v "$version" '.schema == 1 and .name == $n and .version == $v' \
	"$tmp/manifest.json" > /dev/null 2>&1 ||
	fail "The studio signed this manifest, but not as a release of $name at $version, so nothing was pinned."

page=$(jq -r --arg n "$name" '[.files[].name | select(test("^" + $n + "\\.[1-9]\\.html$"))] | if length == 1 then .[0] else empty end' "$tmp/manifest.json")
[ -n "$page" ] || fail "$name $version holds no manual page, or more than one, so nothing was pinned."
want=$(jq -r --arg p "$page" '.files[] | select(.name == $p) | .sha256' "$tmp/manifest.json")
printf '%s\n' "$want" | grep -Eq '^[0-9a-f]{64}$' || fail "The manifest gives $page no hash, so nothing was pinned."

get "$at/$page" "$tmp/$page"
[ "$(sha256 "$tmp/$page")" = "$want" ] ||
	fail "The $page downloaded is not the one the studio signed for, so nothing was pinned."

jq -S --arg n "$name" --arg v "$version" --arg p "$page" --arg h "$want" \
	'.[$n] = {version: $v, files: {($p): $h}}' games.lock.json > "$tmp/lock" ||
	fail "games.lock.json could not be read, so nothing was pinned."

# What was pinned for this program before goes, so that man/ holds what the lock names and no more.
old=$(jq -r --arg n "$name" '.[$n].files // {} | keys[]' games.lock.json)
for f in $old; do
	case $f in */* | .*) fail "games.lock.json names $f, which is not a file of man/, so nothing was pinned." ;; esac
done
for f in $old; do
	rm -f "man/$f"
done
mkdir -p man
cp "$tmp/$page" "man/$page"
cp "$tmp/lock" games.lock.json
printf '%s\n' "Pinned $name $version: man/$page. Now run make site, and commit what it wrote."
