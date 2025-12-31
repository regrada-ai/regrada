#!/bin/sh
set -e

BASE_URL="https://regrada.com/releases"
VERSION="${REGRADA_VERSION:-latest}"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)

case "$os" in
  darwin) os="darwin" ;;
  linux) os="linux" ;;
  *) echo "Unsupported OS: $os" >&2; exit 1 ;;
esac

case "$arch" in
  x86_64|amd64) arch="amd64" ;;
  arm64|aarch64) arch="arm64" ;;
  *) echo "Unsupported arch: $arch" >&2; exit 1 ;;
esac

if [ "$VERSION" = "latest" ]; then
  VERSION="$(curl -fsSL "$BASE_URL/latest.txt")"
fi

tmpdir=$(mktemp -d)
trap 'rm -rf "$tmpdir"' EXIT

asset="regrada_${VERSION}_${os}_${arch}.tar.gz"
url="$BASE_URL/$VERSION/$asset"

echo "Downloading $url"
curl -fsSL "$url" -o "$tmpdir/$asset"

tar -C "$tmpdir" -xzf "$tmpdir/$asset"
chmod +x "$tmpdir/regrada"

install_dir="$HOME/.local/bin"
mkdir -p "$install_dir"
mv "$tmpdir/regrada" "$install_dir/regrada"

if ! echo "$PATH" | grep -q "$install_dir"; then
  echo "Installed to $install_dir, but it's not on PATH."
  echo "Add this to your shell profile:"
  echo "  export PATH=\"$install_dir:\$PATH\""
fi

echo "Installed regrada to $install_dir/regrada"
