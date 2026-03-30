---
title: "vrk pct"
description: "Percent-encode and decode strings per RFC 3986. Handles URL components and form data correctly."
og_title: "vrk pct - RFC 3986 percent encoding and decoding"
tool: pct
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

You need to percent-encode a URL parameter. `curl --data-urlencode` encodes the whole value but you just need the encoded string. You reach for Python's urllib.parse.quote and it encodes spaces as `+` in some modes and `%20` in others. The difference between path encoding and form encoding matters, and most tools don't let you choose.

`vrk pct` percent-encodes and decodes text per RFC 3986. Use `--encode` for path-safe encoding (spaces become `%20`). Add `--form` for HTML form encoding (spaces become `+`). Processes line by line for batch conversion.

## The problem

You're building a URL with query parameters in a shell script. A parameter value contains spaces and ampersands. You use Python's quote() but forget the difference between quote() and quote_plus(). Spaces become + instead of %20. The API rejects the request because it expects path encoding, not form encoding.

## Before and after

**Before**

```bash
python3 -c "from urllib.parse import quote; print(quote('hello world'))"
# Pulls in Python runtime for a one-line operation
```

**After**

```bash
echo 'hello world' | vrk pct --encode
```

## Example

```bash
echo 'hello world&foo=bar' | vrk pct --encode
```

## Exit codes

| Code | Meaning                                                                  |
|------|--------------------------------------------------------------------------|
| 0    | Success                                                                  |
| 1    | Invalid percent sequence during decode, I/O error                        |
| 2    | Neither --encode nor --decode specified, both specified, interactive TTY |

## Flags

| Flag       | Short | Type | Description                                              |
|------------|-------|------|----------------------------------------------------------|
| `--encode` |       | bool | Percent-encode input (RFC 3986 unless --form)            |
| `--decode` |       | bool | Percent-decode input                                     |
| `--form`   |       | bool | Use application/x-www-form-urlencoded rules (spaces / +) |
| `--json`   | -j    | bool | Emit JSON per line: {input, output, op, mode}            |
| `--quiet`  | -q    | bool | Suppress stderr output                                   |


<!-- notes - edit in notes/pct.notes.md -->

## How it works

### Encode (path mode, default)

```bash
$ echo 'hello world' | vrk pct --encode
hello%20world

$ echo 'key=value&other=data' | vrk pct --encode
key%3Dvalue%26other%3Ddata
```

Spaces become `%20`. All RFC 3986 reserved characters are encoded.

### Encode (form mode)

```bash
$ echo 'hello world' | vrk pct --encode --form
hello+world
```

Spaces become `+` instead of `%20`. Use this for HTML form data and `application/x-www-form-urlencoded` content.

### Decode

```bash
$ echo 'hello%20world' | vrk pct --decode
hello world

$ echo 'hello+world' | vrk pct --decode --form
hello world
```

Without `--form`, `+` is left as-is. With `--form`, `+` decodes to a space.

### Batch processing

Processes line by line:

```bash
$ printf 'hello world\nfoo bar\n' | vrk pct --encode
hello%20world
foo%20bar
```

### JSON output

```bash
$ echo 'hello world' | vrk pct --encode --json
{"input":"hello world","output":"hello%20world","op":"encode","mode":"path"}
```

## Pipeline integration

### Build a URL with encoded parameters

```bash
QUERY=$(echo "$SEARCH_TERM" | vrk pct --encode)
vrk grab "https://api.example.com/search?q=$QUERY" | vrk prompt --system 'Summarize'
```

### Decode URL components

```bash
# Parse a URL and decode the query parameter
vrk urlinfo --field query.q 'https://example.com?q=hello%20world' | vrk pct --decode
```

## When it fails

Both --encode and --decode:

```bash
$ echo 'test' | vrk pct --encode --decode
usage error: pct: specify --encode or --decode, not both
$ echo $?
2
```

Neither flag:

```bash
$ echo 'test' | vrk pct
usage error: pct: specify --encode or --decode
$ echo $?
2
```

Invalid percent sequence:

```bash
$ echo '%ZZ' | vrk pct --decode
error: pct: invalid percent-encoding
$ echo $?
1
```
