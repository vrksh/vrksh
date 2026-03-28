---
title: "vrk kv"
description: "Key-value store - SQLite-backed, namespaces, TTL, atomic counters."
tool: kv
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

Your pipeline runs on a schedule. Between runs, you need to remember
state -- the last processed item, a counter, a cached response. There is
nowhere to put it without spinning up a database or parsing flat files
manually.

## The fix

Six subcommands: `set`, `get`, `del`, `list`, `incr`, `decr`.

```bash
$ vrk kv set last_run "2024-03-28T10:00:00Z"

$ vrk kv get last_run
2024-03-28T10:00:00Z

$ vrk kv incr run_count
1

$ vrk kv incr run_count
2

$ vrk kv list
last_run
run_count

$ vrk kv del last_run
```

SQLite-backed, so it survives restarts. No server to run. The database
lives at `~/.vrk.db` by default (override with `VRK_KV_PATH`).

When a key doesn't exist:

```bash
$ vrk kv get nonexistent
error: kv get: key not found
$ echo $?
1
```

Exit 1 -- use this as a condition in shell scripts.

## TTL: auto-expiring keys

```bash
vrk kv set --ttl 1h cache:response "the cached value"
```

The key deletes itself after one hour. After expiry,
`vrk kv get cache:response` returns exit 1 (key not found).
Use for caches, rate limit windows, and temporary state.

## Namespacing

```bash
$ vrk kv set --ns agent-1 status "running"
$ vrk kv set --ns agent-2 status "idle"
$ vrk kv list --ns agent-1
status
```

Namespaces prevent key collisions between pipelines. Each namespace is
fully isolated -- keys in `agent-1` are invisible to `agent-2`.

## In a pipeline

A cron-style pipeline that processes only new items:

```bash
LAST=$(vrk kv get --ns ingest last_id 2>/dev/null || echo "0")
curl -s "https://api.example.com/items?since=$LAST" \
  | vrk jsonl \
  | while read -r line; do
      echo "$line" | vrk prompt --system "Classify this item."
    done
vrk kv set --ns ingest last_id "$(date -u +%s)"
```

Cache a fetched page to avoid re-downloading:

```bash
vrk kv get --ns cache page_content || \
  vrk grab https://example.com | tee >(vrk kv set --ns cache page_content --ttl 1h)
```

## Flags

| Flag | Short | Subcommands | Type | Default | Description |
|------|-------|-------------|------|---------|-------------|
| `--ns` | | all | string | `default` | Namespace |
| `--ttl` | | set | duration | `0` | Expiry duration (e.g. `1h`, `30m`); 0 = no expiry |
| `--by` | | incr, decr | int | `1` | Delta (must be >= 1) |
| `--dry-run` | | set | bool | `false` | Print intent without writing |
| `--json` | `-j` | get, incr, decr | bool | `false` | Emit output as JSON |
| `--quiet` | `-q` | all | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Key not found, value not a number (incr/decr), or database error |
| 2 | Usage error - unknown subcommand, missing args |
