---
title: "vrk jwt"
description: "JWT inspector — decode, --claim, --expired, --valid."
tool: jwt
group: v1
mcp_callable: true
noindex: false
---

<!-- generated — do not edit below this line -->

## Contract

`stdin → jwt → stdout`

Exit 0 Success or token is valid · Exit 1 Token expired/invalid or runtime error · Exit 2 Usage error — bad format, too many args

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | -j | bool | Emit decoded token as JSON |
| `--claim` | -c | string | Print value of a single claim |
| `--expired` | -e | bool | Exit 1 if the token is expired |
| `--valid` |   | bool | Exit 1 if expired, not-yet-valid, or issued in the future |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
vrk jwt --claim sub eyJhbGciOiJIUzI1NiJ9...
```
