---
title: "vrk slug"
description: "vrk slug turns any text into a URL-safe, filename-safe slug. It folds Latin text to ASCII, accepts custom separators, and limits length."
meta_lead: "vrk slug converts any text to a URL-safe, filename-safe slug with Unicode normalization."
og_title: "vrk slug - URL and filename slug generator"
tool: slug
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk slug converts any text to a URL-safe, filename-safe slug with Unicode normalization.

## The problem

A slugify function turns "Uber's $3.1B Q4 Revenue" into "uber-s--3-1b-q4-revenue" with a double hyphen. Another title has Unicode characters that pass through unchanged and break a downstream system expecting ASCII-only paths. `tr` and `sed` handle simple cases but miss edge cases consistently.

## The solution

`vrk slug` converts any text to a URL-safe, filename-safe slug. It folds Latin text to ASCII (`Straße` becomes `strasse`) and keeps other scripts, such as Devanagari and CJK, as they are. It lowercases the text, replaces non-alphanumeric characters with hyphens, and collapses consecutive hyphens. `--max` cuts the slug at a word boundary.

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

| Flag          | Short | Type   | Description                                                                               |
|---------------|-------|--------|-------------------------------------------------------------------------------------------|
| `--separator` |       | string | Word separator character or string                                                        |
| `--max`       |       | int    | Max output length in characters; truncated at last separator (0 = no limit)               |
| `--ascii`     |       | bool   | Keep only ASCII; other scripts are dropped (empty slug plus a warning if nothing is left) |
| `--json`      | -j    | bool   | Emit JSON per line: {input, output}                                                       |
| `--quiet`     | -q    | bool   | Suppress stderr output                                                                    |


<!-- notes - edit in notes/slug.notes.md -->

## How it works

`vrk slug` turns text into a URL-safe slug: lowercase words joined by hyphens.

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

`--max` cuts the slug at a word boundary, so a word is never cut in half:

```bash
$ echo 'My Very Long Blog Post Title That Goes On Forever' | vrk slug --max 30
my-very-long-blog-post-title
```

### Batch processing

slug makes one slug per input line:

```bash
$ printf 'First Post\nSecond Post\nThird Post\n' | vrk slug
first-post
second-post
third-post
```

### Non-Latin text

Latin letters fold to ASCII. slug keeps other scripts as they are, so a Hindi or Japanese title still gets a slug:

```bash
$ printf 'Straße\nनमस्ते दुनिया\n東京 2026\n' | vrk slug
strasse
नमस्ते-दुनिया
東京-2026
```

Every input line gives exactly one output line. If a line has nothing to keep, slug prints an empty line and warns on stderr, so the output lines match the input lines. Use `--ascii` when the target needs strict ASCII:

```bash
$ echo '東京 2026' | vrk slug --ascii
2026
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

If you give no input on an interactive terminal, slug exits 2:

<!-- doccheck: skip (needs an interactive terminal, doccheck runs with empty stdin) -->
```bash
$ vrk slug
usage error: slug: no input
$ echo $?
2
```
