---
title: "vrk assert"
description: "pipeline condition check - jq conditions, --contains, --matches"
tool: assert
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Checks a condition on your pipeline data and either passes it through or stops the pipeline. You can match JSON with jq expressions or plain text with substring and regex checks. If the condition fails, nothing reaches the next step.

## The problem

You pipe JSON through a pipeline and assume the shape is correct. Three steps later, a malformed record silently corrupts your output. You only notice when a customer reports wrong data.

## Before and after

**Before**

```bash
curl -s https://api.example.com/health | jq -e '.status == "ok"' > /dev/null
```

**After**

```bash
vrk grab https://api.example.com/health | vrk assert --contains '"status":"ok"'
```

## Example

```bash
vrk grab https://api.example.com/health | vrk assert --contains '"status":"ok"'
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All conditions passed; input passed through to stdout |
| 1 | Assertion failed, or runtime error |
| 2 | No condition specified, mixed modes, invalid regex, interactive TTY |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--contains` |   | string | Assert stdin contains this literal substring |
| `--matches` |   | string | Assert stdin matches this regular expression |
| `--message` | -m | string | Custom message on failure |
| `--json` | -j | bool | Emit errors as JSON to stdout |
| `--quiet` | -q | bool | Suppress stderr output on failure |

