---
title: "vrk epoch"
description: "timestamp converter - unix/ISO, relative time"
tool: epoch
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → epoch → stdout`

Exit 0 Success · Exit 1 Runtime error (I/O failure) · Exit 2 Unsupported format, ambiguous timezone, --tz without --iso/--json

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--iso` |   | bool | Output as ISO 8601 string instead of Unix integer |
| `--json` | -j | bool | Emit JSON with all representations |
| `--tz` |   | string | Timezone for --iso or --json output (IANA name or offset) |
| `--now` |   | bool | Print current Unix timestamp without reading stdin |
| `--at` |   | string | Reference timestamp for relative input (makes scripts deterministic) |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
vrk epoch 1740009600 --iso
```
