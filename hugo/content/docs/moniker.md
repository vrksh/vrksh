---
title: "vrk moniker"
description: "memorable name generator - run IDs, job labels, temp dirs"
tool: moniker
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → moniker → stdout`

Exit 0 Success · Exit 1 Word pool exhausted for requested count · Exit 2 --count less than 1, --words less than 2

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--count` | -n | int | Number of names to generate |
| `--separator` |   | string | Word separator |
| `--words` |   | int | Words per name (minimum 2) |
| `--seed` |   | int64 | Random seed for deterministic output |
| `--json` | -j | bool | Emit JSON per name: {name, words} |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
vrk moniker --seed 42
```
