---
title: "vrk recase"
description: "naming convention converter - snake, camel, kebab, pascal, title"
tool: recase
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → recase → stdout`

Exit 0 Success · Exit 1 Runtime error (I/O failure) · Exit 2 --to missing or invalid value, interactive TTY with no stdin

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--to` |   | string | Target convention: camel, pascal, snake, kebab, screaming, title, lower, upper |
| `--json` | -j | bool | Emit JSON per line: {input, output, from, to} |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
echo 'getUserName' | vrk recase --to snake
```
