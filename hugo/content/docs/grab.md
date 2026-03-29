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

Fetches a web page and extracts the readable content as clean markdown. Strips navigation, ads, scripts, and boilerplate - you get the article text, not the page chrome. Also supports plain text and raw HTML output modes.

## The problem

You need the content of a web page for an LLM pipeline. curl gives you raw HTML full of nav bars, ads, and scripts. You write a Python script with BeautifulSoup to extract the article text, and it breaks on the next site.

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
vrk grab --text https://example.com/article
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | HTTP error, fetch timeout, or I/O error |
| 2 | Usage error - invalid URL, no input, mutually exclusive flags |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--text` | -t | bool | Plain prose output, no markdown syntax |
| `--raw` |   | bool | Raw HTML, no processing |
| `--json` | -j | bool | Emit JSON envelope with metadata |
| `--quiet` | -q | bool | Suppress stderr output |

