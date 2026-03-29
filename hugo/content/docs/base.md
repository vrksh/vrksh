---
title: "vrk base"
description: "Encode and decode base64, base64url, hex, and base32 from the command line. No more openssl flags."
og_title: "vrk base - base64, hex, and base32 encoding in one tool"
tool: base
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Encodes and decodes between base64, base64url, hex, and base32. Works identically on macOS and Linux - no more remembering whether it is base64 -D or base64 -d. Two subcommands: encode and decode.

## The problem

You write a deploy script that base64-decodes a secret on macOS. It works. CI runs on Linux and fails because macOS uses base64 -D while Linux uses base64 -d. You add an OS check and now your three-line decode is twelve lines.

## Before and after

**Before**

```bash
if [[ "$OSTYPE" == "darwin"* ]]; then
  echo "$ENCODED" | base64 -D
else
  echo "$ENCODED" | base64 -d
fi
```

**After**

```bash
echo "$ENCODED" | vrk base decode --from base64
```

## Example

```bash
echo 'hello' | vrk base encode --to base64
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Invalid encoded input (bad characters, wrong padding) |
| 2 | Missing subcommand, --to/--from missing or unsupported |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--to` |   | string | Target encoding: base64, base64url, hex, base32 (encode subcommand) |
| `--from` |   | string | Source encoding: base64, base64url, hex, base32 (decode subcommand) |
| `--quiet` | -q | bool | Suppress stderr output |

