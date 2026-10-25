---
title: "vrk pct"
description: "vrk pct percent-encodes and decodes text per RFC 3986. Use --form for HTML form encoding, where spaces become +, and --decode to reverse it."
meta_lead: "vrk pct percent-encodes and decodes text per RFC 3986 for URL-safe strings."
og_title: "vrk pct - RFC 3986 percent encoding and decoding"
tool: pct
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk pct percent-encodes and decodes text per RFC 3986 for URL-safe strings.

## The problem

A URL parameter contains spaces and ampersands. Python's `quote()` encodes spaces as `%20`, `quote_plus()` encodes them as `+`. The difference matters: the API expects path encoding, not form encoding. `curl --data-urlencode` encodes the whole value but you only need the encoded string by itself.

## The solution

`vrk pct` percent-encodes and decodes text per RFC 3986. `--encode` gives path-safe encoding (spaces become `%20`). `--form` switches to HTML form encoding (spaces become `+`). It converts input line by line, so you can convert a batch.

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

| Flag       | Short | Type | Description                                                                  |
|------------|-------|------|------------------------------------------------------------------------------|
| `--encode` |       | bool | Percent-encode input (RFC 3986 unless --form)                                |
| `--decode` |       | bool | Percent-decode input                                                         |
| `--form`   |       | bool | Use application/x-www-form-urlencoded rules (spaces / +)                     |
| `--json`   | -j    | bool | Emit JSON per line: {input, output, op, mode}, where mode is percent or form |
| `--quiet`  | -q    | bool | Suppress stderr output                                                       |


<!-- notes - edit in notes/pct.notes.md -->

## How it works

`vrk pct` percent-encodes and decodes text, as defined in RFC 3986, so you can build safe URLs in a pipeline.

### Encode (default)

```bash
$ echo 'hello world' | vrk pct --encode
hello%20world

$ echo 'key=value&other=data' | vrk pct --encode
key%3Dvalue%26other%3Ddata
```

A space becomes `%20`. Every RFC 3986 reserved character is encoded.

### Encode (form mode)

```bash
$ echo 'hello world' | vrk pct --encode --form
hello+world
```

A space becomes `+` instead of `%20`. Use this for HTML form data and `application/x-www-form-urlencoded` content.

### Decode

```bash
$ echo 'hello%20world' | vrk pct --decode
hello world

$ echo 'hello+world' | vrk pct --decode --form
hello world
```

Without `--form`, `+` stays as `+`. With `--form`, `+` decodes to a space.

### Batch processing

pct reads the input line by line:

```bash
$ printf 'hello world\nfoo bar\n' | vrk pct --encode
hello%20world
foo%20bar
```

### JSON output

```bash
$ echo 'hello world' | vrk pct --encode --json
{"input":"hello world","output":"hello%20world","op":"encode","mode":"percent"}
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

If you pass both `--encode` and `--decode`, pct exits 2:

```bash
$ echo 'test' | vrk pct --encode --decode
usage error: pct: flags --encode and --decode are mutually exclusive
$ echo $?
2
```

If you pass neither flag, pct exits 2:

```bash
$ echo 'test' | vrk pct
usage error: pct: either --encode or --decode is required
$ echo $?
2
```

If the input has an invalid percent sequence, pct exits 1:

```bash
$ echo '%ZZ' | vrk pct --decode
error: pct: invalid percent-encoded input: invalid URL escape "%ZZ"
$ echo $?
1
```
