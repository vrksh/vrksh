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

## The problem

A multicall binary has 26 tool names and hundreds of flags. Without tab completion you are guessing names and checking `--help` constantly.

## The solution

`vrk completions` generates tab-completion scripts for bash, zsh, and fish. After installing, `vrk <tab>` completes tool names and `vrk tok --<tab>` completes flags. The completions are generated from the binary itself, so they always match the installed version.

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

| Code | Meaning                    |
|------|----------------------------|
| 0    | Script emitted to stdout   |
| 1    | Unknown shell argument     |
| 2    | No shell argument provided |

## Flags

| Flag     | Short | Type | Description         |
|----------|-------|------|---------------------|
| `--json` | -j    | bool | Emit errors as JSON |


<!-- notes - edit in notes/completions.notes.md -->

## Setup

### Bash

```bash
vrk completions bash > ~/.bash_completion.d/vrk
source ~/.bash_completion.d/vrk
```

### Zsh

```bash
vrk completions zsh > ~/.zsh/completions/_vrk
# Add to ~/.zshrc if not already there:
# fpath=(~/.zsh/completions $fpath)
# autoload -Uz compinit && compinit
```

### Fish

```bash
vrk completions fish > ~/.config/fish/completions/vrk.fish
```

## What you get

After installing completions:

```
$ vrk <tab>
assert   base   chunk   coax   completions   digest   emit   epoch   grab   ...

$ vrk tok --<tab>
--check   --json   --model   --quiet
```

Tool names and flags are completed from the binary itself, so they always match the installed version.

## When it fails

Unknown shell:

```bash
$ vrk completions powershell
error: completions: unsupported shell "powershell"
$ echo $?
1
```

No shell specified:

```bash
$ vrk completions
usage error: completions: shell argument required (bash, zsh, fish)
$ echo $?
2
```
