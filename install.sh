#!/usr/bin/env bash
# installs the lagoon binary from github releases
# usage: curl -fsSL https://raw.githubusercontent.com/kuldeepojha/lagoon/main/install.sh | bash
set -e

REPO="kuldeepojha/lagoon"
BIN="lagoon"

# lagoon requires bwrap which is linux-only
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ]; then
  echo "error: lagoon only runs on linux (requires bubblewrap)"
  exit 1
fi

# map uname arch to goreleaser arch names
ARCH=$(uname -m)
case "$ARCH" in
  aarch64) ARCH="arm64" ;;
  x86_64)  ARCH="amd64" ;;
  *) echo "error: unsupported arch: $ARCH"; exit 1 ;;
esac

# get the latest release tag from github
TAG=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | cut -d'"' -f4)

if [ -z "$TAG" ]; then
  echo "error: could not fetch latest release tag"
  exit 1
fi

URL="https://github.com/$REPO/releases/download/$TAG/${BIN}_linux_${ARCH}.tar.gz"

# use /usr/local/bin if writable, otherwise ~/.local/bin
if [ -w /usr/local/bin ]; then
  DEST="/usr/local/bin"
else
  DEST="$HOME/.local/bin"
  mkdir -p "$DEST"
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$URL" | tar -xz -C "$TMPDIR"
mv "$TMPDIR/$BIN" "$DEST/$BIN"
chmod +x "$DEST/$BIN"

if "$DEST/$BIN" --help >/dev/null 2>&1; then
  echo "installed $BIN $TAG to $DEST/$BIN"
  if [ "$DEST" = "$HOME/.local/bin" ]; then
    echo "note: add $DEST to your PATH if it is not already there"
  fi
else
  echo "error: install failed — binary did not start"
  exit 1
fi
