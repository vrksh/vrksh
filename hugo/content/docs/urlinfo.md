---
title: "vrk urlinfo"
description: "URL parser - scheme, host, port, path, query, --field"
tool: urlinfo
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → urlinfo → stdout`

Exit 0 Success · Exit 1 Invalid URL that cannot be parsed, I/O error · Exit 2 Interactive TTY with no stdin or positional arg

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--field` | -F | string | Extract a single field as plain text (supports dot-path for query params) |
| `--json` | -j | bool | Append metadata trailer |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
echo 'https://api.example.com:8443/v2/users?page=2' | vrk urlinfo
```
