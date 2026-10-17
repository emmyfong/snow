#!/bin/sh
# Install Snow on Linux or WSL:
#
#   curl -fsSL https://raw.githubusercontent.com/emmyfong/snow/main/install.sh | sh
#
# It downloads the newest release for this machine, checks it against the
# release's checksums, and installs snow to ~/.local/bin. Settings:
#
#   SNOW_VERSION      a tag such as v0.1.0; default: the newest release
#   SNOW_INSTALL_DIR  where snow goes; default: ~/.local/bin
#   SNOW_BASE_URL     where the release files are; for tests and mirrors
set -eu

fail() {
	echo "snow install: $*" >&2
	exit 1
}

need() {
	command -v "$1" >/dev/null 2>&1 || fail "$1 is needed but not installed"
}

need curl
need tar
need uname

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) fail "this script installs on Linux and WSL; on Windows use Scoop or the zip" ;;
esac

case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
aarch64 | arm64) arch=arm64 ;;
*) fail "no release for $(uname -m)" ;;
esac

if [ -n "${SNOW_BASE_URL:-}" ]; then
	base=$SNOW_BASE_URL
elif [ -n "${SNOW_VERSION:-}" ]; then
	base=https://github.com/emmyfong/snow/releases/download/$SNOW_VERSION
else
	base=https://github.com/emmyfong/snow/releases/latest/download
fi
dest=${SNOW_INSTALL_DIR:-$HOME/.local/bin}
archive=snow_${os}_${arch}.tar.gz

# The temp folder sits beside the destination, so the final move is a
# rename in one folder: an interrupted install never leaves half a binary.
mkdir -p "$dest"
tmp=$(mktemp -d "$dest/.snow-install.XXXXXX")
trap 'rm -rf "$tmp"' EXIT

# HTTPS only, so a redirect cannot downgrade the download; file:// is for
# tests and local mirrors.
fetch() {
	curl -fsSL --proto '=https,file' --proto-redir '=https' --tlsv1.2 "$1" -o "$2"
}

echo "Downloading $archive"
fetch "$base/$archive" "$tmp/$archive" || fail "could not download $base/$archive"
fetch "$base/checksums.txt" "$tmp/checksums.txt" || fail "could not download $base/checksums.txt"

want=$(awk -v f="$archive" '$2 == f { print $1 }' "$tmp/checksums.txt")
[ -n "$want" ] || fail "checksums.txt has no entry for $archive"
if command -v sha256sum >/dev/null 2>&1; then
	got=$(sha256sum "$tmp/$archive" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	got=$(shasum -a 256 "$tmp/$archive" | awk '{ print $1 }')
else
	fail "sha256sum or shasum is needed to check the download"
fi
[ "$got" = "$want" ] || fail "checksum mismatch for $archive: the download is damaged or was changed; nothing was installed"

tar -xzf "$tmp/$archive" -C "$tmp" snow || fail "the archive has no snow binary"
chmod 755 "$tmp/snow"
mv "$tmp/snow" "$dest/snow"

echo "Installed $("$dest/snow" version) to $dest/snow"
case ":$PATH:" in
*":$dest:"*) ;;
*) echo "Add $dest to your PATH to run snow from any folder." ;;
esac
