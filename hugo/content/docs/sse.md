---
title: "vrk sse"
description: "SSE stream parser - text/event-stream to JSONL"
tool: sse
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Parses Server-Sent Event streams into clean JSONL records. Handles the data: prefixes, blank-line delimiters, multi-line data fields, and [DONE] sentinels that make raw SSE awkward to work with. You can extract nested fields from the JSON data using dot-path syntax.

## The problem

You curl an LLM streaming endpoint and get raw SSE frames - data: prefixes, blank lines, [DONE] sentinels. You need clean JSONL records. You write an awk script that breaks on multi-line data fields.

## Before and after

**Before**

```bash
curl -sN https://api.example.com/stream | \
  grep '^data: ' | sed 's/^data: //' | grep -v '^\[DONE\]$'
# breaks on multi-line data fields
# loses event types and IDs
```

**After**

```bash
curl -sN https://api.example.com/stream | vrk sse
```

## Example

```bash
curl -sN https://api.example.com/stream | vrk sse
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, including clean [DONE] termination |
| 1 | I/O error reading stdin |
| 2 | Usage error - interactive terminal with no piped input, unknown flag |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--event` | -e | string | Only emit events of this type |
| `--field` | -F | string | Extract dot-path field from record as plain text |

