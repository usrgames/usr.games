#!/bin/sh
# Pins a release of one of the studio's programs on the site: its manual page, which the site
# shows at /man/NAME, a game's recording, NAME.gif and its still NAME.png, which its screens
# show, and a game's build for a web page, NAME.wasm.gz and its wasm_exec.js, which its page
# plays. Run it after a release, from anywhere in the repository:
#
#   scripts/pin.sh rustwake v0.1.0
#
# It downloads the release's manifest and signature from GitHub and checks the signature with
# ssh-keygen against site/etc/allowed_signers, then that the manifest is NAME's at VERSION, then
# each file against the manifest. Only then does it put the page in man/, the recording in
# site/media/NAME/, the build in site/play/NAME/VERSION/ and their hashes in games.lock.json, in
# place of what was pinned for NAME before. On any failure it writes nothing. Then `make site`
# writes the pages again.
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

# sum prints the hash the manifest gives a file, or nothing if it does not list the file once.
sum() {
	jq -r --arg f "$1" '[.files[] | select(.name == $f) | .sha256] | if length == 1 then .[0] else empty end' "$tmp/manifest.json"
}

# where prints where the site keeps a file of a release at a version: a page in man/, which its
# page on the site is made from, a recording where the site serves it, and a build at an address
# that carries its version, so that a browser may keep it as long as it likes.
where() { # file, version
	case $1 in
	*.html) printf 'man/%s\n' "$1" ;;
	"$name.wasm.gz" | wasm_exec.js) printf 'site/play/%s/%s/%s\n' "$name" "$2" "$1" ;;
	*) printf 'site/media/%s/%s\n' "$name" "$1" ;;
	esac
}

case "$(sum "$name.gif"):$(sum "$name.png")" in
:) files=$page ;;
?*:?*) files="$page $name.gif $name.png" ;;
*) fail "$name $version holds a recording without its still, or a still without its recording, so nothing was pinned." ;;
esac
case "$(sum "$name.wasm.gz"):$(sum wasm_exec.js)" in
:) ;;
?*:?*) files="$files $name.wasm.gz wasm_exec.js" ;;
*) fail "$name $version holds a build for a web page without its wasm_exec.js, or the other way round, so nothing was pinned." ;;
esac

entry='{}'
for f in $files; do
	want=$(sum "$f")
	printf '%s\n' "$want" | grep -Eq '^[0-9a-f]{64}$' || fail "The manifest gives $f no hash, so nothing was pinned."
	get "$at/$f" "$tmp/$f"
	[ "$(sha256 "$tmp/$f")" = "$want" ] ||
		fail "The $f downloaded is not the one the studio signed for, so nothing was pinned."
	entry=$(printf '%s' "$entry" | jq -c --arg f "$f" --arg h "$want" '. + {($f): $h}')
done

jq -S --arg n "$name" --arg v "$version" --argjson files "$entry" \
	'.[$n] = {version: $v, files: $files}' games.lock.json > "$tmp/lock" ||
	fail "games.lock.json could not be read, so nothing was pinned."

# What was pinned for this program before goes, so that man/, site/media/ and site/play/ hold
# what the lock names and no more.
old=$(jq -r --arg n "$name" '.[$n].files // {} | keys[]' games.lock.json)
was=$(jq -r --arg n "$name" '.[$n].version // empty' games.lock.json)
for f in $old; do
	case $f in */* | .*) fail "games.lock.json names $f, which is not a file the site keeps, so nothing was pinned." ;; esac
done
case $was in
'' | *[!v0-9.]* | .*) [ -z "$old" ] || fail "games.lock.json gives $name the version \"$was\", which is not one, so nothing was pinned." ;;
esac
for f in $old; do
	rm -f "$(where "$f" "$was")"
done
[ -z "$was" ] || rmdir "site/play/$name/$was" 2> /dev/null || true
for f in $files; do
	mkdir -p "$(dirname "$(where "$f" "$version")")"
	cp "$tmp/$f" "$(where "$f" "$version")"
done
cp "$tmp/lock" games.lock.json
printf '%s\n' "Pinned $name $version: $(for f in $files; do where "$f" "$version"; done | tr '\n' ' ')" "Now run make site, and commit what it wrote."
