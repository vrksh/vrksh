---
title: "vrk bare"
description: "symlink creator - use vrksh tools without the vrk prefix"
tool: bare
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → bare → stdout`

Exit 0 Success · Exit 1 Filesystem error creating or removing symlinks · Exit 2 Usage error

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--force` |   | bool | Overwrite existing files at symlink paths |
| `--remove` |   | bool | Remove bare symlinks (only those pointing to vrk) |
| `--list` |   | bool | List currently active bare symlinks |
| `--dry-run` |   | bool | Show what would happen, make no changes |

## Example

```bash
vrk --bare --dry-run
```
