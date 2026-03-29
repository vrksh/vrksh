---
title: "vrk sse"
description: "SSE stream parser - text/event-stream to JSONL"
tool: sse
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → sse → stdout`

Exit 0 Success, including clean [DONE] termination · Exit 1 I/O error reading stdin · Exit 2 Usage error - interactive terminal with no piped input, unknown flag

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--event` | -e | string | Only emit events of this type |
| `--field` | -F | string | Extract dot-path field from record as plain text |

## Example

```bash
curl -sN https://api.example.com/stream | vrk sse
```
