#!/usr/bin/env sh
set -e

REPO="faizalv/lemongrass"
INSTALL_DIR="/usr/local/bin"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)        ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH"
    exit 1 ;;
esac

if ! command -v docker >/dev/null 2>&1; then
  echo "Docker is required. Install at https://docs.docker.com/get-docker/"
  exit 1
fi
if ! docker info >/dev/null 2>&1; then
  echo "Docker is not running. Start Docker and try again."
  exit 1
fi

VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"v\([^"]*\)".*/\1/')

echo "Lemongrass v${VERSION}  ${OS}/${ARCH}"
echo ""

BINARY="lemongrass-${OS}-${ARCH}"
URL="https://github.com/${REPO}/releases/download/v${VERSION}/${BINARY}"
TMP=$(mktemp)

printf "Downloading...  "
curl -fsSL "$URL" -o "$TMP"
echo "done"

EXPECTED=$(curl -fsSL \
  "https://github.com/${REPO}/releases/download/v${VERSION}/checksums.txt" \
  | grep "$BINARY" | awk '{print $1}')
ACTUAL=$(sha256sum "$TMP" 2>/dev/null | awk '{print $1}' \
  || shasum -a 256 "$TMP" | awk '{print $1}')
[ "$EXPECTED" = "$ACTUAL" ] || { echo "Checksum mismatch."; rm -f "$TMP"; exit 1; }

chmod +x "$TMP"
if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "${INSTALL_DIR}/lemongrass"
else
  sudo mv "$TMP" "${INSTALL_DIR}/lemongrass"
fi

SHELL_NAME=$(basename "$SHELL")
RC_FILE=""
COMPLETION_LINE=""
case "$SHELL_NAME" in
  bash)
    RC_FILE="$HOME/.bashrc"
    COMPLETION_LINE='eval "$(lemongrass completion bash)"'
    ;;
  zsh)
    RC_FILE="$HOME/.zshrc"
    COMPLETION_LINE='eval "$(lemongrass completion zsh)"'
    ;;
esac

if [ -n "$RC_FILE" ] && [ -n "$COMPLETION_LINE" ]; then
  if ! grep -qF "lemongrass completion" "$RC_FILE" 2>/dev/null; then
    printf '\n# lemongrass\n%s\n' "$COMPLETION_LINE" >> "$RC_FILE"
    echo "Tab completion added to $RC_FILE"
  fi
fi

echo ""
echo "Installed lemongrass v${VERSION}"
echo "Run: lemongrass up"
