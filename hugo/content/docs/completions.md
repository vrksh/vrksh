---
title: "vrk completions"
description: "Shell completion script generator — bash, zsh, fish."
tool: completions
group: meta
mcp_callable: false
noindex: false
---

## The problem

Typing `vrk` followed by a tool name and flags from memory is slow and
error-prone, especially when a tool has flags you rarely use.

## The fix

Generate a completion script for your shell and source it once. After that,
pressing Tab completes tool names and flags.

### Bash

```bash
vrk completions bash > ~/.bash_completion.d/vrk
source ~/.bash_completion.d/vrk
```

Or system-wide:

```bash
vrk completions bash > /etc/bash_completion.d/vrk
```

### Zsh

```bash
vrk completions zsh > "${fpath[1]}/_vrk"
```

### Fish

```bash
vrk completions fish > ~/.config/fish/completions/vrk.fish
```

## How it works

The completion script is generated from the tool registry that ships inside the
binary. Every tool registers its name, description, and flags at init time, so
the completion output always matches the exact binary you are running.
