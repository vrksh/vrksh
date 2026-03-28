---
title: "vrk links"
description: "Hyperlink extractor - markdown, HTML, bare URLs to JSONL."
tool: links
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You have a README, a web page, or a documentation file full of links. You want to extract them all - to check for broken URLs, to crawl a site, or to feed them into another tool. The options are either a regex that misses edge cases (markdown reference-style links, HTML anchors, bare URLs in the same document) or a full HTML parser that only handles one format.

## The fix

```bash
cat README.md | vrk links
```

Each link becomes a JSONL record with its link text, URL, and source line number:

```
{"text":"Homebrew","url":"https://brew.sh","line":12}
{"text":"Node.js","url":"https://nodejs.org","line":15}
```

## Walkthrough

Links handles three input formats in a single pass, with overlap detection so a URL that appears in a markdown link is not also extracted as a bare URL.

**Markdown inline** - `[text](url)`:

```bash
echo '[Download](https://example.com/download)' | vrk links
# {"text":"Download","url":"https://example.com/download","line":1}
```

**Markdown reference-style** - two-pass extraction. Pass 1 collects all `[label]: url` definitions; pass 2 resolves `[text][label]` usages against them:

```bash
cat README.md | vrk links
```

**HTML anchors** - `<a href="url">text</a>`, case-insensitive:

```bash
vrk grab https://example.com | vrk links
```

**Bare URLs** - `https://...` appearing anywhere in the text, at positions not already consumed by a richer pattern.

**URLs only** - `--bare` drops the text and line number, outputting one URL per line. Useful for piping into `curl`, `xargs`, or another `vrk grab`:

```bash
cat README.md | vrk links --bare
# https://brew.sh
# https://nodejs.org
```

**Metadata count** - `--json` appends a trailer record after all link records:

```bash
cat README.md | vrk links --json
# {"text":"...","url":"...","line":1}
# ...
# {"_vrk":"links","count":42}
```

Empty input exits 0 with no output (or just the count trailer if `--json` is active with count 0).

## Pipeline example

Extract every link from a page and check each one with grab:

```bash
vrk grab https://example.com | vrk links --bare | xargs -I{} vrk grab --quiet {}
```

Crawl a README for external links and count the tokens at each destination:

```bash
cat README.md | vrk links --bare | while IFS= read -r url; do
  vrk grab --text "$url" | vrk tok --json | jq -c ". + {url: \"$url\"}"
done
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--bare` | `-b` | bool | false | Output URLs only, one per line. No JSON, no text, no line numbers. |
| `--json` | `-j` | bool | false | Append `{"_vrk":"links","count":N}` after all records |
| `--quiet` | `-q` | bool | false | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success, including empty input and documents with no links |
| 1 | Runtime error - I/O error reading stdin |
| 2 | Usage error - interactive TTY with no piped input, unknown flag |
