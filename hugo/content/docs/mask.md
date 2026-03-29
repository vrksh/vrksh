---
title: "vrk mask"
description: "secret redactor - entropy + pattern-based, streaming"
tool: mask
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → mask → stdout`

Exit 0 All lines processed · Exit 1 Stdin scanner error or write failure · Exit 2 Interactive TTY with no piped input, invalid regex

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--pattern` |   | []string | Additional Go regex to match and redact (repeatable) |
| `--entropy` |   | float64 | Shannon entropy threshold; lower catches more |
| `--json` | -j | bool | Append metadata trailer after output |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
cat output.txt | vrk mask
```
