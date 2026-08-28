#!/bin/sh
# Install bible-cli from GitHub Releases into ${PREFIX:-$HOME/.local/bin}.
# Never installs a binary named bible (Debian bible-kjv owns /usr/bin/bible).
set -eu

REPO="tuxr/bible-cli"
GITHUB_API="${GITHUB_API:-https://api.github.com}"
GITHUB="${GITHUB:-https://github.com}"

usage() {
	cat <<EOF
Usage: install.sh [--prefix DIR] [--version vX.Y.Z] [--uninstall]

Install the latest (or --version) bible-cli GitHub Release asset for this
OS/arch into PREFIX (default: \$HOME/.local/bin). Verifies SHA-256 against
checksums.txt.

  --prefix DIR      install prefix (also PREFIX env)
  --version vX.Y.Z  release tag (default: latest)
  --uninstall       remove \$PREFIX/bible-cli only
EOF
}

VERSION=""
UNINSTALL=0
PREFIX="${PREFIX:-}"

while [ $# -gt 0 ]; do
	case "$1" in
	--prefix)
		[ $# -ge 2 ] || { echo "install.sh: --prefix needs a directory" >&2; exit 1; }
		PREFIX="$2"
		shift 2
		;;
	--prefix=*)
		PREFIX="${1#--prefix=}"
		shift
		;;
	--version)
		[ $# -ge 2 ] || { echo "install.sh: --version needs a tag" >&2; exit 1; }
		VERSION="$2"
		shift 2
		;;
	--version=*)
		VERSION="${1#--version=}"
		shift
		;;
	--uninstall)
		UNINSTALL=1
		shift
		;;
	-h|--help)
		usage
		exit 0
		;;
	*)
		echo "install.sh: unknown option: $1" >&2
		usage >&2
		exit 1
		;;
	esac
done

if [ -z "${PREFIX}" ]; then
	PREFIX="${HOME}/.local/bin"
fi

BIN="${PREFIX}/bible-cli"

die() {
	echo "install.sh: $*" >&2
	exit 1
}

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "need $1"
}

sha256_of() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$1" | awk '{print $1}'
	elif command -v shasum >/dev/null 2>&1; then
		shasum -a 256 "$1" | awk '{print $1}'
	else
		die "need sha256sum or shasum"
	fi
}

detect_os() {
	os=$(uname -s | tr '[:upper:]' '[:lower:]')
	case "$os" in
	linux|darwin) printf '%s\n' "$os" ;;
	*) die "unsupported OS: $os (linux or darwin)" ;;
	esac
}

detect_arch() {
	arch=$(uname -m)
	case "$arch" in
	x86_64|amd64) printf '%s\n' "amd64" ;;
	aarch64|arm64) printf '%s\n' "arm64" ;;
	*) die "unsupported arch: $arch (amd64 or arm64)" ;;
	esac
}

uninstall() {
	[ "$(basename "$BIN")" = "bible-cli" ] || die "refusing to delete $BIN"
	[ "$(basename "$BIN")" != "bible" ] || die "refusing to delete $BIN"
	if [ -e "$BIN" ]; then
		rm -f "$BIN" || die "could not remove $BIN"
	fi
	exit 0
}

if [ "$UNINSTALL" -eq 1 ]; then
	uninstall
fi

need_cmd curl
need_cmd tar
need_cmd uname
need_cmd mktemp
need_cmd awk
need_cmd tr
need_cmd basename

OS=$(detect_os)
ARCH=$(detect_arch)

if [ -z "$VERSION" ]; then
	need_cmd sed
	api_url="${GITHUB_API}/repos/${REPO}/releases/latest"
	json=$(curl -fsSL "$api_url") || die "failed to fetch $api_url"
	VERSION=$(printf '%s\n' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | sed -n '1p')
	[ -n "$VERSION" ] || die "could not parse latest tag_name"
fi

case "$VERSION" in
v*) ;;
*) die "version must look like vX.Y.Z (got $VERSION)" ;;
esac

ver="${VERSION#v}"
asset="bible-cli_${ver}_${OS}_${ARCH}.tar.gz"
base="${GITHUB}/${REPO}/releases/download/${VERSION}"
archive_url="${base}/${asset}"
sums_url="${base}/checksums.txt"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT INT HUP TERM

curl -fsSL "$sums_url" -o "$tmp/checksums.txt" || die "failed to download checksums.txt"
curl -fsSL "$archive_url" -o "$tmp/$asset" || die "failed to download $asset"

want=$(awk -v f="$asset" '
	NF >= 2 && ($NF == f || $NF == ("*" f)) { print $1; exit }
' "$tmp/checksums.txt")
[ -n "$want" ] || die "checksum missing for $asset"
[ "${#want}" -eq 64 ] || die "checksum malformed for $asset"

got=$(sha256_of "$tmp/$asset")
[ "$got" = "$want" ] || die "checksum mismatch for $asset"

tar -xzf "$tmp/$asset" -C "$tmp" || die "failed to extract $asset"

src="$tmp/bible-cli"
[ -f "$src" ] || die "archive missing bible-cli"
[ "$(basename "$src")" != "bible" ] || die "refusing to install bible"

mkdir -p "$PREFIX" || die "could not create $PREFIX"
# Never write a file named bible. Only bible-cli, even if PREFIX is /usr/bin.
[ "$(basename "$BIN")" = "bible-cli" ] || die "refusing to write $BIN"

cp "$src" "$BIN" || die "could not install $BIN"
chmod 0755 "$BIN" || die "could not chmod $BIN"

printf 'installed %s\n' "$BIN"
