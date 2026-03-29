---
title: "vrk sip"
description: "stream sampler - --first, --count, --every, --sample"
tool: sip
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Samples lines from a stream without loading the whole thing into memory. You can take a random sample of N lines, every Nth line, or the first N lines. Uses reservoir sampling for uniform random selection, so every line has an equal chance of being picked.

## The problem

You have a 10GB JSONL file and need a random sample of 100 records. head gives you the first 100, not a random sample. shuf loads the entire file into memory. You write a Python script with reservoir sampling and it takes 20 lines.

## Before and after

**Before**

```bash
shuf -n 100 huge.jsonl
# loads entire file into memory
# not available on macOS without coreutils
```

**After**

```bash
cat huge.jsonl | vrk sip --count 100 --seed 42
```

## Example

```bash
cat huge.jsonl | vrk sip --count 100 --seed 42
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | I/O failure reading stdin |
| 2 | No strategy specified, multiple strategies, --sample outside 1-100, interactive TTY |

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

