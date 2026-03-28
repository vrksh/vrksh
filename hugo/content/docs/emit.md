---
title: "vrk emit"
description: "Structured logger - wraps stdin lines as JSONL log records"
tool: emit
group: utilities
mcp_callable: true
noindex: false
---

## The problem

Your scripts write unstructured text to stdout. A `grep` result, a curl response, a status message from a long-running job. Downstream, your log aggregator, your monitoring system, or the next tool in the pipeline expects JSONL with timestamps, levels, and consistent field names. You need a bridge between the human-readable line and the machine-parseable record - without rewriting every script to use a logging library.

`vrk emit` is that bridge. Each line in becomes a JSON object out, stamped with a UTC timestamp and a log level. One command, added to the pipe, turns any stream into structured logs.

## The fix

```bash
some-script.sh | vrk emit --level info --tag my-script
```

Or pass a single message as a positional argument:

```bash
vrk emit "deployment complete"
```

## Walkthrough

**Basic usage - wrap every line**

Every non-empty line from stdin becomes one JSONL record on stdout. Empty lines are skipped.

```bash
cat deploy.log | vrk emit
```

Each record looks like:

```json
{"ts":"2026-03-28T14:22:01.000Z","level":"info","msg":"Starting deployment"}
```

Timestamps are UTC ISO 8601 with millisecond precision. The `level` defaults to `info`.

**Tagging output by source**

The `--tag` flag adds a `tag` field to every record. Use it to identify the source when merging streams from multiple tools.

```bash
vrk grab https://example.com 2>&1 | vrk emit --level warn --tag grab
```

The record gains a `"tag":"grab"` field between `level` and `msg`.

**Auto-detecting level from line content**

When your source already uses prefixes like `ERROR:` or `WARN:`, `--parse-level` detects them and sets the level automatically. The prefix is stripped from `msg`. Unrecognised prefixes fall back to the `--level` value.

```bash
cat server.log | vrk emit --parse-level --tag server
```

A line like `ERROR: connection refused` becomes:

```json
{"ts":"...","level":"error","tag":"server","msg":"connection refused"}
```

Recognised prefixes: `ERROR`, `WARNING`, `WARN`, `INFO`, `DEBUG` - case-insensitive, followed by `:`, a space, or end of line.

**Merging extra fields with `--msg`**

When `--msg` is set, the fixed message overrides `msg` for every record, and each stdin line is parsed as a JSON object whose fields are merged into the record. Use this when stdin is already structured but missing the standard log envelope.

```bash
$ echo '{"user_id":"abc","action":"login"}' | vrk emit --msg "user event" --level info --tag auth
{"ts":"2026-03-28T20:48:34.376Z","level":"info","tag":"auth","msg":"user event","action":"login","user_id":"abc"}
```

<!-- output varies: ts field reflects current time -->

Core field names in the merged JSON (`ts`, `level`, `tag`, `msg`) are suppressed - your flag values always win.

**Composing with coax for retry tracking**

```bash
vrk coax --times 3 -- vrk grab https://example.com 2>&1 | vrk emit --parse-level --tag grab
```

`coax` retries the command up to three times. Any output - from successful attempts and failures alike - is wrapped as structured log records.

## Pipeline example

```bash
vrk coax --times 3 -- vrk grab https://example.com 2>&1 | vrk emit --parse-level --tag grab
```

Retry a URL fetch up to three times, merge stdout and stderr, auto-detect log levels from any `ERROR:` or `WARN:` prefixes, and emit every line as a structured JSONL record tagged `grab`. Pipe the result to a log file or to `vrk kv set` to store the last record.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--level` | `-l` | string | `"info"` | Log level for all records: `debug`, `info`, `warn`, `error` |
| `--tag` | | string | `""` | Value for the `tag` field on every record; field omitted when empty |
| `--msg` | | string | `""` | Fixed message override; when set, stdin lines are parsed as JSON and merged as extra fields |
| `--parse-level` | | bool | false | Auto-detect `level` from `ERROR`/`WARN`/`WARNING`/`INFO`/`DEBUG` line prefixes |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success - all non-empty lines emitted as JSONL records |
| 1 | Runtime error - stdin scanner error or write failure |
| 2 | Usage error - interactive TTY with no positional arg, or unknown `--level` value |
