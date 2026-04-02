---
title: "vrk moniker"
description: "Generate memorable names for run IDs, job labels, and temp dirs. No more UUIDs in log output."
meta_lead: "vrk moniker generates memorable adjective-noun names for run IDs, job labels, and temp directories."
og_title: "vrk moniker - memorable name generator for run IDs and labels"
tool: moniker
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk moniker generates memorable adjective-noun names for run IDs, job labels, and temp directories.

## The problem

Timestamp labels like `run-20260330-174856` collide when three runs start in the same minute. UUIDs are unique but nobody can say "check run 98d06ca5" in an incident call. You need identifiers that are both unique and pronounceable.

## The solution

`vrk moniker` generates memorable adjective-noun names from embedded wordlists. Deterministic with `--seed` for reproducible names. `--count` for batches, `--words` for longer names when you need more uniqueness. No duplicates within a single batch.

## Before and after

**Before**

```bash
echo "run-$(date +%Y%m%d-%H%M%S)"
# Identical for concurrent runs, not human-memorable
```

**After**

```bash
vrk moniker
```

## Example

```bash
vrk moniker --count 5 --seed 42
```

## Exit codes

| Code | Meaning                                  |
|------|------------------------------------------|
| 0    | Success                                  |
| 1    | Word pool exhausted for requested count  |
| 2    | --count less than 1, --words less than 2 |

## Flags

| Flag          | Short | Type   | Description                          |
|---------------|-------|--------|--------------------------------------|
| `--count`     | -n    | int    | Number of names to generate          |
| `--separator` |       | string | Word separator                       |
| `--words`     |       | int    | Words per name (minimum 2)           |
| `--seed`      |       | int64  | Random seed for deterministic output |
| `--json`      | -j    | bool   | Emit JSON per name: {name, words}    |
| `--quiet`     | -q    | bool   | Suppress stderr output               |


<!-- notes - edit in notes/moniker.notes.md -->

## How it works

```bash
$ vrk moniker
woven-vent

$ vrk moniker --count 3
distant-shore
slate-contour
northern-brine
```

### Deterministic output (--seed)

```bash
$ vrk moniker --count 3 --seed 42
distant-shore
slate-contour
northern-brine
```

Same seed always produces the same names. Use this in tests or when you need reproducible identifiers.

### Custom separator

```bash
$ vrk moniker --separator _
bold_falcon
```

### Longer names (--words)

More words means more uniqueness. Use `--words 3` when two-word names aren't enough:

```bash
$ vrk moniker --words 3
bright-amber-whisper
```

### JSON output

```bash
$ vrk moniker --json
{"name":"woven-vent","adjective":"woven","noun":"vent"}
```

## Pipeline integration

### Label pipeline runs

```bash
RUN_NAME=$(vrk moniker)
vrk kv set --ns pipeline current_run "$RUN_NAME"
echo "Starting run: $RUN_NAME" | vrk emit --tag pipeline
# ... pipeline stages ...
echo "Completed run: $RUN_NAME" | vrk emit --tag pipeline
```

### Create labeled temp directories

```bash
WORKDIR="/tmp/$(vrk moniker)"
mkdir -p "$WORKDIR"
# Process files in $WORKDIR
```

### Tag batch jobs with memorable names

```bash
# Each batch gets a human-readable name for incident response
BATCH=$(vrk moniker)
vrk kv set --ns batch "$BATCH" "$(vrk epoch --now)" --ttl 168h
echo "Batch $BATCH started" | vrk emit --tag batch --level info
```

## When it fails

Word pool exhausted (unlikely with default settings):

```bash
$ vrk moniker --count 100000
error: moniker: word pool exhausted
$ echo $?
1
```

Invalid count:

```bash
$ vrk moniker --count 0
usage error: moniker: --count must be >= 1
$ echo $?
2
```
