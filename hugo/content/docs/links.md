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

## The problem

`grep -oE 'https?://[^ ]+'` finds bare URLs but misses `[text](url)` markdown links. A regex that handles inline links misses reference-style `[text][ref]` links. Neither approach gives you line numbers or link text.

## The solution

`vrk links` extracts hyperlinks from markdown, HTML, or plain text and outputs one JSONL record per link with the link text, URL, and line number. Handles inline links, reference-style links, HTML anchor tags, and bare URLs. `--bare` outputs just the URLs, one per line.

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
vrk grab https://example.com/docs | vrk links --bare
```

## Exit codes

| Code | Meaning                                                    |
|------|------------------------------------------------------------|
| 0    | Success, including empty input and documents with no links |
| 1    | I/O error reading stdin                                    |
| 2    | Interactive TTY with no piped input, unknown flag          |

## Flags

| Flag      | Short | Type | Description                               |
|-----------|-------|------|-------------------------------------------|
| `--bare`  | -b    | bool | Output URLs only, one per line            |
| `--json`  | -j    | bool | Append metadata trailer after all records |
| `--quiet` | -q    | bool | Suppress stderr output                    |


<!-- notes - edit in notes/links.notes.md -->

## How it works

### JSONL output (default)

```bash
$ printf '# Getting Started\n\nVisit [our docs](https://docs.example.com) for the full guide.\nAlso check [GitHub](https://github.com/example/repo) and https://blog.example.com/updates.\n' | vrk links
{"text":"our docs","url":"https://docs.example.com","line":3}
{"text":"GitHub","url":"https://github.com/example/repo","line":4}
{"text":"https://blog.example.com/updates.","url":"https://blog.example.com/updates.","line":4}
```

Each record includes the link text, URL, and line number. Bare URLs use the URL itself as the text.

### URLs only (--bare)

```bash
$ printf '# Getting Started\n\nVisit [our docs](https://docs.example.com) for the full guide.\nAlso check [GitHub](https://github.com/example/repo) and https://blog.example.com/updates.\n' | vrk links --bare
https://docs.example.com
https://github.com/example/repo
https://blog.example.com/updates.
```

One URL per line, no JSON. Pipe directly to `xargs`, `while read`, or `vrk grab`.

### Metadata trailer (--json)

```bash
cat README.md | vrk links --json
```

Appends `{"_vrk":"links","count":N}` as the last record.

## What gets detected

- Markdown inline links: `[text](url)`
- Markdown reference links: `[text][ref]` with `[ref]: url`
- HTML anchor tags: `<a href="url">text</a>`
- Bare URLs: `https://example.com` in plain text

## Pipeline integration

### Find broken links in documentation

```bash
# Extract all links from a doc and check each one
cat docs/README.md | vrk links --bare | while IFS= read -r url; do
  STATUS=$(curl -sI -o /dev/null -w '%{http_code}' "$url")
  if [ "$STATUS" != "200" ]; then
    echo "Broken: $url ($STATUS)" | vrk emit --level warn --tag linkcheck
  fi
done
```

### Extract links from a web page

```bash
# Grab a page, extract all links, get unique domains
vrk grab https://example.com | vrk links --bare | \
  while IFS= read -r url; do
    vrk urlinfo --field host "$url"
  done | sort -u
```

### Index links in kv

```bash
# Store all links from a document with their text for later lookup
cat document.md | vrk links | while IFS= read -r record; do
  URL=$(echo "$record" | jq -r '.url')
  TEXT=$(echo "$record" | jq -r '.text')
  vrk kv set --ns links "$(echo "$URL" | vrk slug)" "$TEXT"
done
```

## When it fails

No input:

```bash
$ vrk links
usage error: links: no input: pipe text to stdin
$ echo $?
2
```

Empty input produces no output and exits 0 - there are simply no links to extract.
