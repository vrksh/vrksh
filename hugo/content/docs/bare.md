---
title: "vrk bare"
description: "Create symlinks for any vrk tool. Use tok, grab, prompt directly without the vrk prefix."
og_title: "vrk bare - use vrksh tools without the vrk prefix"
tool: bare
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Lets you call any vrksh tool by name without the vrk prefix. Instead of vrk tok, vrk jwt, vrk epoch, you just type tok, jwt, epoch. It creates symlinks in the same directory as the binary - nothing is copied, nothing is installed elsewhere.

## The problem

Every vrksh command requires the vrk prefix. In interactive sessions you type vrk tok, vrk jwt, vrk epoch dozens of times a day. The prefix adds friction without adding clarity.

## Before and after

**Before**

```bash
alias tok='vrk tok'
alias jwt='vrk jwt'
alias epoch='vrk epoch'
# repeat for 28 tools...
```

**After**

```bash
vrk bare
```

## Example

```bash
vrk --bare --dry-run
```

## Exit codes

| Code | Meaning                                        |
|------|------------------------------------------------|
| 0    | Success                                        |
| 1    | Filesystem error creating or removing symlinks |
| 2    | Usage error                                    |

## Flags

| Flag        | Short | Type | Description                                       |
|-------------|-------|------|---------------------------------------------------|
| `--force`   |       | bool | Overwrite existing files at symlink paths         |
| `--remove`  |       | bool | Remove bare symlinks (only those pointing to vrk) |
| `--list`    |       | bool | List currently active bare symlinks               |
| `--dry-run` |       | bool | Show what would happen, make no changes           |


<!-- notes - edit in notes/bare.notes.md -->

## How it works

### Create symlinks

```bash
$ vrk bare
Created 26 symlinks in /usr/local/bin/
```

Each tool gets a symlink: `tok -> vrk`, `jwt -> vrk`, `epoch -> vrk`, etc. The multicall binary detects which name it was invoked as and dispatches to the right tool.

### Preview before creating (--dry-run)

```bash
$ vrk bare --dry-run
Would create: /usr/local/bin/tok -> /usr/local/bin/vrk
Would create: /usr/local/bin/jwt -> /usr/local/bin/vrk
Would create: /usr/local/bin/epoch -> /usr/local/bin/vrk
...
```

### Check existing symlinks (--list)

```bash
$ vrk bare --list
tok -> /usr/local/bin/vrk
jwt -> /usr/local/bin/vrk
epoch -> /usr/local/bin/vrk
```

### Remove symlinks (--remove)

```bash
$ vrk bare --remove
Removed 26 symlinks
```

Only removes symlinks that point to the vrk binary. Other files with the same names are left untouched.

### Overwrite conflicts (--force)

If a file already exists at a symlink path, `--force` replaces it:

```bash
vrk bare --force
```

Without `--force`, existing files are skipped with a warning.

## After setup

```bash
# Before: vrk prefix required
vrk tok --check 8000 < prompt.txt

# After: direct invocation
tok --check 8000 < prompt.txt
grab https://example.com | prompt --system 'Summarize'
```

All flags and piping work identically.
