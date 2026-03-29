---
title: "vrk tok"
description: "Count tokens. Gate pipelines before they fail."
tool: tok
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Token counter and pipeline gate. Uses cl100k_base (~95% accurate for Claude).
Without --check: pure measurement, always exits 0.
With --check N: passes input through if within N tokens, exits 1 with empty
stdout if over. The sentinel before any LLM call.

## Example

```bash
cat prompt.txt | vrk tok --check 8000 | vrk prompt
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Measurement success; or --check within limit |
| 1 | --check over limit; I/O error; tokenizer error |
| 2 | Usage error — unknown flag, no stdin, --check without value |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--check` |   | int | Pass input through if ≤N tokens; exit 1 with empty stdout if over |
| `--model` |   | string | Tokenizer — cl100k_base (default) |
| `--json` | -j | bool | Emit JSON (measurement) or JSON error (gate). Does not wrap passthrough. |
| `--quiet` | -q | bool | Suppress stderr on failure |

