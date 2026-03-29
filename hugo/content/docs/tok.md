---
title: "vrk tok"
description: "token counter - cl100k_base, --budget guard, --json."
tool: tok
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Counts tokens in your text using the cl100k_base tokenizer. Exact for GPT-4 and roughly 95% accurate for Claude. Use it as a guard before sending prompts to an LLM - if the input exceeds your token budget, tok exits 1 and your pipeline stops before the API call.

## The problem

LLM APIs silently truncate prompts that exceed the context window. You get a degraded response with no error. There is no built-in way to check token count before you send.

## Before and after

**Before**

```bash
python3 -c "
import tiktoken
enc = tiktoken.get_encoding('cl100k_base')
print(len(enc.encode(open('prompt.txt').read())))
"
```

**After**

```bash
cat prompt.txt | vrk tok --budget 4000
```

## Example

```bash
cat prompt.txt | vrk tok --budget 4000
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, within budget |
| 1 | Over budget or I/O error |
| 2 | Usage error - no input, unknown flag |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | -j | bool | Emit JSON with token count and metadata |
| `--budget` |   | int | Exit 1 if token count exceeds N |
| `--model` | -m | string | Tokenizer model (currently cl100k_base only) |
| `--quiet` | -q | bool | Suppress stderr output |

