---
title: "Install"
description: "Install vrk on macOS or Linux with Homebrew, go install, or a shell script. Includes shell completions, bare mode, and uninstall steps."
noindex: false
---

vrk is one static binary with no runtime and no dependencies. Pick the install method that fits your setup.

## Homebrew (macOS and Linux)

If you already have Homebrew, use it:

```bash
brew tap vrksh/vrksh
brew install vrk
```

## go install

If you have Go 1.25+ installed:

```bash
go install github.com/vrksh/vrksh@latest
```

Go puts the binary in `$GOPATH/bin/vrk`. Make sure that directory is in your `$PATH`.

## Shell script (CI, Docker, and ephemeral environments)

This needs no package manager:

```bash
curl -fsSL vrk.sh/install.sh | sh
```

The script detects your OS and architecture, downloads the right binary, and verifies the SHA256 checksum. It installs to `/usr/local/bin/vrk`, or to `~/.local/bin/vrk` if `/usr/local/bin` is not writable. Use it in Dockerfiles, CI pipelines, and fresh VMs where you do not want to install Homebrew or Go.

### Agent bootstrap

To set up vrk for an AI agent, use the agent script. It installs the binary and prints an onboarding block. The agent reads the block to learn how to use the tools:

```bash
curl -fsSL vrk.sh/agent.sh | sh
```

## Verify

Run this to check the install:

```bash
vrk --manifest | jq '.tools | length'
```

```
28
```

Or run a tool:

```bash
echo 'hello world' | vrk tok
```

```
2
```

## Shell completions

Completions let you press tab to finish tool names and flags. Set them up once:

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

After setup, `vrk <tab>` completes tool names and `vrk tok --<tab>` completes flags.

## Bare mode (optional)

If you use vrk interactively, you can drop the `vrk` prefix:

```bash
vrk --bare
```

After this, `tok`, `jwt`, `epoch`, and every other tool works without the `vrk` prefix. See the [bare docs](/docs/bare/) for details.

## Uninstall

Homebrew:

```bash
brew uninstall vrk
```

Manual:

```bash
vrk --bare --remove 2>/dev/null   # remove symlinks if you used bare mode
rm $(which vrk)
```

## Platforms

vrk is a static binary with no CGO. It runs on these platforms:

- **macOS** - amd64 (Intel) and arm64 (Apple Silicon)
- **Linux** - amd64 and arm64

Windows is not currently supported.
