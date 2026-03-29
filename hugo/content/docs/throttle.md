---
title: "vrk throttle"
description: "rate limiter for pipes - --rate N/s or N/m"
tool: throttle
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → throttle → stdout`

Exit 0 All lines emitted at specified rate · Exit 1 Stdin read error, write error, or --tokens-field not found · Exit 2 --rate missing or invalid, interactive TTY

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--rate` | -r | string | Rate limit in N/s or N/m format (required) |
| `--burst` |   | int | Emit first N lines immediately before applying rate limit |
| `--tokens-field` |   | string | Dot-path to JSONL field for token-based rate limiting |
| `--json` | -j | bool | Append metadata record after all output |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
cat urls.txt | vrk throttle --rate 10/s | xargs -I{} vrk grab {}
```
