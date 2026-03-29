---
title: "vrk completions"
description: "Generate shell completion scripts for bash, zsh, and fish. Tab-complete every vrk tool and flag."
og_title: "vrk completions - shell completion for bash, zsh, and fish"
tool: completions
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Generates tab-completion scripts for your shell. After installing, vrk <tab> completes tool names and vrk tok --<tab> completes flags. The completions are generated from the binary itself, so they always match the version you have installed.

## The problem

You install vrksh but tab-completion does not work. You cannot remember all 28 tool names or their flags. You type vrk and hit tab and nothing happens.

## Before and after

**Before**

```bash
# write a bash completion script by hand
_vrk_complete() {
  COMPREPLY=($(compgen -W "tok jwt epoch ..." -- "${COMP_WORDS[1]}"))
}
complete -F _vrk_complete vrk
```

**After**

```bash
vrk completions bash > ~/.bash_completion.d/vrk
```

## Example

```bash
vrk completions bash > ~/.bash_completion.d/vrk
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Script emitted to stdout |
| 1 | Unknown shell argument |
| 2 | No shell argument provided |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | -j | bool | Emit errors as JSON |

