---
title: "vrk urlinfo"
description: "Parse URLs and extract components. Get scheme, host, path, query params, or any field individually."
og_title: "vrk urlinfo - URL parser and component extractor"
tool: urlinfo
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Breaks URLs into their components - scheme, host, port, path, query parameters - as structured JSON. You can extract a single piece with --field, including nested query parameters using dot-path syntax. Pure parsing, no network calls.

## The problem

You have a URL from an API response and need to extract a query parameter to pass to the next pipeline step. Splitting on ? and & in bash is fragile - encoded characters, missing parameters, and multiple values for the same key all break your parsing.

## Before and after

**Before**

```bash
URL="https://api.example.com/callback?code=abc123&state=xyz"
echo "$URL" | grep -oP 'code=\K[^&]+'
# breaks if code= is missing (grep exits 1, pipeline fails)
# breaks on percent-encoded values like code=abc%3D123
# not portable: -P (Perl regex) is GNU grep only
```

**After**

```bash
echo "$URL" | vrk urlinfo --field query.code
```

## Example

```bash
echo 'https://api.example.com:8443/v2/users?page=2' | vrk urlinfo
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Invalid URL that cannot be parsed, I/O error |
| 2 | Interactive TTY with no stdin or positional arg |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--field` | -F | string | Extract a single field as plain text (supports dot-path for query params) |
| `--json` | -j | bool | Append metadata trailer |
| `--quiet` | -q | bool | Suppress stderr output |

