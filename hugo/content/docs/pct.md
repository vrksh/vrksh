---
title: "vrk pct"
description: "Percent encoder/decoder - RFC 3986, --encode, --decode, --form"
tool: pct
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You're building URLs with query parameters, decoding form data from a webhook payload, or dealing with percent-encoded paths in a log file. Python's `urllib.parse.quote` is always the wrong import from the wrong submodule, bash has no built-in for this, and `curl --data-urlencode` only helps if you're making a request. You need a composable, scriptable tool that handles RFC 3986 encoding correctly - including multi-byte UTF-8 characters - without writing a throwaway script every time.

`pct` encodes or decodes one line at a time. RFC 3986 mode passes through only unreserved characters (`[A-Za-z0-9\-_.~]`); everything else is percent-encoded. Form mode (`--form`) uses `application/x-www-form-urlencoded` rules, where spaces become `+` instead of `%20`.

## The fix

```bash
echo "hello world & goodbye" | vrk pct --encode
```

<!-- output: verify against binary -->

```
hello%20world%20%26%20goodbye
```

## Walkthrough

### Encoding a query parameter value

```bash
echo "search query with spaces" | vrk pct --encode
echo "hello+world" | vrk pct --decode
```

<!-- output: verify against binary -->

By default, encoding follows RFC 3986 - spaces become `%20`. Reserved characters like `&`, `=`, and `?` are encoded so they're safe to embed as a query parameter value.

### Form encoding (spaces as +)

```bash
echo "hello world" | vrk pct --encode --form
echo "hello+world" | vrk pct --decode --form
```

<!-- output: verify against binary -->

`--form` switches to `application/x-www-form-urlencoded` rules. Spaces become `+`, and `+` in the input decodes back to a space. Use this for form POST bodies, not for URL path segments.

### Unicode input

```bash
echo "café au lait" | vrk pct --encode
echo "日本語" | vrk pct --encode
```

<!-- output: verify against binary -->

Multi-byte UTF-8 characters are encoded byte by byte. Each byte becomes its own `%XX` sequence, which is the correct behavior per RFC 3987.

### Decoding a URL-encoded string

```bash
echo "hello%20world%21" | vrk pct --decode
echo "search%3Fq%3Dhello%20world" | vrk pct --decode
```

<!-- output: verify against binary -->

Decoding is the exact inverse of encoding. Invalid percent sequences (like `%GG` or a truncated `%2`) cause an exit 1 with an error message identifying the bad sequence.

### JSON output

```bash
echo "hello world" | vrk pct --encode --json
```

<!-- output: verify against binary -->

```json
{"input":"hello world","output":"hello%20world","op":"encode","mode":"rfc3986"}
```

The `op` field is always `"encode"` or `"decode"`. The `mode` field is `"rfc3986"` or `"form"`. Useful when auditing a batch of values and you need a traceable record of what changed.

### Batch processing

```bash
cat urls.txt | vrk pct --decode
```

One result per line. Blank lines pass through unchanged.

## Pipeline example

Build a URL with encoded query parameters:

```bash
QUERY="what is the meaning of life & everything?"
BASE="https://example.com/search?q="
ENCODED=$(echo "$QUERY" | vrk pct --encode --form)
echo "${BASE}${ENCODED}"
```

Decode a stream of form-encoded webhook payloads:

```bash
cat webhooks.log \
  | grep "^body:" \
  | cut -d' ' -f2 \
  | vrk pct --decode --form \
  | jq -r '.'
```

Or encode a list of user-submitted filenames before embedding them in a URL:

```bash
cat filenames.txt | vrk pct --encode | sed 's|^|https://cdn.example.com/files/|'
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--encode` | | bool | `false` | Percent-encode input (RFC 3986 unless `--form`) |
| `--decode` | | bool | `false` | Percent-decode input |
| `--form` | | bool | `false` | Use `application/x-www-form-urlencoded` rules (spaces ↔ `+`) |
| `--json` | `-j` | bool | `false` | Emit JSON per line: `{input, output, op, mode}` |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error - invalid percent sequence during decode, I/O error |
| 2 | Usage error - neither `--encode` nor `--decode` specified, both specified together, interactive TTY with no stdin |
