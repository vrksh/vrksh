---
title: "vrk links"
description: "hyperlink extractor - markdown, HTML, bare URLs to JSONL"
tool: links
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → links → stdout`

Exit 0 Success, including empty input and documents with no links · Exit 1 I/O error reading stdin · Exit 2 Interactive TTY with no piped input, unknown flag

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--bare` | -b | bool | Output URLs only, one per line |
| `--json` | -j | bool | Append metadata trailer after all records |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
cat README.md | vrk links
```
