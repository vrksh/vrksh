---
title: "vrk jwt"
description: "Decode JWTs and extract claims from the command line. Check expiry, validate structure, pull fields."
og_title: "vrk jwt - JWT decoder and claim inspector"
tool: jwt
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Decodes JWT tokens and shows you what is inside - without sending them to a third-party website. You can extract a single claim, check whether the token is expired, or validate all time-based claims at once.

## The problem

You have a JWT from an API response and need to check what is in it. You paste it into jwt.io (leaking the token to a third party) or write a Python one-liner that breaks on padding. You just want to see the claims.

## Before and after

**Before**

```bash
python3 -c "
import base64, json, sys
token = sys.argv[1].split('.')[1]
token += '=' * (-len(token) % 4)
print(json.dumps(json.loads(base64.urlsafe_b64decode(token)), indent=2))
" eyJhbGciOi...
```

**After**

```bash
vrk jwt eyJhbGciOi...
```

## Example

```bash
vrk jwt --claim sub eyJhbGciOiJIUzI1NiJ9...
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success or token is valid |
| 1 | Token expired/invalid or runtime error |
| 2 | Usage error - bad format, too many args |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | -j | bool | Emit decoded token as JSON |
| `--claim` | -c | string | Print value of a single claim |
| `--expired` | -e | bool | Exit 1 if the token is expired |
| `--valid` |   | bool | Exit 1 if expired, not-yet-valid, or issued in the future |
| `--quiet` | -q | bool | Suppress stderr output |

