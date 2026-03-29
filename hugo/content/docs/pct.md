---
title: "vrk pct"
description: "percent encoder/decoder - RFC 3986, --encode, --decode, --form"
tool: pct
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → pct → stdout`

Exit 0 Success · Exit 1 Invalid percent sequence during decode, I/O error · Exit 2 Neither --encode nor --decode specified, both specified, interactive TTY

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--encode` |   | bool | Percent-encode input (RFC 3986 unless --form) |
| `--decode` |   | bool | Percent-decode input |
| `--form` |   | bool | Use application/x-www-form-urlencoded rules (spaces / +) |
| `--json` | -j | bool | Emit JSON per line: {input, output, op, mode} |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
echo 'hello world' | vrk pct --encode
```
