---
title: "vrk grab"
description: "URL fetcher - clean markdown, plain text, or raw HTML."
tool: grab
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

You need the content of a web page in your pipeline. `curl` gives you raw HTML
full of navigation, ads, and script tags. You could pipe it through a series of
tools to extract the article body, but that's fragile and different for every
site. You need a fetcher that returns clean, readable content by default.

## The fix

```bash
vrk grab https://example.com/article
```

<!-- output: verify against binary -->

That fetches the page and returns clean markdown - article body extracted,
navigation and boilerplate stripped. Ready to pipe into `tok`, `prompt`, or
any other tool.

## Walkthrough

### Plain text output

When you don't want markdown syntax at all:

```bash
vrk grab --text https://example.com/article
```

<!-- output: verify against binary -->

The `--text` flag strips all markdown formatting, giving you pure prose.
Useful for feeding into token counters or search indexes where syntax is noise.

### What failure looks like

When the URL is unreachable or returns an error:

```bash
vrk grab https://example.com/nonexistent
echo $?
# 1
```

<!-- output: verify against binary -->

Exit 1 with an error message to stderr. The pipeline stops.

### Raw HTML

When you need the full HTML response without any processing:

```bash
vrk grab --raw https://example.com/page
```

## Pipeline example

Fetch a page, check it fits the context window, then send to an LLM:

```bash
vrk grab https://example.com/article | vrk tok --budget 8000 && \
vrk grab https://example.com/article | vrk prompt --system "Summarize this"
```

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
