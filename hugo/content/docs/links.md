---
title: "vrk links"
description: "Extract every hyperlink from markdown, HTML, or plain text. JSONL output with text, URL, and line number."
og_title: "vrk links - extract hyperlinks from markdown and HTML"
tool: links
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Extracts every link from markdown, HTML, or plain text in one pass. Each link comes out as a JSONL record with the link text, URL, and source line number. Handles inline markdown links, reference-style links, HTML anchors, and bare URLs.

## The problem

You need to extract all links from a markdown file. grep finds bare URLs but misses markdown links. A Python script with regex handles inline links but misses reference-style links. You end up with partial results and no line numbers.

## Before and after

**Before**

```bash
grep -oE 'https?://[^ ]+' README.md
# misses [text](url) markdown links
# misses [text][ref] reference links
# no line numbers, no link text
```

**After**

```bash
cat README.md | vrk links
```

## Example

```bash
cat README.md | vrk links
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, including empty input and documents with no links |
| 1 | I/O error reading stdin |
| 2 | Interactive TTY with no piped input, unknown flag |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--bare` | -b | bool | Output URLs only, one per line |
| `--json` | -j | bool | Append metadata trailer after all records |
| `--quiet` | -q | bool | Suppress stderr output |

