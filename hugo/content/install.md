---
title: "Install"
description: "Install vrksh - one command, any platform. Homebrew, go install, or shell script."
noindex: false
---

One static binary. No runtime. No dependencies. Pick the method that fits your setup.

## Homebrew (macOS and Linux)

The fastest path if you already have Homebrew:

```bash
brew tap vrksh/vrksh
brew install vrk
```

## go install

If you have Go 1.25+ installed:

```bash
go install github.com/vrksh/vrksh@latest
```

The binary lands in `$GOPATH/bin/vrk`. Make sure that's on your `$PATH`.

## Shell script (CI, Docker, and ephemeral environments)

One line, no package manager required:

```bash
curl -fsSL vrk.sh/install.sh | sh
```

The script detects your OS and architecture, downloads the right binary, verifies the SHA256 checksum, and installs to `/usr/local/bin/vrk` (or `~/.local/bin/vrk` if `/usr/local/bin` is not writable). Use this in Dockerfiles, CI pipelines, and fresh VMs where you don't want to install Homebrew or Go.

### Agent bootstrap

If you're setting up vrk for an AI agent, use the agent script instead. It installs the binary and then prints an onboarding block the agent can read to learn how to use the tools:

```bash
curl -fsSL vrk.sh/agent.sh | sh
```

## Verify

Check that everything works:

```bash
vrk --manifest | jq '.tools | length'
```

```
26
```

Or just run a tool:

```bash
echo 'hello world' | vrk tok
```

```
2
```

## Shell completions

Tab-complete tool names and flags. Set up once, works forever:

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

If you use vrk interactively and want to drop the prefix:

```bash
vrk --bare
```

Now `tok`, `jwt`, `epoch`, and every other tool works without the `vrk` prefix. See the [bare docs](/docs/bare/) for details.

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

vrk is a single static binary with no CGO. It runs anywhere Go cross-compiles to:

- **macOS** - amd64 (Intel) and arm64 (Apple Silicon)
- **Linux** - amd64 and arm64

Windows is not currently supported.
