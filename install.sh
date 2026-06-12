#!/usr/bin/env bash
# installs the lagoon binary from github releases
# usage: curl -fsSL https://raw.githubusercontent.com/imraghavojha/lagoon/main/install.sh | bash
set -e

REPO="imraghavojha/lagoon"
BIN="lagoon"

# linux uses nix+bubblewrap; macOS uses apple/container (or docker)
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
if [ "$OS" != "linux" ] && [ "$OS" != "darwin" ]; then
  echo "error: lagoon runs on linux and macOS"
  exit 1
fi

# map uname arch to goreleaser arch names
ARCH=$(uname -m)
case "$ARCH" in
  aarch64|arm64) ARCH="arm64" ;;
  x86_64)        ARCH="amd64" ;;
  *) echo "error: unsupported arch: $ARCH"; exit 1 ;;
esac

if [ "$OS" = "darwin" ] && [ "$ARCH" != "arm64" ]; then
  echo "error: macOS support requires Apple Silicon (apple/container)"
  exit 1
fi

# get the latest release tag from github
# python3 parses json properly; grep+cut is a fragile fallback
_json=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest")
if command -v python3 >/dev/null 2>&1; then
  TAG=$(printf '%s' "$_json" | python3 -c "import json,sys; print(json.load(sys.stdin)['tag_name'])")
else
  TAG=$(printf '%s' "$_json" | grep '"tag_name"' | cut -d'"' -f4)
fi

if [ -z "$TAG" ]; then
  echo "error: could not fetch latest release tag"
  exit 1
fi

URL="https://github.com/$REPO/releases/download/$TAG/${BIN}_${OS}_${ARCH}.tar.gz"

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
  if [ "$OS" = "darwin" ] && ! command -v container >/dev/null 2>&1 && ! command -v docker >/dev/null 2>&1; then
    echo "note: install a container engine: brew install container && container system start"
  fi
else
  echo "error: install failed — binary did not start"
  exit 1
fi
