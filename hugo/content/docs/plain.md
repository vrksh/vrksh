---
title: "vrk plain"
description: "markdown stripper - removes syntax, keeps prose"
tool: plain
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → plain → stdout`

Exit 0 Success · Exit 1 Could not read stdin or write stdout · Exit 2 Interactive TTY with no piped input and no positional arg

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | -j | bool | Emit JSON with text, input_bytes, output_bytes |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
cat README.md | vrk plain | vrk tok --budget 4000
```
