---
title: "vrk uuid"
description: "UUID generator - v4/v7, --count, --json."
tool: uuid
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You need a unique ID for a pipeline run, a batch job, or a temporary key in your store. The options are all slightly annoying: `uuidgen` is not available everywhere and its output format varies by platform, Python's `uuid.uuid4()` requires opening a REPL, and writing a shell function is overkill for something this basic. When you need a time-ordered ID that sorts correctly as a database primary key, the options get worse.

## The fix

```bash
vrk uuid
```

<!-- output: verify against binary -->

Generate a time-ordered v7 UUID for use as a sortable key:

```bash
vrk uuid --v7
```

<!-- output: verify against binary -->

## Walkthrough

By default, `vrk uuid` generates a single v4 (random) UUID and prints it to stdout with a trailing newline.

**Multiple IDs** - `--count` generates a batch, one per line:

```bash
vrk uuid --count 5
```

<!-- output: verify against binary -->

**Structured output** - `--json` wraps each UUID in a record with version and generation timestamp. With `--count > 1` it emits JSONL:

```bash
vrk uuid --json
# {"uuid":"...","version":4,"generated_at":1740009600}

vrk uuid --v7 --count 3 --json
# {"uuid":"...","version":7,"generated_at":1740009600}
# {"uuid":"...","version":7,"generated_at":1740009600}
# {"uuid":"...","version":7,"generated_at":1740009600}
```

`generated_at` is a Unix timestamp computed once per batch, so all records in the same run share the same value.

**v4 vs v7** - v4 UUIDs are random and have no intrinsic order. v7 UUIDs embed a millisecond-precision timestamp in the high bits, so they sort lexicographically by creation time. Use v7 when inserting into a database index where insert order matters.

UUID does not read from stdin - it has nothing to parse. It generates fresh IDs on every call.

## Pipeline example

Tag a pipeline run and store it for correlation later:

```bash
vrk uuid --v7 | vrk kv set --ns jobs run_id
```

Generate a batch of IDs and pipe them into another tool:

```bash
vrk uuid --count 10 | while IFS= read -r id; do
  echo "$id" | vrk kv set --ns batch "item_$id" "pending"
done
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--v7` | | bool | false | Generate a v7 (time-ordered) UUID instead of v4 |
| `--count` | `-n` | int | 1 | Number of UUIDs to generate. Must be >= 1. |
| `--json` | `-j` | bool | false | Emit `{"uuid","version","generated_at"}` JSON. Multiple UUIDs emit JSONL. |
| `--quiet` | `-q` | bool | false | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (generation failure) |
| 2 | Usage error - `--count` less than 1, unknown flag |
