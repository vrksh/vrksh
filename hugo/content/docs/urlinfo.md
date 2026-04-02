---
title: "vrk urlinfo"
description: "Parse URLs and extract components. Get scheme, host, path, query params, or any field individually."
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

`vrk urlinfo` parses URLs into structured JSON components: scheme, host, port, path, query parameters, fragment, and user. `--field` with dot-path syntax extracts a single component. Handles edge cases that regex approaches miss. No network calls, pure parsing.

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

### Full JSON output

```bash
$ vrk urlinfo 'https://api.example.com:8080/v1/search?q=llm+tools&limit=10#results'
{"scheme":"https","host":"api.example.com","port":8080,"path":"/v1/search","query":{"limit":"10","q":"llm tools"},"fragment":"results","user":""}
```

Every URL component is extracted into a structured JSON object. Query parameters are parsed into a nested object.

### Extract a single field (--field)

```bash
$ vrk urlinfo --field host 'https://api.example.com:8080/v1/search'
api.example.com

$ vrk urlinfo --field path 'https://api.example.com:8080/v1/search'
/v1/search

$ vrk urlinfo --field query.q 'https://api.example.com?q=llm+tools'
llm tools
```

Dot-path syntax reaches into query parameters: `query.q`, `query.page`, etc.

### Batch processing

```bash
$ printf 'https://a.com/path\nhttps://b.com/other\n' | vrk urlinfo --field host
a.com
b.com
```

Processes multiple URLs, one per line.

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

### Parse links from a web page by domain

```bash
# Grab a page, extract links, group by domain
vrk grab https://example.com | vrk links --bare | \
  while IFS= read -r url; do
    vrk urlinfo --field host "$url"
  done | sort | uniq -c | sort -rn
```

## When it fails

Invalid URL:

```bash
$ vrk urlinfo 'not a url'
error: urlinfo: invalid URL
$ echo $?
1
```

No input:

```bash
$ vrk urlinfo
usage error: urlinfo: no URL provided
$ echo $?
2
```
