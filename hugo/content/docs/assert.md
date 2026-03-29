---
title: "vrk assert"
description: "pipeline condition check - jq conditions, --contains, --matches"
tool: assert
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → assert → stdout`

Exit 0 All conditions passed; input passed through to stdout · Exit 1 Assertion failed, or runtime error · Exit 2 No condition specified, mixed modes, invalid regex, interactive TTY

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--contains` |   | string | Assert stdin contains this literal substring |
| `--matches` |   | string | Assert stdin matches this regular expression |
| `--message` | -m | string | Custom message on failure |
| `--json` | -j | bool | Emit errors as JSON to stdout |
| `--quiet` | -q | bool | Suppress stderr output on failure |

## Example

```bash
vrk grab https://api.example.com/health | vrk assert --contains '"status":"ok"'
```
