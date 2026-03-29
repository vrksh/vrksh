---
title: "vrk coax"
description: "retry wrapper - --times, --backoff, --on, --until"
tool: coax
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → coax → stdout`

Exit 0 Command succeeded (first attempt or a retry) · Exit 1 All retries exhausted (last exit code from wrapped command) · Exit 2 Missing command after --, invalid --backoff spec

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

## Example

```bash
vrk coax --times 5 --backoff exp:200ms -- curl -sf https://api.example.com
```
