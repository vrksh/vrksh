---
title: "vrk coax"
description: "retry wrapper - --times, --backoff, --on, --until"
tool: coax
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Wraps any command with automatic retries and backoff. If the command fails, coax re-runs it - buffering stdin so each attempt gets the same input. You control how many times to retry, how long to wait between attempts, and which exit codes count as retriable.

## The problem

You call an external API in a script and it fails once out of fifty times. You add a retry loop with sleep, but you hardcode the delay, forget to cap retries, and the stdin is consumed on the first attempt so retries send empty input.

## Before and after

**Before**

```bash
for i in 1 2 3 4 5; do
  curl -sf https://api.example.com && break
  sleep $((i * 2))
done
```

**After**

```bash
vrk coax --times 5 --backoff exp:200ms -- curl -sf https://api.example.com
```

## Example

```bash
vrk coax --times 5 --backoff exp:200ms -- curl -sf https://api.example.com
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Command succeeded (first attempt or a retry) |
| 1 | All retries exhausted (last exit code from wrapped command) |
| 2 | Missing command after --, invalid --backoff spec |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--times` |   | int | Number of retries (total attempts = N+1) |
| `--backoff` |   | string | Delay between retries: 500ms for fixed, exp:100ms for exponential |
| `--backoff-max` |   | duration | Cap for exponential backoff; 0 means uncapped |
| `--on` |   | []int | Retry only on these exit codes; repeatable |
| `--until` |   | string | Shell condition; retry while this exits non-zero |
| `--quiet` | -q | bool | Suppress retry progress lines |
| `--json` | -j | bool | Emit errors as JSON |

