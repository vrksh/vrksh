---
title: "vrk jsonl"
description: "Convert between JSON arrays and JSONL. Flatten arrays for streaming, collect lines back into arrays."
meta_lead: "vrk jsonl converts JSON arrays to JSONL and back for line-by-line pipeline processing."
og_title: "vrk jsonl - convert between JSON arrays and JSONL"
tool: jsonl
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk jsonl converts JSON arrays to JSONL and back for line-by-line pipeline processing.

## The problem

An API returns a JSON array of 50,000 records. `jq '.[]'` flattens it but loads the entire array into memory first. On a 2GB response the process gets OOM-killed. Line-by-line processing requires JSONL, but the API only returns arrays.

## The solution

`vrk jsonl` converts JSON arrays to JSONL (one object per line) and back. The default mode splits arrays for line-by-line pipeline processing. `--collect` gathers JSONL lines back into a JSON array. The streaming decoder handles files larger than available memory.

## Before and after

**Before**

```bash
cat data.json | jq '.[]'
# Loads entire array into memory. OOM on large files.
```

**After**

```bash
cat data.json | vrk jsonl
```

## Example

```bash
cat api-response.json | vrk jsonl | vrk validate --schema '{"name":"string"}'
```

## Exit codes

| Code | Meaning                                     |
|------|---------------------------------------------|
| 0    | Success, including empty input              |
| 1    | Invalid JSON, I/O error                     |
| 2    | Interactive TTY with no input, unknown flag |

## Flags

| Flag        | Short | Type | Description                                                 |
|-------------|-------|------|-------------------------------------------------------------|
| `--collect` | -c    | bool | Collect JSONL lines into a JSON array                       |
| `--json`    | -j    | bool | Append metadata trailer after all records (split mode only) |


<!-- notes - edit in notes/jsonl.notes.md -->

## How it works

### Split a JSON array into JSONL

```bash
$ echo '[{"name":"Alice"},{"name":"Bob"},{"name":"Carol"}]' | vrk jsonl
{"name":"Alice"}
{"name":"Bob"}
{"name":"Carol"}
```

Each array element becomes one line. Pipe to `while read`, `vrk validate`, or any line-oriented tool.

### Collect JSONL back into an array (--collect)

```bash
$ printf '{"name":"Alice"}\n{"name":"Bob"}\n' | vrk jsonl --collect
[{"name":"Alice"},{"name":"Bob"}]
```

Use this when a downstream tool or API expects a JSON array.

### Metadata trailer (--json)

```bash
$ echo '[{"a":1},{"b":2}]' | vrk jsonl --json
{"a":1}
{"b":2}
{"_vrk":"jsonl","count":2}
```

## Pipeline integration

### Split an API response for validation

```bash
# API returns a JSON array; split it for per-record validation
curl -s https://api.example.com/users | \
  vrk jsonl | \
  vrk validate --schema '{"name":"string","email":"string"}' --strict
```

### Process array records through an LLM

```bash
# Split array, process each record, collect results back
cat data.json | vrk jsonl | \
  while IFS= read -r record; do
    echo "$record" | vrk prompt --system 'Classify this record'
  done | vrk jsonl --collect > results.json
```

### Sample from a large array

```bash
# Split a large JSON array, sample 100 records
cat large-dataset.json | vrk jsonl | vrk sip --count 100 --seed 42
```

### Throttle array processing

```bash
# Split array and rate-limit processing to 5 records per second
cat data.json | vrk jsonl | vrk throttle --rate 5/s | \
  while IFS= read -r record; do
    process "$record"
  done
```

## When it fails

Invalid JSON input:

```bash
$ echo 'not json' | vrk jsonl
error: jsonl: invalid JSON
$ echo $?
1
```

No input:

```bash
$ vrk jsonl
usage error: jsonl: no input: pipe JSON to stdin
$ echo $?
2
```
