---
title: "vrk throttle"
description: "Pace LLM batch jobs to respect rate limits. No more failures on job 847 of 10,000."
og_title: "vrk throttle - rate-limit-aware pacing for LLM batch jobs"
tool: throttle
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Controls how fast lines flow through a pipeline. You set a rate like 10/s or 100/m and throttle re-emits lines at that pace. Sits between a source and a consumer to keep you under API rate limits without writing sleep loops.

## The problem

You pipe 10,000 URLs into an API client and get rate-limited after the first 100. You add a sleep between calls but it is either too slow (wasting time) or too fast (still hitting limits). Burst handling requires a token bucket you do not want to implement.

## Before and after

**Before**

```bash
cat urls.txt | while read url; do
  curl -s "$url"
  sleep 0.1
done
# too slow for APIs that allow bursts
# too fast when the API is strict
```

**After**

```bash
cat urls.txt | vrk throttle --rate 10/s | xargs -I{} curl -s {}
```

## Example

```bash
cat urls.txt | vrk throttle --rate 10/s | xargs -I{} vrk grab {}
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All lines emitted at specified rate |
| 1 | Stdin read error, write error, or --tokens-field not found |
| 2 | --rate missing or invalid, interactive TTY |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--rate` | -r | string | Rate limit in N/s or N/m format (required) |
| `--burst` |   | int | Emit first N lines immediately before applying rate limit |
| `--tokens-field` |   | string | Dot-path to JSONL field for token-based rate limiting |
| `--json` | -j | bool | Append metadata record after all output |
| `--quiet` | -q | bool | Suppress stderr output |

