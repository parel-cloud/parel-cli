#!/usr/bin/env sh
# parel CLI installer (POSIX shell). Reads the latest GitHub release, downloads
# the right archive for the host OS+arch, verifies the SHA256, extracts to
# ${INSTALL_DIR:-$HOME/.local/bin}/parel, and clears the macOS quarantine bit
# for the v0.1 unsigned-binary period.
#
#   curl -fsSL https://parel.cloud/install.sh | sh
#   curl -fsSL https://parel.cloud/install.sh | INSTALL_DIR=/usr/local/bin sh

set -eu

REPO="parel-cloud/parel-cli"
BIN="parel"
INSTALL_DIR="${INSTALL_DIR:-$HOME/.local/bin}"
VERSION="${VERSION:-latest}"

bold() { printf '\033[1m%s\033[0m\n' "$1"; }
warn() { printf '\033[33m%s\033[0m\n' "$1" >&2; }
fail() { printf '\033[31merror:\033[0m %s\n' "$1" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || fail "$1 is required to run this installer"
}
need curl
need tar
need uname

OS_RAW="$(uname -s)"
case "$OS_RAW" in
  Darwin)  OS="darwin" ;;
  Linux)   OS="linux" ;;
  *)       fail "unsupported OS: $OS_RAW (use the Windows installer for Windows)" ;;
esac

ARCH_RAW="$(uname -m)"
case "$ARCH_RAW" in
  x86_64|amd64)         ARCH="amd64" ;;
  arm64|aarch64)        ARCH="arm64" ;;
  *)                    fail "unsupported architecture: $ARCH_RAW" ;;
esac

resolve_version() {
  if [ "$VERSION" = "latest" ]; then
    VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | head -1 \
      | sed -E 's/.*"tag_name":[[:space:]]*"([^"]+)".*/\1/')"
    [ -n "$VERSION" ] || fail "could not resolve latest release tag"
  fi
  case "$VERSION" in
    v*) ;;
    *)  VERSION="v$VERSION" ;;
  esac
}

resolve_version

VERSION_BARE="${VERSION#v}"
ARCHIVE_NAME="parel_${VERSION_BARE}_${OS}_${ARCH}.tar.gz"
ARCHIVE_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ARCHIVE_NAME}"
SHASUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/SHA256SUMS"

bold "Installing parel ${VERSION} (${OS}/${ARCH})"

TMP="$(mktemp -d 2>/dev/null || mktemp -d -t parel-install)"
trap 'rm -rf "$TMP"' EXIT

curl -fsSL -o "$TMP/$ARCHIVE_NAME" "$ARCHIVE_URL" \
  || fail "failed to download $ARCHIVE_URL"

if curl -fsSL -o "$TMP/SHA256SUMS" "$SHASUM_URL" 2>/dev/null; then
  EXPECTED="$(grep "$ARCHIVE_NAME" "$TMP/SHA256SUMS" | awk '{print $1}')"
  if [ -n "$EXPECTED" ]; then
    if command -v shasum >/dev/null 2>&1; then
      ACTUAL="$(shasum -a 256 "$TMP/$ARCHIVE_NAME" | awk '{print $1}')"
    else
      ACTUAL="$(sha256sum "$TMP/$ARCHIVE_NAME" | awk '{print $1}')"
    fi
    [ "$EXPECTED" = "$ACTUAL" ] || fail "SHA256 mismatch (expected $EXPECTED, got $ACTUAL)"
    echo "  SHA256 OK"
  else
    warn "could not find $ARCHIVE_NAME in SHA256SUMS; skipping verification"
  fi
else
  warn "SHA256SUMS not available; skipping verification"
fi

tar -xzf "$TMP/$ARCHIVE_NAME" -C "$TMP"
mkdir -p "$INSTALL_DIR"
mv "$TMP/$BIN" "$INSTALL_DIR/$BIN"
chmod 0755 "$INSTALL_DIR/$BIN"

# v0.1 unsigned macOS binary: clear Gatekeeper quarantine so users don't see
# "developer cannot be verified" on first run.
if [ "$OS" = "darwin" ] && command -v xattr >/dev/null 2>&1; then
  xattr -d com.apple.quarantine "$INSTALL_DIR/$BIN" 2>/dev/null || true
fi

if ! command -v parel >/dev/null 2>&1; then
  warn "$INSTALL_DIR is not on PATH. Add this to your shell profile:"
  printf '\n  export PATH="%s:$PATH"\n\n' "$INSTALL_DIR"
fi

"$INSTALL_DIR/$BIN" version
bold "Done. Try: parel auth login"
