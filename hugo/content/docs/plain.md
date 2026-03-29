---
title: "vrk plain"
description: "Strip markdown syntax and keep the prose. Clean input for token counting or plain-text pipelines."
og_title: "vrk plain - strip markdown to plain text"
tool: plain
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Strips markdown formatting and gives you plain prose. Headings, links, fences, and bullet markers are removed, but the actual text content is preserved. Useful before sending content to an LLM where formatting syntax wastes tokens.

## The problem

You feed a markdown README to an LLM and waste tokens on formatting syntax - header markers, link URLs, fence markers, bullet characters. The content is the same but the token count is 20-30% higher than it needs to be.

## Before and after

**Before**

```bash
# no standard CLI tool for this
python3 -c "
import re, sys
text = sys.stdin.read()
text = re.sub(r'#{1,6}\s+', '', text)
text = re.sub(r'\[([^\]]+)\]\([^\)]+\)', r'\1', text)
print(text)" < README.md
```

**After**

```bash
cat README.md | vrk plain
```

## Example

```bash
cat README.md | vrk plain | vrk tok --budget 4000
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Could not read stdin or write stdout |
| 2 | Interactive TTY with no piped input and no positional arg |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | -j | bool | Emit JSON with text, input_bytes, output_bytes |
| `--quiet` | -q | bool | Suppress stderr output |

