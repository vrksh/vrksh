---
title: "vrk recase"
description: "Convert between naming conventions. snake_case, camelCase, kebab-case, PascalCase, and Title Case."
og_title: "vrk recase - convert between snake, camel, kebab, and more"
tool: recase
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Converts between naming conventions - snake_case, camelCase, kebab-case, PascalCase, and Title Case. Auto-detects the input format so you just specify what you want. Handles acronyms correctly: HTMLParser becomes html_parser, not h_t_m_l_parser.

## The problem

You need to convert getUserName to get_user_name for a database column. You write a regex but it breaks on acronyms - HTMLParser becomes h_t_m_l_parser instead of html_parser. Every naming convention edge case is its own bug.

## Before and after

**Before**

```bash
echo 'getUserName' | sed 's/\([A-Z]\)/_\L\1/g' | sed 's/^_//'
# breaks on acronyms: HTMLParser -> h_t_m_l_parser
```

**After**

```bash
echo 'getUserName' | vrk recase --to snake
```

## Example

```bash
echo 'getUserName' | vrk recase --to snake
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (I/O failure) |
| 2 | --to missing or invalid value, interactive TTY with no stdin |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--to` |   | string | Target convention: camel, pascal, snake, kebab, screaming, title, lower, upper |
| `--json` | -j | bool | Emit JSON per line: {input, output, from, to} |
| `--quiet` | -q | bool | Suppress stderr output |

