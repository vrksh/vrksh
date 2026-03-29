---
title: "vrk base"
description: "encoding converter - base64, base64url, hex, base32"
tool: base
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → base → stdout`

Exit 0 Success · Exit 1 Invalid encoded input (bad characters, wrong padding) · Exit 2 Missing subcommand, --to/--from missing or unsupported

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--to` |   | string | Target encoding: base64, base64url, hex, base32 (encode subcommand) |
| `--from` |   | string | Source encoding: base64, base64url, hex, base32 (decode subcommand) |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
echo 'hello' | vrk base encode --to base64
```
