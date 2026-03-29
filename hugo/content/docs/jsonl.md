---
title: "vrk jsonl"
description: "JSON array to JSONL converter - --collect, --json"
tool: jsonl
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → jsonl → stdout`

Exit 0 Success, including empty input · Exit 1 Invalid JSON, I/O error · Exit 2 Interactive TTY with no input, unknown flag

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--collect` | -c | bool | Collect JSONL lines into a JSON array |
| `--json` | -j | bool | Append metadata trailer after all records (split mode only) |

## Example

```bash
cat data.json | vrk jsonl
```
