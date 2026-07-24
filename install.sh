#!/bin/sh
# Install the kdbx binary. Usage:
#   curl -LsSf https://raw.githubusercontent.com/yarrasys/kdbx/main/install.sh | sh
# Override the destination with KDBX_INSTALL_DIR, the version with KDBX_VERSION.
set -eu

REPO="yarrasys/kdbx"
INSTALL_DIR="${KDBX_INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${KDBX_VERSION:-latest}"

os="$(uname -s)"
case "$os" in
  Linux)  goos=linux ;;
  Darwin) goos=darwin ;;
  *) echo "kdbx: unsupported OS '$os' (use the Windows release archive)" >&2; exit 1 ;;
esac

arch="$(uname -m)"
case "$arch" in
  x86_64|amd64)  goarch=amd64 ;;
  arm64|aarch64) goarch=arm64 ;;
  *) echo "kdbx: unsupported architecture '$arch'" >&2; exit 1 ;;
esac

if [ "$VERSION" = latest ]; then
  base="https://github.com/$REPO/releases/latest/download"
  tag="$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest" | sed 's#.*/tag/##')"
else
  base="https://github.com/$REPO/releases/download/$VERSION"
  tag="$VERSION"
fi

archive="kdbx_${tag#v}_${goos}_${goarch}.tar.gz"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

echo "kdbx: downloading $archive"
curl -fsSL "$base/$archive" -o "$tmp/$archive"
curl -fsSL "$base/SHA256SUMS" -o "$tmp/SHA256SUMS"

echo "kdbx: verifying checksum"
( cd "$tmp" && grep " $archive\$" SHA256SUMS | { sha256sum -c - 2>/dev/null || shasum -a 256 -c -; } ) \
  || { echo "kdbx: CHECKSUM MISMATCH — refusing to install" >&2; exit 1; }

tar -xzf "$tmp/$archive" -C "$tmp"
mkdir -p "$INSTALL_DIR"
install -m 0755 "$tmp/kdbx" "$INSTALL_DIR/kdbx"

echo "kdbx: installed $INSTALL_DIR/kdbx"
case ":$PATH:" in
  *":$INSTALL_DIR:"*) ;;
  *) echo "kdbx: NOTE — $INSTALL_DIR is not on your PATH; add it to your shell profile." ;;
esac
"$INSTALL_DIR/kdbx" --version
