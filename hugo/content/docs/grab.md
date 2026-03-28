---
title: "vrk grab"
description: "URL fetcher — clean markdown, plain text, or raw HTML."
tool: grab
group: v1
mcp_callable: true
noindex: false
---

<!-- generated — do not edit below this line -->

## Contract

`stdin → grab → stdout`

Exit 0 Success · Exit 1 HTTP error, fetch timeout, or I/O error · Exit 2 Usage error — invalid URL, no input, mutually exclusive flags

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--text` | -t | bool | Plain prose output, no markdown syntax |
| `--raw` |   | bool | Raw HTML, no processing |
| `--json` | -j | bool | Emit JSON envelope with metadata |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
vrk grab --text https://example.com/article
```
