#!/bin/sh

set -eu

DEFAULT_VERSION=0.1.0
REPOSITORY=Yacobolo/toolbelt

usage() {
	cat <<'EOF'
Install Sourcebook from a GitHub release.

Usage:
  install.sh [--version <version>] [--bin-dir <directory>]

Environment:
  SOURCEBOOK_VERSION                   Release version (default: 0.1.0)
  SOURCEBOOK_INSTALL_DIR               Install directory (default: ~/.local/bin)
  SOURCEBOOK_RELEASE_BASE_URL          Override release download base URL
EOF
}

fail() {
	printf 'sourcebook installer: %s\n' "$1" >&2
	exit 1
}

version=${SOURCEBOOK_VERSION:-$DEFAULT_VERSION}
if [ -n "${SOURCEBOOK_INSTALL_DIR:-}" ]; then
	bin_dir=$SOURCEBOOK_INSTALL_DIR
elif [ -n "${HOME:-}" ]; then
	bin_dir=$HOME/.local/bin
else
	bin_dir=
fi

while [ "$#" -gt 0 ]; do
	case "$1" in
	--version)
		[ "$#" -ge 2 ] || fail "--version requires a value"
		version=$2
		shift 2
		;;
	--bin-dir)
		[ "$#" -ge 2 ] || fail "--bin-dir requires a value"
		bin_dir=$2
		shift 2
		;;
	-h | --help)
		usage
		exit 0
		;;
	*)
		fail "unknown argument: $1"
		;;
	esac
done

case "$version" in
sourcebook/v*) version=${version#sourcebook/v} ;;
v*) version=${version#v} ;;
esac
case "$version" in
"" | *[!0-9A-Za-z._-]*) fail "invalid version: $version" ;;
esac
[ -n "$bin_dir" ] || fail "install directory is empty; set HOME or SOURCEBOOK_INSTALL_DIR"

for command in curl tar awk mktemp; do
	command -v "$command" >/dev/null 2>&1 || fail "required command not found: $command"
done

target_os=${SOURCEBOOK_OS:-}
if [ -z "$target_os" ]; then
	case "$(uname -s)" in
	Darwin) target_os=darwin ;;
	Linux) target_os=linux ;;
	*) fail "unsupported operating system: $(uname -s)" ;;
	esac
fi
case "$target_os" in
darwin | linux) ;;
*) fail "unsupported operating system: $target_os" ;;
esac

target_arch=${SOURCEBOOK_ARCH:-}
if [ -z "$target_arch" ]; then
	case "$(uname -m)" in
	x86_64 | amd64) target_arch=amd64 ;;
	arm64 | aarch64) target_arch=arm64 ;;
	*) fail "unsupported architecture: $(uname -m)" ;;
	esac
fi
case "$target_arch" in
amd64 | arm64) ;;
*) fail "unsupported architecture: $target_arch" ;;
esac

release_base_url=${SOURCEBOOK_RELEASE_BASE_URL:-https://github.com/$REPOSITORY/releases/download}
release_base_url=${release_base_url%/}
asset=sourcebook_${version}_${target_os}_${target_arch}.tar.gz
release_url=$release_base_url/sourcebook/v${version}

temporary_dir=$(mktemp -d "${TMPDIR:-/tmp}/sourcebook-install.XXXXXX")
staged_binary=
cleanup() {
	if [ -n "$staged_binary" ]; then
		rm -f "$staged_binary"
	fi
	rm -rf "$temporary_dir"
}
trap cleanup EXIT HUP INT TERM

archive=$temporary_dir/$asset
checksums=$temporary_dir/checksums.txt
curl -fsSL --retry 3 --retry-delay 1 -o "$archive" "$release_url/$asset" || fail "download failed: $asset"
curl -fsSL --retry 3 --retry-delay 1 -o "$checksums" "$release_url/checksums.txt" || fail "download failed: checksums.txt"

expected_checksum=$(awk -v asset="$asset" '$2 == asset || $2 == "*" asset { print $1; exit }' "$checksums")
[ -n "$expected_checksum" ] || fail "checksum not found for $asset"
if command -v sha256sum >/dev/null 2>&1; then
	actual_checksum=$(sha256sum "$archive" | awk '{ print $1 }')
elif command -v shasum >/dev/null 2>&1; then
	actual_checksum=$(shasum -a 256 "$archive" | awk '{ print $1 }')
else
	fail "sha256sum or shasum is required to verify the download"
fi
[ "$actual_checksum" = "$expected_checksum" ] || fail "checksum verification failed for $asset"

extracted_dir=$temporary_dir/extracted
mkdir -p "$extracted_dir"
tar -xzf "$archive" -C "$extracted_dir"
[ -f "$extracted_dir/sourcebook" ] || fail "archive does not contain the sourcebook binary"

mkdir -p "$bin_dir" || fail "cannot create install directory: $bin_dir"
[ -w "$bin_dir" ] || fail "install directory is not writable: $bin_dir"
staged_binary=$bin_dir/.sourcebook.tmp.$$
cp "$extracted_dir/sourcebook" "$staged_binary"
chmod 0755 "$staged_binary"
mv -f "$staged_binary" "$bin_dir/sourcebook"
staged_binary=

printf 'Sourcebook v%s installed to %s/sourcebook\n' "$version" "$bin_dir"
case ":${PATH:-}:" in
*":$bin_dir:"*) ;;
*) printf 'Add %s to PATH to run sourcebook from any directory.\n' "$bin_dir" >&2 ;;
esac
