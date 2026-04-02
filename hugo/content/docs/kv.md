---
title: "vrk kv"
description: "Persistent key-value store for pipelines. SQLite-backed, with namespaces, TTL, and atomic counters."
og_title: "vrk kv - SQLite key-value store for shell pipelines"
tool: kv
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## The problem

A pipeline needs to remember state between runs: a cursor position, a processed count, a last-run timestamp. Writing to a flat file works until two cron jobs overlap. Both read the same cursor. Both process the same 200 items. Redis solves concurrency but is a whole server for one key.

## The solution

`vrk kv` is a persistent key-value store backed by SQLite. Namespaces isolate different pipelines. TTL handles automatic expiry. Atomic `incr`/`decr` is safe under concurrent access. The database lives at `~/.vrk.db` (override with `VRK_KV_PATH`). No server, no config.

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
vrk kv set cursor "12345" --ttl 24h
vrk kv get cursor
vrk kv incr --ns pipeline processed
```

## Example

```bash
vrk kv set --ns nightly-pipeline last_run "$(vrk epoch --now)" --ttl 24h
```

## Exit codes

| Code | Meaning                                        |
|------|------------------------------------------------|
| 0    | Success                                        |
| 1    | Key not found, not a number, or database error |
| 2    | Usage error - unknown subcommand, missing args |

## Flags

| Flag        | Short | Type     | Description                               |
|-------------|-------|----------|-------------------------------------------|
| `--ns`      |       | string   | Namespace (default "default")             |
| `--quiet`   | -q    | bool     | Suppress stderr output                    |
| `--ttl`     |       | duration | Expiry duration (set only); 0 = no expiry |
| `--dry-run` |       | bool     | Print intent without writing (set only)   |
| `--json`    | -j    | bool     | Emit errors as JSON (get, incr, decr)     |
| `--by`      |       | int      | Delta for incr/decr (must be >= 1)        |


<!-- notes - edit in notes/kv.notes.md -->

## Subcommands

### set - store a value

```bash
$ vrk kv set mykey "hello world"
$ vrk kv set --ns cache api_response "..." --ttl 1h
```

Values can also come from stdin when the value argument is omitted:

```bash
echo "pipeline output" | vrk kv set --ns results latest
```

### get - retrieve a value

```bash
$ vrk kv get mykey
hello world
```

Exits 1 if the key doesn't exist or has expired:

```bash
$ vrk kv get nonexistent
error: kv get: key not found
$ echo $?
1
```

### del - delete a key

```bash
vrk kv del mykey
```

Silent if the key doesn't exist. Always exits 0.

### list - show all keys

```bash
$ vrk kv list --ns cache
api_response
last_fetch
status
```

Lists all keys in the namespace, sorted alphabetically. Expired keys are not shown.

### incr / decr - atomic counters

```bash
$ vrk kv incr counter
1
$ vrk kv incr counter
2
$ vrk kv incr counter --by 10
12
$ vrk kv decr counter
11
```

Missing keys start at 0. Increments are atomic - safe for concurrent access from multiple processes.

## Flag details

### --ns (namespace)

Isolates keys between different pipelines or tasks. Defaults to "default".

```bash
vrk kv set --ns daily-summary status "running"
vrk kv set --ns weekly-report status "pending"
vrk kv get --ns daily-summary status    # "running"
vrk kv get --ns weekly-report status    # "pending"
```

### --ttl (time to live)

Automatically expires keys after a duration. Supports Go duration format: `1s`, `5m`, `24h`, `168h` (7 days).

```bash
vrk kv set --ns cache response "$DATA" --ttl 1h
# After 1 hour:
vrk kv get --ns cache response
# error: kv get: key not found
```

### --dry-run

Shows what `set` would do without writing:

```bash
$ vrk kv set --ns prod important_key "value" --dry-run
kv: would set important_key = "value" (ns: prod, ttl: 0s)
```

## A complete agent loop

```bash
#!/bin/bash
# Morning: mark pipeline started
vrk kv set --ns daily-summary run_at "$(vrk epoch --now)"
vrk kv set --ns daily-summary status "running"

# Process items, tracking count
for item in $(cat queue.txt); do
  process "$item"
  vrk kv incr --ns daily-summary processed
done

# Evening: check what happened
echo "Status: $(vrk kv get --ns daily-summary status)"
echo "Processed: $(vrk kv get --ns daily-summary processed)"

# Store the final result with a 7-day TTL
vrk kv set --ns daily-summary last_result "$OUTPUT" --ttl 168h
vrk kv set --ns daily-summary status "complete"
```

## Pipeline integration

### Track progress in a batch pipeline

```bash
# Process URLs, track successes and failures in kv
vrk kv set --ns batch run_id "$(vrk moniker --seed $RANDOM)"
for url in $(cat urls.txt); do
  if vrk grab "$url" | vrk prompt --system 'Summarize' > /dev/null 2>&1; then
    vrk kv incr --ns batch success
  else
    vrk kv incr --ns batch failure
  fi
done
echo "Done: $(vrk kv get --ns batch success) ok, $(vrk kv get --ns batch failure) failed"
```

### Cache LLM responses to avoid re-processing

```bash
# Check cache before calling the LLM
KEY=$(echo "$INPUT" | vrk digest --bare)
CACHED=$(vrk kv get --ns llm-cache "$KEY" 2>/dev/null)
if [ -n "$CACHED" ]; then
  echo "$CACHED"
else
  RESULT=$(echo "$INPUT" | vrk prompt --system 'Analyze this')
  vrk kv set --ns llm-cache "$KEY" "$RESULT" --ttl 24h
  echo "$RESULT"
fi
```

### Resume a pipeline from where it left off

```bash
# Store cursor position; resume after crash
CURSOR=$(vrk kv get --ns ingest cursor 2>/dev/null || echo "0")
cat data.jsonl | tail -n +$((CURSOR + 1)) | \
  while IFS= read -r line; do
    process "$line"
    CURSOR=$((CURSOR + 1))
    vrk kv set --ns ingest cursor "$CURSOR"
  done
```

## When it fails

Key not found:

```bash
$ vrk kv get nonexistent
error: kv get: key not found
$ echo $?
1
```

Missing subcommand:

```bash
$ vrk kv
usage error: kv: subcommand required (set, get, del, list, incr, decr)
$ echo $?
2
```

Increment on a non-numeric value:

```bash
$ vrk kv set mykey "not a number"
$ vrk kv incr mykey
error: kv incr: value is not an integer
$ echo $?
1
```
