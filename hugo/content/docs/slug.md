---
title: "vrk slug"
description: "URL/filename slug generator - --separator, --max, --json"
tool: slug
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You're generating URLs from article titles, filenames from user-submitted project names, or directory names from arbitrary strings. The input has accented characters, parentheses, emoji, and mixed case - none of which belong in a URL or a filesystem path. You need consistent, safe slugs without unicode surprises or special characters.

`slug` handles Unicode normalization (NFD decomposition, then strip non-ASCII), lowercases everything, keeps only `[a-z0-9]`, and joins words with a configurable separator. Empty slugs are suppressed. Length is capped at a word boundary so you never cut a word in half.

## The fix

```bash
$ echo "My Article Title (2024 Edition)" | vrk slug
my-article-title-2024-edition
```

## Walkthrough

### Basic slugging

```bash
echo "Hello, World!" | vrk slug
echo "Ångström Unit" | vrk slug
echo "  leading and trailing  " | vrk slug
```

<!-- output: verify against binary -->

The accent on `Å` is decomposed via NFD, the base letter `A` is kept, and the combining diacritic is dropped. Input is trimmed of surrounding whitespace before processing.

### Custom separator

```bash
echo "my project name" | vrk slug --separator _
echo "hello world" | vrk slug --separator .
```

<!-- output: verify against binary -->

The separator can be any string. An empty string `--separator ""` concatenates words with no separator, which is useful for generating identifiers rather than URLs.

### Length limit

```bash
echo "A Very Long Title That Would Break Your Database Column" | vrk slug --max 40
```

<!-- output: verify against binary -->

`--max` truncates at the last separator that fits within the limit. You never get a half-word at the end. If the first word already exceeds `--max`, it's truncated at the character boundary.

### JSON output

```bash
$ echo "My Post Title" | vrk slug --json
{"input":"My Post Title","output":"my-post-title"}
```

### Batch processing

```bash
cat titles.txt | vrk slug --max 60
```

One slug per line. Blank input lines produce blank output lines. Lines that produce an empty slug after stripping (for example, a line containing only punctuation) are suppressed entirely - they're skipped with no output.

## Pipeline example

Generate URL-safe filenames for a batch of blog posts:

```bash
cat posts.jsonl \
  | jq -r '.title' \
  | vrk slug --max 60 \
  | while read slug; do echo "${slug}.html"; done
```

Or check whether a user-submitted project name would collide with an existing slug:

```bash
echo "My New Project" \
  | vrk slug \
  | xargs -I{} vrk kv get --ns projects {}
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--separator` | | string | `"-"` | Word separator character or string |
| `--max` | | int | `0` | Max output length; truncated at last separator (0 = no limit) |
| `--json` | `-j` | bool | `false` | Emit JSON per line: `{input, output}` |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (I/O failure) |
| 2 | Usage error - interactive TTY with no stdin |
