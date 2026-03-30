---
title: "vrk grab"
description: "Fetch any URL and get clean markdown back. No BeautifulSoup, no parsing scripts."
og_title: "vrk grab - fetch any URL as clean markdown"
tool: grab
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

You need the text of a web page for an LLM pipeline. You curl the URL and get 47KB of HTML - navigation bars, ad scripts, cookie banners, tracking pixels, and somewhere buried in the middle, the 2KB article you actually wanted. You write a Python script with BeautifulSoup. It works on one site. The next site uses different class names.

`vrk grab` fetches any URL and returns clean, readable markdown. It applies Readability-style content extraction to strip navigation, ads, scripts, and boilerplate. Use `--text` for plain prose with no formatting, or `--raw` for the unprocessed HTML. One command replaces curl + BeautifulSoup + custom extraction logic.

## The problem

You curl a documentation page to feed to an LLM for summarization. The HTML is 52KB. The actual article content is 3KB. The other 49KB is navigation, JavaScript, CSS, footers, and cookie consent dialogs. Your LLM wastes most of its context window on boilerplate and returns a summary of the navigation menu.

## Before and after

**Before**

```bash
curl -s https://example.com/article | \
  python3 -c "
from bs4 import BeautifulSoup
import sys
soup = BeautifulSoup(sys.stdin.read(), 'html.parser')
print(soup.get_text())"
```

**After**

```bash
vrk grab https://example.com/article
```

## Example

```bash
vrk grab https://blog.example.com/llm-best-practices | vrk tok
```

## Exit codes

| Code | Meaning                                                       |
|------|---------------------------------------------------------------|
| 0    | Success                                                       |
| 1    | HTTP error, fetch timeout, or I/O error                       |
| 2    | Usage error - invalid URL, no input, mutually exclusive flags |

## Flags

| Flag      | Short | Type | Description                            |
|-----------|-------|------|----------------------------------------|
| `--text`  | -t    | bool | Plain prose output, no markdown syntax |
| `--raw`   |       | bool | Raw HTML, no processing                |
| `--json`  | -j    | bool | Emit JSON envelope with metadata       |
| `--quiet` | -q    | bool | Suppress stderr output                 |


<!-- notes - edit in notes/grab.notes.md -->

## Output formats

### Markdown (default)

```bash
$ vrk grab https://example.com/blog/llm-pipelines
# Getting Started with LLM Pipelines

Building reliable LLM pipelines requires careful attention to...

## Token Management

Before sending any document to an LLM, measure its token count...
```

Clean markdown - headers, paragraphs, links, and code blocks preserved. Ready to pipe to `vrk tok`, `vrk chunk`, or `vrk prompt`.

### Plain text (--text)

```bash
$ vrk grab --text https://example.com/blog/llm-pipelines
Getting Started with LLM Pipelines

Building reliable LLM pipelines requires careful attention to...
```

No markdown syntax. Just prose. Useful when you want to minimize tokens or feed text to a tool that doesn't understand markdown.

### Raw HTML (--raw)

The full, unprocessed HTML. Use when you need to extract specific elements yourself, or when the content extraction strips something you need.

```bash
vrk grab --raw https://example.com/blog/llm-pipelines
```

### JSON envelope (--json)

Wraps the content in a JSON object with metadata including the title, token estimate, and fetch timestamp:

```bash
vrk grab --json https://example.com/blog/llm-pipelines
```

## Pipeline integration

### Fetch, check budget, and summarize

```bash
# Grab an article, make sure it fits in context, then summarize
vrk grab https://blog.example.com/quarterly-review | \
  vrk tok --check 12000 | \
  vrk prompt --system 'Summarize the key findings in 5 bullet points'
```

### Extract and validate links from a web page

```bash
# Grab a page, extract all links, parse each URL
vrk grab https://docs.example.com/api-reference | \
  vrk links --bare | \
  while IFS= read -r url; do
    vrk urlinfo --field host "$url"
  done | sort -u
```

### Fetch, chunk, and process a long article

```bash
# Grab a long page, chunk it if needed, summarize each section
CONTENT=$(vrk grab https://example.com/whitepaper)
TOKENS=$(echo "$CONTENT" | vrk tok --json | jq -r '.tokens')
if [ "$TOKENS" -le 8000 ]; then
  echo "$CONTENT" | vrk prompt --system 'Summarize this'
else
  echo "$CONTENT" | vrk chunk --size 4000 --overlap 200 | \
    while IFS= read -r chunk; do
      echo "$chunk" | jq -r '.text' | vrk prompt --system 'Summarize this section'
    done
fi
```

### Nightly batch with state tracking

```bash
# Fetch each URL, redact secrets from content, summarize, store result
for url in $(cat urls.txt); do
  CONTENT=$(vrk grab "$url" | vrk mask)
  SUMMARY=$(echo "$CONTENT" | vrk prompt --system @prompts/summarize.txt --retry 2)
  if [ $? -eq 0 ]; then
    vrk kv set --ns summaries "$(echo "$url" | vrk slug)" "$SUMMARY" --ttl 168h
    vrk kv incr --ns summaries processed
  fi
done
```

## When it fails

Unreachable URL:

```bash
$ vrk grab https://nonexistent.example.com/page
error: grab: Get "https://nonexistent.example.com/page": dial tcp: lookup nonexistent.example.com: no such host
$ echo $?
1
```

HTTP error:

```bash
$ vrk grab https://example.com/this-page-does-not-exist
error: grab: HTTP 404
$ echo $?
1
```

No URL provided:

```bash
$ vrk grab
usage error: grab: no URL provided
$ echo $?
2
```
