---
title: "vrk slug"
description: "Generate URL-safe slugs from any text. Handles unicode, custom separators, and length limits."
og_title: "vrk slug - URL and filename slug generator"
tool: slug
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

You need URL-safe slugs from user-provided titles. You lowercase, replace spaces with hyphens, strip special characters. Then a user submits a title with Unicode characters, curly quotes, or unusual punctuation. Your regex misses them. The slug breaks the URL.

`vrk slug` converts any text to a URL-safe, filename-safe slug. Normalizes Unicode to ASCII, lowercases everything, replaces non-alphanumeric characters with hyphens, and collapses consecutive hyphens. Use `--max` to truncate at word boundaries for length-limited contexts.

## The problem

You generate filenames from document titles. A title contains "Uber's $3.1B Q4 Revenue" and your slugify function produces "uber-s--3-1b-q4-revenue" with a double hyphen. Another title has Unicode characters that pass through unchanged and break a downstream system that expects ASCII-only paths.

## Before and after

**Before**

```bash
echo "$TITLE" | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -cd 'a-z0-9-'
# Misses Unicode normalization, doesn't collapse double hyphens
```

**After**

```bash
echo $TITLE | vrk slug
```

## Example

```bash
echo 'My Blog Post: A Deep Dive into LLM Pipelines!' | vrk slug
```

## Exit codes

| Code | Meaning                       |
|------|-------------------------------|
| 0    | Success                       |
| 1    | Runtime error (I/O failure)   |
| 2    | Interactive TTY with no stdin |

## Flags

| Flag          | Short | Type   | Description                                                   |
|---------------|-------|--------|---------------------------------------------------------------|
| `--separator` |       | string | Word separator character or string                            |
| `--max`       |       | int    | Max output length; truncated at last separator (0 = no limit) |
| `--json`      | -j    | bool   | Emit JSON per line: {input, output}                           |
| `--quiet`     | -q    | bool   | Suppress stderr output                                        |


<!-- notes - edit in notes/slug.notes.md -->

## How it works

```bash
$ echo 'Hello World' | vrk slug
hello-world

$ echo 'My Blog Post: A Deep Dive into LLM Pipelines!' | vrk slug
my-blog-post-a-deep-dive-into-llm-pipelines
```

### Custom separator

```bash
$ echo 'Hello World' | vrk slug --separator _
hello_world
```

### Length limit

Truncates at word boundaries so slugs don't end mid-word:

```bash
$ echo 'My Very Long Blog Post Title That Goes On Forever' | vrk slug --max 30
my-very-long-blog-post-title
```

### Batch processing

One slug per input line:

```bash
$ printf 'First Post\nSecond Post\nThird Post\n' | vrk slug
first-post
second-post
third-post
```

### JSON output

```bash
$ echo 'Hello World' | vrk slug --json
{"input":"Hello World","output":"hello-world"}
```

## Pipeline integration

### Generate cache keys from URLs

```bash
# Use slugified URLs as kv keys
for url in $(cat urls.txt); do
  KEY=$(echo "$url" | vrk slug)
  vrk kv set --ns cache "$KEY" "$(vrk grab "$url")" --ttl 24h
done
```

### Create filenames from document titles

```bash
# Generate safe filenames for downloaded articles
while IFS= read -r url; do
  TITLE=$(vrk grab --json "$url" | jq -r '.title')
  FILENAME=$(echo "$TITLE" | vrk slug --max 50).md
  vrk grab "$url" > "$FILENAME"
done < urls.txt
```

## When it fails

No input:

```bash
$ vrk slug
usage error: slug: no input
$ echo $?
2
```
