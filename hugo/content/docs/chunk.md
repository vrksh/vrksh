---
title: "vrk chunk"
description: "token-aware text splitter - JSONL chunks within a token budget"
tool: chunk
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Splits long documents into chunks that fit within an LLM's context window. Each chunk comes out as a JSONL record with its index, text, and exact token count. Splits at paragraph boundaries when possible and supports overlap so you don't lose context at the edges.

## The problem

You need to send a long document to an LLM but it exceeds the context window. You split on line count or character count, but neither maps to tokens. Chunks end up too large or too small, and you lose context at split boundaries.

## Before and after

**Before**

```bash
split -l 100 document.txt chunk_
# no idea how many tokens each chunk is
# no overlap for context continuity
```

**After**

```bash
cat document.txt | vrk chunk --size 4000 --overlap 200
```

## Example

```bash
cat long-doc.md | vrk chunk --size 4000
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, including empty input |
| 1 | I/O error |
| 2 | No input, --size missing or < 1, --overlap >= --size, unknown flag |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--size` |   | int | Max tokens per chunk (required) |
| `--overlap` |   | int | Token overlap between adjacent chunks |
| `--by` |   | string | Chunking strategy: paragraph |
| `--quiet` | -q | bool | Suppress stderr output |

