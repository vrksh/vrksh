---
title: "vrk uuid"
description: "Generate UUIDs from the command line. v4 random or v7 time-sorted. Batch with --count."
og_title: "vrk uuid - UUID v4 and v7 generator"
tool: uuid
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

You need a unique ID for a database row, a batch run, or a file name. `uuidgen` exists on macOS but not everywhere. It only generates v4. You need v7 (time-ordered) for database primary keys that sort by creation time, and `uuidgen` can't do that.

`vrk uuid` generates v4 (random) or v7 (time-ordered) UUIDs with consistent output across macOS and Linux. Use `--v7` for database keys that sort chronologically. Use `--count` to generate multiple UUIDs in one call.

## The problem

You generate UUIDs for database rows with uuidgen. Your queries are slow because B-tree indexes fragment on random v4 UUIDs. You read about v7 UUIDs that sort by creation time. Your system uuidgen doesn't support v7. You pull in a Python dependency for one function.

## Before and after

**Before**

```bash
uuidgen  # v4 only, macOS only, no batch mode
# Need v7? Install Python + uuid7 package
```

**After**

```bash
vrk uuid --v7
```

## Example

```bash
vrk uuid --v7 --count 5
```

## Exit codes

| Code | Meaning                            |
|------|------------------------------------|
| 0    | Success                            |
| 1    | Runtime error (generation failure) |
| 2    | --count less than 1, unknown flag  |

## Flags

| Flag      | Short | Type | Description                                     |
|-----------|-------|------|-------------------------------------------------|
| `--v7`    |       | bool | Generate a v7 (time-ordered) UUID instead of v4 |
| `--count` | -n    | int  | Number of UUIDs to generate                     |
| `--json`  | -j    | bool | Emit JSON with uuid, version, generated_at      |
| `--quiet` | -q    | bool | Suppress stderr output                          |


<!-- notes - edit in notes/uuid.notes.md -->

## How it works

### v4 (random, default)

```bash
$ vrk uuid
98d06ca5-e747-4a63-8fc9-045f0e02f8d4
```

Standard random UUID. Good for correlation IDs, file names, and anywhere uniqueness matters but ordering doesn't.

### v7 (time-ordered)

```bash
$ vrk uuid --v7
019d3fdb-2de7-756e-a340-de561cfa43ab
```

The first 48 bits encode a millisecond timestamp. v7 UUIDs sort chronologically - newer IDs are always lexicographically greater. Use these for database primary keys to keep B-tree indexes efficient.

### Batch generation

```bash
$ vrk uuid --count 3
c00bb1e4-51fd-4cfc-96b2-2c0123268c80
c4fd6dd9-c340-4f42-9929-7f0eae856230
58cbbd73-e561-43de-b20a-ccd5f5f92222

$ vrk uuid --v7 --count 3 --json
{"version":7,"uuid":"019d3fdb-..."}
{"version":7,"uuid":"019d3fdb-..."}
{"version":7,"uuid":"019d3fdb-..."}
```

## Pipeline integration

### Generate a run ID for a batch pipeline

```bash
RUN_ID=$(vrk uuid --v7)
vrk kv set --ns pipeline run_id "$RUN_ID"
echo "Starting run $RUN_ID"
```

### Use as chunk IDs

```bash
# Assign unique IDs to each chunk of a document
cat document.md | vrk chunk --size 4000 | \
  while IFS= read -r record; do
    CHUNK_ID=$(vrk uuid --v7)
    echo "$record" | jq --arg id "$CHUNK_ID" '. + {id: $id}'
  done
```

### Generate test data

```bash
# Create 100 unique user records
vrk uuid --count 100 | while IFS= read -r id; do
  echo "{\"id\":\"$id\",\"name\":\"user-$RANDOM\"}"
done
```

## When it fails

Invalid count:

```bash
$ vrk uuid --count 0
usage error: uuid: --count must be >= 1
$ echo $?
2
```
