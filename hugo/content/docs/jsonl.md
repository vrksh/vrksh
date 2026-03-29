---
title: "vrk jsonl"
description: "JSON array to JSONL converter - --collect, --json"
tool: jsonl
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Converts between JSON arrays and JSONL (one record per line). Splits arrays for pipeline processing, or collects JSONL lines back into an array when an API expects one. Uses a streaming decoder, so it handles arrays larger than available memory.

## The problem

You have a 3GB JSON array from a data export and need to process records one per line. jq -c '.[]' loads the entire array into memory and gets OOM-killed. Going the other direction - collecting JSONL back into an array for an API that expects one - requires careful bracket and comma handling that breaks on empty input.

## Before and after

**Before**

```bash
# split: jq loads entire array into memory, OOM on large files
cat data.json | jq -c '.[]'
# collect: manual bracket/comma handling
echo '['; cat records.jsonl | sed 's/$/,/' | sed '$ s/,$//' ; echo ']'
```

**After**

```bash
cat data.json | vrk jsonl
cat records.jsonl | vrk jsonl --collect
```

## Example

```bash
cat data.json | vrk jsonl
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, including empty input |
| 1 | Invalid JSON, I/O error |
| 2 | Interactive TTY with no input, unknown flag |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--collect` | -c | bool | Collect JSONL lines into a JSON array |
| `--json` | -j | bool | Append metadata trailer after all records (split mode only) |

