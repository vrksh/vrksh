---
title: "Install"
description: "Install vrksh - Homebrew, go install, or binary download."
noindex: false
---

## Homebrew

```bash
brew install vrk
```

The tap is [vrksh/homebrew-vrksh](https://github.com/vrksh/homebrew-vrksh). Homebrew handles updates automatically.

## go install

```bash
go install github.com/vrksh/vrksh@latest
```

Requires Go 1.22 or later. The binary lands in `$GOPATH/bin/vrk`.

## Binary download

```bash
curl -sSL https://vrk.sh/install.sh | sh
```

The install script detects your OS and architecture, downloads the right binary, verifies the SHA256 checksum, and installs to `/usr/local/bin/vrk` (or `~/.local/bin/vrk` if `/usr/local/bin` is not writable).

### Manual download

Download from [GitHub releases](https://github.com/vrksh/vrksh/releases):

| Platform | File |
|----------|------|
| macOS (Apple Silicon) | `vrk_darwin_arm64.tar.gz` |
| macOS (Intel) | `vrk_darwin_amd64.tar.gz` |
| Linux (x86_64) | `vrk_linux_amd64.tar.gz` |
| Linux (ARM64) | `vrk_linux_arm64.tar.gz` |

### Verify

```bash
sha256sum vrk_linux_amd64.tar.gz
# compare with checksums.txt from the release
```

### Symlink (optional)

If your agent environment expects tools in a specific location:

```bash
ln -sf /usr/local/bin/vrk /usr/local/bin/vrk-tok
```

The multicall binary responds to `vrk tok` and symlinked names like `vrk-tok`.

## Platform note

vrksh is a single static binary - no runtime dependencies, no CGO. It runs anywhere Go cross-compiles to: Linux (amd64, arm64) and macOS (amd64, arm64). Windows is not currently supported.
