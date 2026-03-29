---
title: "vrk emit"
description: "structured logger - wraps stdin lines as JSONL log records"
tool: emit
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → emit → stdout`

Exit 0 All non-empty lines emitted as JSONL records · Exit 1 Stdin scanner error or write failure · Exit 2 Interactive TTY with no positional arg, or unknown --level value

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--level` | -l | string | Log level: debug, info, warn, error |
| `--tag` |   | string | Value for the tag field on every record |
| `--msg` |   | string | Fixed message override; stdin lines parsed as JSON and merged |
| `--parse-level` |   | bool | Auto-detect level from ERROR/WARN/INFO/DEBUG line prefixes |

## Example

```bash
some-script.sh | vrk emit --level info --tag my-script
```
