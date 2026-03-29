---
title: "vrk slug"
description: "URL/filename slug generator - --separator, --max, --json"
tool: slug
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Turns arbitrary text into URL-safe slugs. Normalizes Unicode, lowercases everything, and keeps only letters and numbers. Truncates at word boundaries so you don't get cut-off words. Processes one line at a time for batch use.

## The problem

You need a URL-safe slug from a blog title. You lowercase and replace spaces with hyphens, but forget about Unicode, punctuation, and consecutive separators. The slug "my--great--post-!" breaks your router.

## Before and after

**Before**

```bash
echo 'My Great Post!' | tr '[:upper:]' '[:lower:]' | tr ' ' '-' | tr -cd 'a-z0-9-'
# consecutive hyphens, no Unicode normalization, no truncation
```

**After**

```bash
echo 'My Great Post!' | vrk slug
```

## Example

```bash
echo 'My Article Title' | vrk slug
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (I/O failure) |
| 2 | Interactive TTY with no stdin |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--separator` |   | string | Word separator character or string |
| `--max` |   | int | Max output length; truncated at last separator (0 = no limit) |
| `--json` | -j | bool | Emit JSON per line: {input, output} |
| `--quiet` | -q | bool | Suppress stderr output |

