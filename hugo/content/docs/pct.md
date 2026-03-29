---
title: "vrk pct"
description: "percent encoder/decoder - RFC 3986, --encode, --decode, --form"
tool: pct
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Percent-encodes and decodes strings following RFC 3986. Handles the difference between path encoding (spaces become %20) and form encoding (spaces become +) so you don't have to remember which is which. Processes one result per line for batch use.

## The problem

You build a URL with a query parameter that contains spaces and special characters. You use printf with manual %20 substitution, miss an ampersand, and the API returns a 400. The rules for path encoding vs form encoding differ and you always pick the wrong one.

## Before and after

**Before**

```bash
QUERY=$(python3 -c "import urllib.parse; print(urllib.parse.quote_plus('$USER_INPUT'))")
curl "https://api.example.com/search?q=$QUERY"
# quote() for paths, quote_plus() for forms - easy to mix up
# breaks if $USER_INPUT contains single quotes
```

**After**

```bash
QUERY=$(echo "$USER_INPUT" | vrk pct --encode --form)
curl "https://api.example.com/search?q=$QUERY"
```

## Example

```bash
echo 'hello world' | vrk pct --encode
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Invalid percent sequence during decode, I/O error |
| 2 | Neither --encode nor --decode specified, both specified, interactive TTY |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--encode` |   | bool | Percent-encode input (RFC 3986 unless --form) |
| `--decode` |   | bool | Percent-decode input |
| `--form` |   | bool | Use application/x-www-form-urlencoded rules (spaces / +) |
| `--json` | -j | bool | Emit JSON per line: {input, output, op, mode} |
| `--quiet` | -q | bool | Suppress stderr output |

