---
title: "vrk uuid"
description: "UUID generator - v4/v7, --count, --json"
tool: uuid
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → uuid → stdout`

Exit 0 Success · Exit 1 Runtime error (generation failure) · Exit 2 --count less than 1, unknown flag

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--v7` |   | bool | Generate a v7 (time-ordered) UUID instead of v4 |
| `--count` | -n | int | Number of UUIDs to generate |
| `--json` | -j | bool | Emit JSON with uuid, version, generated_at |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
vrk uuid --v7
```
