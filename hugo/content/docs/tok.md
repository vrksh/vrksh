---
title: "vrk tok"
description: "token counter - cl100k_base, --budget guard, --json."
tool: tok
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → tok → stdout`

Exit 0 Success, within budget · Exit 1 Over budget or I/O error · Exit 2 Usage error - no input, unknown flag

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | -j | bool | Emit JSON with token count and metadata |
| `--budget` |   | int | Exit 1 if token count exceeds N |
| `--model` | -m | string | Tokenizer model (currently cl100k_base only) |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
cat prompt.txt | vrk tok --budget 4000
```
