---
title: "vrk bare"
description: "symlink creator - use vrksh tools without the vrk prefix"
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

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Filesystem error creating or removing symlinks |
| 2 | Usage error |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` |   | bool | Overwrite existing files at symlink paths |
| `--remove` |   | bool | Remove bare symlinks (only those pointing to vrk) |
| `--list` |   | bool | List currently active bare symlinks |
| `--dry-run` |   | bool | Show what would happen, make no changes |

