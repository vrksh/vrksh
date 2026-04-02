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

## The problem

`base64` behaves differently on macOS and Linux. macOS wraps output at 76 characters. `-w0` disables wrapping on Linux but does not exist on macOS. `-d` means decode on macOS, `--decode` on Linux. A script that works locally breaks in CI on a different OS.

## The solution

`vrk base` encodes and decodes base64, base64url, hex, and base32 with identical behavior on every platform. Two subcommands: `encode` and `decode`. Strips one trailing newline from stdin so `echo` input works correctly. No platform-specific flags to remember.

## Before and after

**Before**

```bash
# macOS
echo 'hello' | base64
# Linux
echo 'hello' | base64 -w0
# Different flags, different output
```

**After**

```bash
echo 'hello' | vrk base encode --to base64
```

## Example

```bash
echo 'hello' | vrk base encode --to base64
```

## Exit codes

| Code | Meaning                                                |
|------|--------------------------------------------------------|
| 0    | Success                                                |
| 1    | Invalid encoded input (bad characters, wrong padding)  |
| 2    | Missing subcommand, --to/--from missing or unsupported |

## Flags

| Flag      | Short | Type   | Description                                                         |
|-----------|-------|--------|---------------------------------------------------------------------|
| `--to`    |       | string | Target encoding: base64, base64url, hex, base32 (encode subcommand) |
| `--from`  |       | string | Source encoding: base64, base64url, hex, base32 (decode subcommand) |
| `--quiet` | -q    | bool   | Suppress stderr output                                              |


<!-- notes - edit in notes/base.notes.md -->

## Supported encodings

| Encoding | Flag value | Example output |
|----------|-----------|----------------|
| Base64 | `base64` | `aGVsbG8=` |
| Base64url | `base64url` | `aGVsbG8` (no padding, URL-safe) |
| Hex | `hex` | `68656c6c6f` |
| Base32 | `base32` | `NBSWY3DP` |

## How it works

### Encode

```bash
$ echo 'hello' | vrk base encode --to base64
aGVsbG8=

$ echo 'hello' | vrk base encode --to hex
68656c6c6f

$ echo 'hello' | vrk base encode --to base64url
aGVsbG8
```

### Decode

```bash
$ echo 'aGVsbG8=' | vrk base decode --from base64
hello

$ echo '68656c6c6f' | vrk base decode --from hex
hello
```

### Trailing newline handling

`echo` appends a newline. vrk base strips exactly one trailing newline before encoding, so `echo 'hello'` and `printf 'hello'` produce the same result. Use `printf 'hello\n'` if you want the newline included.

## Pipeline integration

### Decode a base64 field from JSON

```bash
echo "$JWT_PAYLOAD" | vrk base decode --from base64url | jq .
```

### Encode content for embedding in JSON

```bash
ENCODED=$(cat binary-file | vrk base encode --to base64)
echo "{\"data\":\"$ENCODED\"}" | vrk kv set --ns cache payload
```

## When it fails

Invalid base64 input:

```bash
$ echo '!!invalid!!' | vrk base decode --from base64
error: base: illegal base64 data at input byte 0
$ echo $?
1
```

Missing encoding flag:

```bash
$ echo 'hello' | vrk base encode
usage error: base encode: --to is required
$ echo $?
2
```
