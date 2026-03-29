---
title: "vrk slug"
description: "URL/filename slug generator - --separator, --max, --json"
tool: slug
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → slug → stdout`

Exit 0 Success · Exit 1 Runtime error (I/O failure) · Exit 2 Interactive TTY with no stdin

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--separator` |   | string | Word separator character or string |
| `--max` |   | int | Max output length; truncated at last separator (0 = no limit) |
| `--json` | -j | bool | Emit JSON per line: {input, output} |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
echo 'My Article Title' | vrk slug
```
