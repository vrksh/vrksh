---
title: "vrk mask"
description: "Redact secrets from pipeline output before logging. Detects API keys, tokens, and high-entropy strings automatically."
og_title: "vrk mask - automatic secret redaction for pipeline output"
tool: mask
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Redacts secrets from text before it reaches an LLM or a log file. Catches Bearer tokens, passwords, API keys, and other credentials using built-in patterns. Also flags high-entropy strings that look like secrets but don't match a known pattern. Streams input, so it handles large files safely.

## The problem

You pipe log output to an LLM for analysis and accidentally send API keys, Bearer tokens, and passwords in the prompt. The LLM provider now has your production credentials in their training pipeline.

## Before and after

**Before**

```bash
cat output.log | sed 's/Bearer [^ ]*/Bearer ***/g' | \
  sed 's/password=[^ ]*/password=***/g'
# misses API keys, high-entropy secrets, custom patterns
```

**After**

```bash
cat output.log | vrk mask
```

## Example

```bash
cat output.txt | vrk mask
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All lines processed |
| 1 | Stdin scanner error or write failure |
| 2 | Interactive TTY with no piped input, invalid regex |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--pattern` |   | []string | Additional Go regex to match and redact (repeatable) |
| `--entropy` |   | float64 | Shannon entropy threshold; lower catches more |
| `--json` | -j | bool | Append metadata trailer after output |
| `--quiet` | -q | bool | Suppress stderr output |

