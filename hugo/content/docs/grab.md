---
title: "vrk grab"
description: "URL fetcher - clean markdown, plain text, or raw HTML."
tool: grab
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

You need to feed a web page to an LLM. Fetching HTML gives you 80% noise --
nav, footer, scripts, cookie banners. Stripping HTML with sed is fragile
and different for every site. There is no Unix tool that fetches a URL and
returns just the content.

## The fix

```bash
$ vrk grab https://example.com
# Example Domain

This domain is for use in documentation examples without needing permission. Avoid use in operations.

[Learn more](https://iana.org/domains/example)
```

The output is clean markdown -- article text only, no navigation, no
scripts, no boilerplate. Ready to pipe into `tok`, `prompt`, or any
tool that takes text.

When the URL is unreachable or returns an error:

```
error: grab: HTTP 404
```

Exit 1. The pipeline stops.

## Plain text for LLM input

```bash
$ vrk grab --text https://example.com
Example Domain

This domain is for use in documentation examples without needing permission. Avoid use in operations.

Learn more
```

The `--text` flag strips markdown syntax too -- no headings, no link
brackets, no emphasis markers. Pure prose. Use this when passing content
to an LLM that shouldn't see formatting characters.

## Full pipeline

```bash
CONTENT=$(vrk grab https://example.com/article)
echo "$CONTENT" | vrk tok --budget 8000 \
  && echo "$CONTENT" | vrk prompt --system "Summarize in 3 bullet points."
```

Capture the content in a variable first to avoid fetching the URL twice.
Fetching twice is wasteful and can return different content if the page
changes between requests.

Extract all links from a remote page:

```bash
vrk grab https://example.com | vrk links --bare
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--text` | `-t` | bool | `false` | Plain prose output, no markdown syntax |
| `--raw` | | bool | `false` | Raw HTML, no processing |
| `--json` | `-j` | bool | `false` | Emit JSON envelope with metadata |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | HTTP error, fetch timeout, or I/O error |
| 2 | Usage error - invalid URL, no input, mutually exclusive flags |
