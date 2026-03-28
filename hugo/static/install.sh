#!/bin/sh
# vrksh installer - detects OS/arch, downloads binary, verifies SHA256, installs.
# Usage: curl -sSL https://vrk.sh/install.sh | sh
set -e

VERSION="0.1.0"
REPO="vrksh/vrksh"

fail() {
  printf 'error: %s\n' "$1" >&2
  exit 1
}

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

TARBALL="vrk_${OS}_${ARCH}.tar.gz"
BASE_URL="https://github.com/${REPO}/releases/download/v${VERSION}"
DOWNLOAD_URL="${BASE_URL}/${TARBALL}"
CHECKSUMS_URL="${BASE_URL}/checksums.txt"

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
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "$INSTALL_DIR"
fi

mv "${TMPDIR}/vrk" "${INSTALL_DIR}/vrk" || fail "install failed: could not move binary to ${INSTALL_DIR}"
chmod +x "${INSTALL_DIR}/vrk"

printf 'vrk v%s installed to %s/vrk\n' "$VERSION" "$INSTALL_DIR"

# Check if install dir is in PATH
case ":${PATH}:" in
  *":${INSTALL_DIR}:"*) ;;
  *) printf 'note: add %s to your PATH\n' "$INSTALL_DIR" ;;
esac
