---
title: "vrk validate"
description: "JSONL schema validator - --schema, --strict, --fix, --json"
tool: validate
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Checks every JSON record in a stream against a schema you provide. Valid records pass through to stdout. Invalid ones are dropped with a warning so downstream tools only see clean data. Use --strict to halt on the first bad record instead of skipping it.

## The problem

You have a JSONL stream and need to ensure every record matches a schema before it enters your database. jq can check types but the expressions get unreadable fast. A bad record slips through and corrupts downstream aggregations.

## Before and after

**Before**

```bash
cat records.jsonl | while read line; do
  echo "$line" | jq -e '.name | type == "string"' > /dev/null && \
  echo "$line" | jq -e '.age | type == "number"' > /dev/null && \
  echo "$line"
done
```

**After**

```bash
cat records.jsonl | vrk validate --schema '{"name":"string","age":"number"}'
```

## Example

```bash
cat records.jsonl | vrk validate --schema '{"name":"string","age":"number"}' --strict
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | All records passed |
| 1 | Record failed in --strict mode, or scanner error |
| 2 | --schema missing, schema JSON invalid, unknown flag |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--schema` | -s | string | JSON schema inline or path to .json file (required) |
| `--strict` |   | bool | Exit 1 on first invalid record |
| `--fix` |   | bool | Send invalid records to vrk prompt for repair |
| `--json` | -j | bool | Append metadata trailer after all output |
| `--quiet` | -q | bool | Suppress stderr output |

