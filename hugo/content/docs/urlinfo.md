---
title: "vrk urlinfo"
description: "vrk urlinfo parses a URL into scheme, host, port, path, query, and fragment as JSON. Use --field to print one component."
meta_lead: "vrk urlinfo parses URLs into structured JSON components with dot-path field extraction."
og_title: "vrk urlinfo - URL parser and component extractor"
tool: urlinfo
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk urlinfo parses URLs into structured JSON components with dot-path field extraction.

## The problem

`cut -d'/' -f3` extracts a hostname until a URL has a port number or basic auth. `https://api.example.com:8080/path` returns "api.example.com:8080" including the port. A URL with `user:pass@host` breaks the regex entirely. The 10% of URLs that don't fit a simple pattern break the pipeline.

## The solution

`vrk urlinfo` parses URLs into structured JSON components: scheme, host, port, path, query parameters, fragment, and user. Use `--field` with a dot-path to extract one component. It handles edge cases that regex approaches miss. It makes no network calls.

## Before and after

**Before**

```bash
echo 'https://api.example.com:8080/path?q=test' | cut -d'/' -f3
# Returns "api.example.com:8080" - includes port
```

**After**

```bash
vrk urlinfo --field host 'https://api.example.com:8080/path?q=test'
```

## Example

```bash
vrk urlinfo 'https://api.example.com:8080/v1/search?q=llm+tools&limit=10'
```

## Exit codes

| Code | Meaning                                         |
|------|-------------------------------------------------|
| 0    | Success                                         |
| 1    | Invalid URL that cannot be parsed, I/O error    |
| 2    | Interactive TTY with no stdin or positional arg |

## Flags

| Flag      | Short | Type   | Description                                                               |
|-----------|-------|--------|---------------------------------------------------------------------------|
| `--field` | -F    | string | Extract a single field as plain text (supports dot-path for query params) |
| `--json`  | -j    | bool   | Append metadata trailer                                                   |
| `--quiet` | -q    | bool   | Suppress stderr output                                                    |


<!-- notes - edit in notes/urlinfo.notes.md -->

## How it works

`vrk urlinfo` splits a URL into its parts (scheme, host, port, path, query, fragment, user) and prints them as JSON.

### Full JSON output

```bash
$ vrk urlinfo 'https://api.example.com:8080/v1/search?q=llm+tools&limit=10#results'
{"scheme":"https","host":"api.example.com","port":8080,"path":"/v1/search","query":{"limit":"10","q":"llm tools"},"fragment":"results","user":""}
```

urlinfo puts every URL part in one JSON object. It parses the query parameters into a nested object.

### Extract a single field (--field)

```bash
$ vrk urlinfo --field host 'https://api.example.com:8080/v1/search'
api.example.com

$ vrk urlinfo --field path 'https://api.example.com:8080/v1/search'
/v1/search

$ vrk urlinfo --field query.q 'https://api.example.com?q=llm+tools'
llm tools
```

A dot path reaches a query parameter, for example `query.q` or `query.page`.

### Batch processing

```bash
$ printf 'https://a.com/path\nhttps://b.com/other\n' | vrk urlinfo --field host
a.com
b.com
```

urlinfo reads one URL per line.

### JSON metadata (--json)

```bash
$ printf 'https://a.com\nhttps://b.com\n' | vrk urlinfo --json
{"scheme":"https","host":"a.com",...}
{"scheme":"https","host":"b.com",...}
{"_vrk":"urlinfo","count":2}
```

## Available fields

| Field | Example value |
|-------|--------------|
| `scheme` | `https` |
| `host` | `api.example.com` |
| `port` | `8080` (0 if not specified) |
| `path` | `/v1/search` |
| `query` | `{"q":"llm tools","limit":"10"}` |
| `query.<key>` | value of a specific parameter |
| `fragment` | `results` |
| `user` | username (from `user@host` URLs) |

## Pipeline integration

### Group URLs by domain

```bash
# Extract unique domains from a list of URLs
cat urls.txt | while IFS= read -r url; do
  vrk urlinfo --field host "$url"
done | sort -u
```

### Extract and decode query parameters

```bash
# Get a query parameter and decode it
vrk urlinfo --field query.q 'https://example.com?q=hello%20world' | vrk pct --decode
```

### Count the links on a web page by domain

```bash
# Grab a page, extract links, group by domain
vrk grab https://example.com | vrk links --bare | \
  while IFS= read -r url; do
    vrk urlinfo --field host "$url"
  done | sort | uniq -c | sort -rn
```

## When it fails

If the URL is invalid, urlinfo exits 1:

```bash
$ vrk urlinfo 'not a url'
error: urlinfo: invalid URL: not a url
$ echo $?
1
```

If you give no input on an interactive terminal, urlinfo exits 2:

<!-- doccheck: skip (needs an interactive terminal, doccheck runs with empty stdin) -->
```bash
$ vrk urlinfo
usage error: urlinfo: no input: pipe a URL to stdin
$ echo $?
2
```
