#!/bin/sh
# vrksh installer - detects OS/arch, downloads binary, verifies SHA256, installs.
# Usage: curl -sSL https://vrk.sh/install.sh | sh
#
# Environment:
#   VRK_VERSION      version to install, e.g. 0.2.0 (default: latest release)
#   VRK_INSTALL_DIR  install directory (default: /usr/local/bin, else ~/.local/bin)
set -e

# Used only when VRK_VERSION is unset and the GitHub API is unreachable.
# The release workflow fails if this does not match the tag being released.
FALLBACK_VERSION="0.2.0"
REPO="vrksh/vrksh"

fail() {
  printf 'error: %s\n' "$1" >&2
  exit 1
}

# fetch URL: print the body to stdout
fetch() {
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "$1"
  elif command -v wget >/dev/null 2>&1; then
    wget -q -O - "$1"
  else
    fail "neither curl nor wget found"
  fi
}

# Version order: $VRK_VERSION, latest GitHub release, pinned fallback.
VERSION="${VRK_VERSION#v}"
if [ -z "$VERSION" ]; then
  VERSION="$(fetch "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | grep '"tag_name"' | head -n 1 | sed -E 's/.*"tag_name"[[:space:]]*:[[:space:]]*"v?([^"]+)".*/\1/')" || VERSION=""
fi
if [ -z "$VERSION" ]; then
  VERSION="$FALLBACK_VERSION"
  printf 'note: could not look up the latest release, using %s\n' "$VERSION" >&2
fi

# Detect OS
OS="$(uname -s)"
case "$OS" in
  Darwin) OS="darwin" ;;
  Linux)  OS="linux" ;;
  *)      fail "unsupported OS: $OS" ;;
esac

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)         ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *)              fail "unsupported architecture: $ARCH" ;;
esac

TARBALL="vrk_${VERSION}_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"
DOWNLOAD_URL="${BASE_URL}/${TARBALL}"
CHECKSUMS_URL="${BASE_URL}/vrk_${VERSION}_checksums.txt"

printf 'installing vrk v%s (%s/%s)\n' "$VERSION" "$OS" "$ARCH"

# Create temp directory
TMPDIR="$(mktemp -d)"
trap 'rm -rf "$TMPDIR"' EXIT

# Download binary and checksums
printf 'downloading %s\n' "$DOWNLOAD_URL"
if command -v curl >/dev/null 2>&1; then
  curl -sSL -o "${TMPDIR}/${TARBALL}" "$DOWNLOAD_URL" || fail "download failed"
  curl -sSL -o "${TMPDIR}/checksums.txt" "$CHECKSUMS_URL" || fail "checksums download failed"
elif command -v wget >/dev/null 2>&1; then
  wget -q -O "${TMPDIR}/${TARBALL}" "$DOWNLOAD_URL" || fail "download failed"
  wget -q -O "${TMPDIR}/checksums.txt" "$CHECKSUMS_URL" || fail "checksums download failed"
else
  fail "neither curl nor wget found"
fi

# Verify SHA256
printf 'verifying checksum\n'
EXPECTED="$(grep "$TARBALL" "${TMPDIR}/checksums.txt" | awk '{print $1}')"
if [ -z "$EXPECTED" ]; then
  fail "checksum not found for $TARBALL in checksums.txt"
fi

if command -v sha256sum >/dev/null 2>&1; then
  ACTUAL="$(sha256sum "${TMPDIR}/${TARBALL}" | awk '{print $1}')"
elif command -v shasum >/dev/null 2>&1; then
  ACTUAL="$(shasum -a 256 "${TMPDIR}/${TARBALL}" | awk '{print $1}')"
else
  fail "neither sha256sum nor shasum found"
fi

if [ "$EXPECTED" != "$ACTUAL" ]; then
  fail "checksum mismatch: expected $EXPECTED, got $ACTUAL"
fi

# Extract
tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR" || fail "extraction failed"

# Install
if [ -n "$VRK_INSTALL_DIR" ]; then
  INSTALL_DIR="$VRK_INSTALL_DIR"
  mkdir -p "$INSTALL_DIR" || fail "cannot create $INSTALL_DIR"
else
  INSTALL_DIR="/usr/local/bin"
  if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="${HOME}/.local/bin"
    mkdir -p "$INSTALL_DIR"
  fi
fi

mv "${TMPDIR}/vrk" "${INSTALL_DIR}/vrk" || fail "install failed: could not move binary to ${INSTALL_DIR}"
chmod +x "${INSTALL_DIR}/vrk"

printf 'installed vrk %s to %s/vrk\n' "$VERSION" "$INSTALL_DIR"

# Check if install dir is in PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) printf 'note: add %s to your PATH\n' "$INSTALL_DIR" ;;
esac
