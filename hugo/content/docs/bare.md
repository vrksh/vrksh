---
title: "vrk bare"
description: "vrk --bare creates symlinks so you can run tok, grab, and prompt without the vrk prefix. Remove the symlinks later with --remove."
meta_lead: "vrk --bare creates symlinks so you can call any tool by name without the vrk prefix."
og_title: "vrk bare - use vrksh tools without the vrk prefix"
tool: bare
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

vrk --bare creates symlinks so you can call any tool by name without the vrk prefix.

## The problem

A multicall binary means typing the prefix on every invocation. In an interactive session where you run 30 commands in a row, the prefix adds up.

## The solution

`vrk --bare` creates symlinks so you can call any tool by name directly: `tok`, `jwt`, `epoch` instead of `vrk tok`, `vrk jwt`, `vrk epoch`. Symlinks point back to the same binary. Nothing is copied. Remove them with `--remove`.

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

`vrk bare` creates a symlink for each vrk tool, so you can run `tok` instead of `vrk tok`.

### Create symlinks

<!-- doccheck: skip (creates symlinks next to the binary) -->
```bash
$ vrk bare
Linking assert   → /usr/local/bin/assert  ✓
Linking base     → /usr/local/bin/base  ✓
...
26 linked.
```

Each tool gets a symlink: `tok -> vrk`, `jwt -> vrk`, `epoch -> vrk`, and so on. The multicall binary checks the name it was started as and runs that tool.

### Preview before creating (--dry-run)

<!-- doccheck: skip (creates symlinks next to the binary) -->
```bash
$ vrk bare --dry-run
Linking assert   → /usr/local/bin/assert
Linking base     → /usr/local/bin/base
...
```

### Check existing symlinks (--list)

<!-- doccheck: skip (depends on which symlinks exist) -->
```bash
$ vrk bare --list
assert    /usr/local/bin/assert
base      /usr/local/bin/base
...
```

### Remove symlinks (--remove)

<!-- doccheck: skip (removes symlinks next to the binary) -->
```bash
$ vrk bare --remove
Removed assert
Removed base
...
26 removed.
```

`--remove` deletes only symlinks that point to the vrk binary. It does not touch other files with the same names.

### Overwrite conflicts (--force)

If a file already exists at a symlink path, `--force` replaces it:

```bash
vrk bare --force
```

If you do not use `--force`, bare skips existing files and prints a warning.

## After setup

```bash
# Before: vrk prefix required
vrk tok --check 8000 < prompt.txt

# After: direct invocation
tok --check 8000 < prompt.txt
grab https://example.com | prompt --system 'Summarize'
```

All flags and pipes work the same as before.
