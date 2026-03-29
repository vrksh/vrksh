---
title: "vrk completions"
description: "Shell completion script generator - bash, zsh, fish."
tool: completions
group: meta
mcp_callable: false
noindex: false
---

## What it does

Generates shell completion scripts for Bash, Zsh, and Fish.
After installation, Tab-completes tool names and their flags.

The completion script is generated from the tool registry inside the binary,
so it always matches the exact version you are running.

## Install completions

### Bash

```bash
vrk completions bash > ~/.bash_completion.d/vrk
source ~/.bash_completion.d/vrk
```

Or system-wide:

```bash
sudo vrk completions bash > /etc/bash_completion.d/vrk
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

Takes effect immediately in new sessions.

## Verify

Type `vrk` and press Tab to see all tool names:

```
$ vrk <TAB>
assert    base      chunk     coax      digest    emit      epoch
grab      jsonl     jwt       kv        links     mask      moniker
pct       plain     prompt    recase    sip       slug      sse
throttle  tok       urlinfo   uuid      validate
```

Type a tool name and `--` then Tab to see flag completions:

```
$ vrk tok --<TAB>
--budget  --json    --model   --quiet
```

## What gets completed

- All tool names as subcommands of `vrk`
- All flags for each tool (long and short forms)

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--json` | `-j` | bool | `false` | Emit errors as JSON |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Script emitted to stdout |
| 1 | Unknown shell argument |
| 2 | No shell argument provided |
