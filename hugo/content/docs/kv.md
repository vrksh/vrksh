---
title: "vrk kv"
description: "key-value store - SQLite-backed, namespaces, TTL, atomic counters."
tool: kv
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

A persistent key-value store for pipelines, backed by SQLite. You can store state between runs - cursors, counters, timestamps - with namespaces to keep things isolated and TTL for automatic expiry. Supports atomic increment and decrement for safe concurrent use.

## The problem

Your pipeline needs to remember state between runs - a cursor, a count, a last-seen timestamp. You write to a flat file, but concurrent runs corrupt it. You add flock, but that does not work on NFS. You reach for Redis, but that is a whole server for one key.

## Before and after

**Before**

```bash
echo "12345" > /tmp/last_cursor.txt
CURSOR=$(cat /tmp/last_cursor.txt)
# no atomic increment, no TTL, no namespaces
# concurrent writes corrupt the file
```

**After**

```bash
vrk kv set cursor "12345" --ttl 1h
vrk kv get cursor
```

## Example

```bash
vrk kv set --ns cache mykey "myvalue" --ttl 1h
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Key not found, not a number, or database error |
| 2 | Usage error - unknown subcommand, missing args |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--ns` |   | string | Namespace (default "default") |
| `--quiet` | -q | bool | Suppress stderr output |
| `--ttl` |   | duration | Expiry duration (set only); 0 = no expiry |
| `--dry-run` |   | bool | Print intent without writing (set only) |
| `--json` | -j | bool | Emit errors as JSON (get, incr, decr) |
| `--by` |   | int | Delta for incr/decr (must be >= 1) |

