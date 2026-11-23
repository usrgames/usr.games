#!/bin/sh
# The installer of tty1, the usr.games launcher:
#
#   curl -fsSL https://usr.games/install.sh | sh
#
# It downloads the newest release of tty1 from GitHub and keeps it only if the studio signed it:
# the signature of the release's manifest is checked with ssh-keygen against the key below, then
# the archive against the manifest. Only then is anything unpacked, and tty1 goes in
# ~/.local/bin. It runs nothing it downloaded until every check has passed, and edits no file of
# yours. KEYS.md in https://github.com/usrgames/usr.games says how to check a release by hand.
#
# TTY1_RELEASES points it elsewhere for trying it out, as it does tty1. Nothing changes whose
# signature it accepts.
#
# Everything is in functions and the last line calls main, so a download cut short runs nothing.

# The studio's root key, as https://usr.games/etc/allowed_signers publishes it. A test holds the
# two equal.
root='release@usr.games cert-authority,namespaces="release@usr.games" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIM1V4RRWS4toT6TmK1CmZxSfzferrpobAAyk58zOIjW7'

say() { printf '%s\n' "$@"; }

fail() {
	printf '%s\n' "$@" >&2
	exit 1
}

has() { command -v "$1" > /dev/null 2>&1; }

sha256() {
	if has sha256sum; then sha256sum "$1"; else shasum -a 256 "$1"; fi | cut -d ' ' -f 1
}

# get downloads an address to a file.
get() {
	curl -fsSL -o "$2" "$1" || fail "tty1 could not be downloaded from $1, so nothing was installed."
}

main() {
	set -u
	[ -n "${HOME:-}" ] || fail "HOME is not set, so there is nowhere to put tty1."
	releases=${TTY1_RELEASES:-https://github.com/usrgames}
	releases=${releases%/}

	system=$(uname -s)
	processor=$(uname -m)
	case $system in
	Darwin) os=darwin ;;
	Linux) os=linux ;;
	*) fail "There is no build of tty1 for $system." ;;
	esac
	case $processor in
	arm64 | aarch64) arch=arm64 ;;
	x86_64 | amd64) arch=amd64 ;;
	*) fail "There is no build of tty1 for $processor processors." ;;
	esac

	has ssh-keygen || fail "tty1 is installed only once the studio's signature on it is checked, and that takes" \
		"ssh-keygen, which comes with OpenSSH (on Debian and Ubuntu, the package openssh-client)." \
		"This machine has none, so nothing was installed."
	for tool in curl tar; do
		has "$tool" || fail "Installing tty1 takes $tool, and this machine has none."
	done
	has sha256sum || has shasum || fail "Installing tty1 takes sha256sum or shasum, and this machine has neither."

	# Named in full: given no name, mktemp on macOS ignores TMPDIR.
	tmp=$(mktemp -d "${TMPDIR:-/tmp}/tty1-install.XXXXXX") || fail "There is no room for a temporary folder, so nothing was installed."
	new=''
	trap 'rm -rf "$tmp"; [ -z "$new" ] || rm -f "$new"' EXIT
	trap 'exit 1' HUP INT TERM

	latest="$releases/tty1/releases/latest/download"
	get "$latest/manifest.json" "$tmp/manifest.json"
	get "$latest/manifest.json.sig" "$tmp/manifest.json.sig"
	say "$root" > "$tmp/allowed_signers"
	if ! said=$(ssh-keygen -Y verify -f "$tmp/allowed_signers" -I release@usr.games -n release@usr.games \
		-s "$tmp/manifest.json.sig" < "$tmp/manifest.json" 2>&1); then
		fail "The release's signature is not the studio's, or this ssh-keygen cannot check it," \
			"so nothing was installed. ssh-keygen said:" "$said"
	fi

	# The manifest is the studio's own words now, in the one shape its release workflow writes.
	# The studio signs other programs too, so it has to be tty1's.
	flat=$(tr -d ' \t\r\n' < "$tmp/manifest.json")
	case $flat in
	'{"schema":1,"name":"tty1","version":"'*) ;;
	*) fail "The release is signed by the studio, but not as a release of tty1, so nothing was installed." ;;
	esac
	version=${flat#*'"version":"'}
	version=${version%%'"'*}
	archive="tty1_${os}_${arch}.tar.gz"
	want=$(say "$flat" | sed -n 's/.*{"name":"tty1_'"${os}_${arch}"'\.tar\.gz","size":[0-9]*,"sha256":"\([0-9a-f]\{64\}\)".*/\1/p')
	[ -n "$want" ] || fail "tty1 $version has no build for $os on $arch, so nothing was installed."

	# The archive comes from the release the manifest names, not from whichever is newest now.
	get "$releases/tty1/releases/download/$version/$archive" "$tmp/$archive"
	[ "$(sha256 "$tmp/$archive")" = "$want" ] ||
		fail "The archive downloaded is not the one the studio signed for, so nothing was installed."
	mkdir "$tmp/unpacked"
	tar -xzf "$tmp/$archive" -C "$tmp/unpacked" || fail "The archive could not be unpacked, so nothing was installed."
	[ -f "$tmp/unpacked/tty1" ] || fail "The archive holds no tty1, so nothing was installed."

	# Written beside where it goes and moved over in one step: an earlier tty1 is either
	# replaced whole or left as it was.
	bin="$HOME/.local/bin"
	mkdir -p "$bin" || fail "The folder $bin could not be made, so nothing was installed."
	new=$(mktemp "$bin/.tty1-new.XXXXXX") || fail "Nothing can be written in $bin, so nothing was installed."
	if ! { cp "$tmp/unpacked/tty1" "$new" && chmod 755 "$new" && mv -f "$new" "$bin/tty1"; }; then
		fail "tty1 could not be put in $bin. An earlier tty1 there is as it was."
	fi
	new=''

	said=$("$bin/tty1" --version) || fail "tty1 is in $bin, but it does not run on this machine."
	say "$said is installed, at $bin/tty1."

	case ":$PATH:" in
	*":$bin:"*)
		first=$(command -v tty1)
		if [ "$first" != "$bin/tty1" ]; then
			say "" "Another tty1 comes first on your PATH, at $first. Remove it, or put $bin before it."
		fi
		say "" "Start it with: tty1"
		;;
	*)
		# The line is printed for the reader to add, so $HOME and $PATH stay as they are written.
		# shellcheck disable=SC2016
		line='export PATH="$HOME/.local/bin:$PATH"'
		shell=${SHELL:-sh}
		case ${shell##*/} in
		zsh) file="$HOME/.zshrc" ;;
		bash) if [ "$os" = darwin ]; then file="$HOME/.bash_profile"; else file="$HOME/.bashrc"; fi ;;
		fish) file="$HOME/.config/fish/config.fish" line='fish_add_path ~/.local/bin' ;;
		*) file="$HOME/.profile" ;;
		esac
		say "" "$bin is not on your PATH yet. Add this line to $file, then open a new terminal:" "" "    $line" "" \
			"Then start it with: tty1"
		;;
	esac
}

main "$@"
