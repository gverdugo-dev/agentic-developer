#!/bin/sh
# adev installer for macOS and Linux.
# Downloads the latest prebuilt binary for your OS/arch from GitHub Releases and
# installs it. No Go toolchain required.
#
#   curl -fsSL https://raw.githubusercontent.com/gverdugo-dev/agentic-developer/main/install.sh | sh
#
# Override the install dir with ADEV_INSTALL_DIR=/path sh install.sh
set -eu

REPO="gverdugo-dev/agentic-developer"
BIN="adev"

# --- detect platform -------------------------------------------------------
os="$(uname -s)"
case "$os" in
	Linux) os="linux" ;;
	Darwin) os="darwin" ;;
	*) echo "adev: unsupported OS '$os'. See $REPO releases for manual install." >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
	x86_64 | amd64) arch="amd64" ;;
	arm64 | aarch64) arch="arm64" ;;
	*) echo "adev: unsupported architecture '$arch'." >&2; exit 1 ;;
esac

asset="adev_${os}_${arch}.tar.gz"
base="https://github.com/${REPO}/releases/latest/download"

# --- choose install dir ----------------------------------------------------
dir="${ADEV_INSTALL_DIR:-}"
if [ -z "$dir" ]; then
	if [ -w /usr/local/bin ]; then
		dir="/usr/local/bin"
	else
		dir="$HOME/.local/bin"
	fi
fi
mkdir -p "$dir"

# --- download --------------------------------------------------------------
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

fetch() {
	# fetch <url> <out>
	if command -v curl >/dev/null 2>&1; then
		curl -fsSL "$1" -o "$2"
	elif command -v wget >/dev/null 2>&1; then
		wget -qO "$2" "$1"
	else
		echo "adev: need curl or wget to download." >&2
		exit 1
	fi
}

echo "adev: downloading $asset ..." >&2
fetch "$base/$asset" "$tmp/$asset"

# --- verify checksum (best effort) -----------------------------------------
shacmd=""
if command -v sha256sum >/dev/null 2>&1; then
	shacmd="sha256sum"
elif command -v shasum >/dev/null 2>&1; then
	shacmd="shasum -a 256"
fi
if [ -n "$shacmd" ] && fetch "$base/checksums.txt" "$tmp/checksums.txt" 2>/dev/null; then
	expected="$(grep " $asset\$" "$tmp/checksums.txt" 2>/dev/null | awk '{print $1}')"
	if [ -n "$expected" ]; then
		actual="$(cd "$tmp" && $shacmd "$asset" | awk '{print $1}')"
		if [ "$expected" != "$actual" ]; then
			echo "adev: checksum mismatch for $asset, aborting." >&2
			exit 1
		fi
	fi
fi

# --- install ---------------------------------------------------------------
tar -xzf "$tmp/$asset" -C "$tmp"
chmod +x "$tmp/$BIN"
mv "$tmp/$BIN" "$dir/$BIN"

echo "adev: installed to $dir/$BIN" >&2

case ":$PATH:" in
	*":$dir:"*) ;;
	*) echo "adev: NOTE: $dir is not on your PATH. Add it, e.g.:  export PATH=\"$dir:\$PATH\"" >&2 ;;
esac

echo "adev: done. Run 'adev setup' to install adev's skills into your harness." >&2
