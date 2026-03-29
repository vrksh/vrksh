---
title: "vrk emit"
description: "structured logger - wraps stdin lines as JSONL log records"
tool: emit
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Wraps any command's output into structured JSONL log records. Each line gets a timestamp, a level, and a msg field. Pipe a script through emit and you get logs you can actually filter and search, instead of grepping through plain text.

## The problem

Your script writes plain text to stdout. When something breaks at 3am, you grep through unstructured logs trying to find the error. No timestamps, no levels, no structured fields to filter on.

## Before and after

**Before**

```bash
./deploy.sh >> pipeline.log 2>&1
# later: grep "ERROR" pipeline.log
# no timestamps, no structured fields, no level filtering
# good luck finding what failed at 3am
```

**After**

```bash
./deploy.sh | vrk emit --tag deploy --parse-level
```

## Example

```bash
some-script.sh | vrk emit --level info --tag my-script
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All non-empty lines emitted as JSONL records |
| 1 | Stdin scanner error or write failure |
| 2 | Interactive TTY with no positional arg, or unknown --level value |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--level` | -l | string | Log level: debug, info, warn, error |
| `--tag` |   | string | Value for the tag field on every record |
| `--msg` |   | string | Fixed message override; stdin lines parsed as JSON and merged |
| `--parse-level` |   | bool | Auto-detect level from ERROR/WARN/INFO/DEBUG line prefixes |

