---
title: "Install"
description: "Install vrksh - Homebrew, go install, or shell script."
noindex: false
---

## Homebrew (macOS and Linux)

```bash
brew install vrksh/homebrew-vrksh/vrk
```

The tap is [vrksh/homebrew-vrksh](https://github.com/vrksh/homebrew-vrksh). Homebrew handles updates automatically.

Verify the install:

```bash
vrk --manifest | head -1
```

## go install

```bash
go install github.com/vrksh/vrksh@latest
```

Requires Go 1.25+. The binary lands in `$GOPATH/bin/vrk`.

## Shell script (CI and ephemeral environments)

```bash
curl -fsSL vrk.sh/install.sh | sh
```

The script detects your OS and architecture, downloads the right binary, verifies the SHA256 checksum, and installs to `/usr/local/bin/vrk` (or `~/.local/bin/vrk` if `/usr/local/bin` is not writable).

For environments where you want the agent onboarding block printed after install:

```bash
curl -fsSL vrk.sh/agent.sh | sh
```

## Shell completions

### Bash

```bash
vrk completions bash > ~/.bash_completion.d/vrk
source ~/.bash_completion.d/vrk
```

### Zsh

```bash
vrk completions zsh > "${fpath[1]}/_vrk"
exec zsh
```

### Fish

```bash
vrk completions fish > ~/.config/fish/completions/vrk.fish
```

## Verify

```bash
vrk --manifest | jq '.tools | length'
```

```
26
```

## Uninstall

Homebrew:

```bash
brew uninstall vrk
```

Manual:

```bash
rm $(which vrk)
```

If you used `vrk --bare` to create symlinks, remove them first:

```bash
vrk --bare --remove
rm $(which vrk)
```

## Platform note

vrksh is a single static binary - no runtime dependencies, no CGO. It runs anywhere Go cross-compiles to: Linux (amd64, arm64) and macOS (amd64, arm64). Windows is not currently supported.
