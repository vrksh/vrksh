---
title: "vrk kv"
description: "Key-value store - SQLite-backed, namespaces, TTL, atomic counters."
tool: kv
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

Your pipeline needs to remember things between runs. A counter that tracks how
many times a job has run. A cache key that expires after an hour. A flag that
prevents duplicate processing. You could write to a file, but then you need
locking, expiry logic, and namespace isolation. You need a key-value store
that's already there.

## The fix

```bash
vrk kv set mykey "myvalue"
```

<!-- output: verify against binary -->

```bash
vrk kv get mykey
```

<!-- output: verify against binary -->

SQLite-backed, so it survives restarts. No server to run. The database lives
at `~/.local/share/vrk/kv.db` by default (override with `VRK_KV_PATH`).

## Walkthrough

### Namespaces

Keep keys isolated between projects or pipeline stages:

```bash
vrk kv set --ns cache page_html "<html>..."
vrk kv get --ns cache page_html
```

### TTL (time-to-live)

Set a key that expires automatically:

```bash
vrk kv set --ns cache result "cached_value" --ttl 1h
```

After one hour, `vrk kv get --ns cache result` returns exit 1 (key not found).

### Atomic counters

```bash
vrk kv incr --ns stats request_count
vrk kv incr --ns stats request_count --by 5
vrk kv decr --ns stats request_count
```

<!-- output: verify against binary -->

Each call returns the new value. Atomic - safe for concurrent pipelines.

### What failure looks like

```bash
vrk kv get nonexistent_key
echo $?
# 1
```

<!-- output: verify against binary -->

Exit 1 when a key doesn't exist. This is intentional - it lets you use
`kv get` as a condition in shell scripts.

## Pipeline example

Cache a fetched page to avoid re-downloading:

```bash
vrk kv get --ns cache page_content || \
  vrk grab https://example.com | tee >(vrk kv set --ns cache page_content --ttl 1h)
```

Track pipeline runs with a counter:

```bash
vrk kv incr --ns pipeline run_count | vrk emit --tag pipeline --msg "run started"
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--ns` | | string | `default` | Namespace |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |
| `--ttl` | | duration | `0` | Expiry duration (set only); 0 = no expiry |
| `--dry-run` | | bool | `false` | Print intent without writing (set only) |
| `--json` | `-j` | bool | `false` | Emit errors as JSON (get, incr, decr) |
| `--by` | | int | `1` | Delta for incr/decr (must be >= 1) |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Key not found, not a number, or database error |
| 2 | Usage error - unknown subcommand, missing args |
