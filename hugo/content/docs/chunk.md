---
title: "vrk chunk"
description: "token-aware text splitter - JSONL chunks within a token budget"
tool: chunk
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → chunk → stdout`

Exit 0 Success, including empty input · Exit 1 I/O error · Exit 2 No input, --size missing or < 1, --overlap >= --size, unknown flag

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--size` |   | int | Max tokens per chunk (required) |
| `--overlap` |   | int | Token overlap between adjacent chunks |
| `--by` |   | string | Chunking strategy: paragraph |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
cat long-doc.md | vrk chunk --size 4000
```
