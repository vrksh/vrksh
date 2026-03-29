---
title: "vrk sip"
description: "stream sampler - --first, --count, --every, --sample"
tool: sip
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → sip → stdout`

Exit 0 Success · Exit 1 I/O failure reading stdin · Exit 2 No strategy specified, multiple strategies, --sample outside 1-100, interactive TTY

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--first` |   | int | Take first N lines |
| `--count` | -n | int | Reservoir sample of exactly N lines |
| `--every` |   | int | Emit every Nth line |
| `--sample` |   | int | Include each line with N% probability (1-100) |
| `--seed` |   | int64 | Random seed for reproducibility |
| `--json` | -j | bool | Append metadata record after all output |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
cat huge.jsonl | vrk sip --count 100 --seed 42
```
