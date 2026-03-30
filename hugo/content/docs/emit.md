---
title: "vrk emit"
description: "Turn plain text into structured JSONL log records. Add timestamps, levels, and fields to any pipeline output."
og_title: "vrk emit - structured JSONL logging for shell pipelines"
tool: emit
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Your pipeline prints unstructured text to stdout. Errors, warnings, and info messages all look the same. When something breaks at 3 AM, you grep through a flat text file trying to find the ERROR lines mixed in with thousands of INFO lines. No timestamps. No log levels. No structure.

`vrk emit` wraps plain text lines into structured JSONL log records with ISO timestamps, log levels, and optional tags. Pipe any command's output through emit and get machine-parseable logs. Use `--parse-level` to auto-detect ERROR/WARN/INFO prefixes from existing output. Use `--tag` to label records by pipeline stage.

## The problem

Your nightly pipeline runs 5 stages. Each one prints to stdout. When stage 3 fails, you have 10,000 lines of unstructured text with no timestamps and no way to tell which stage produced which line. You add echo statements with dates, but the format is inconsistent and grep-hostile.

## Before and after

**Before**

```bash
echo "[$(date)] INFO: Processing started"
echo "[$(date)] ERROR: Connection failed"
# inconsistent format, hard to parse, no machine-readable structure
```

**After**

```bash
run-pipeline | vrk emit --tag pipeline --parse-level
```

## Example

```bash
run-pipeline 2>&1 | vrk emit --tag deploy --parse-level
```

## Exit codes

| Code | Meaning                                                          |
|------|------------------------------------------------------------------|
| 0    | All non-empty lines emitted as JSONL records                     |
| 1    | Stdin scanner error or write failure                             |
| 2    | Interactive TTY with no positional arg, or unknown --level value |

## Flags

| Flag            | Short | Type   | Description                                                   |
|-----------------|-------|--------|---------------------------------------------------------------|
| `--level`       | -l    | string | Log level: debug, info, warn, error                           |
| `--tag`         |       | string | Value for the tag field on every record                       |
| `--msg`         |       | string | Fixed message override; stdin lines parsed as JSON and merged |
| `--parse-level` |       | bool   | Auto-detect level from ERROR/WARN/INFO/DEBUG line prefixes    |


<!-- notes - edit in notes/emit.notes.md -->

## How it works

### Basic structured logging

```bash
$ echo 'Application started' | vrk emit
{"ts":"2026-03-30T17:48:56.194Z","level":"info","msg":"Application started"}
```

Every line becomes a JSONL record with an ISO timestamp and log level.

### Set log level

```bash
$ echo 'Connection refused' | vrk emit --level error
{"ts":"2026-03-30T17:48:56.194Z","level":"error","msg":"Connection refused"}
```

Levels: `debug`, `info` (default), `warn`, `error`.

### Auto-detect levels from existing output (--parse-level)

```bash
$ printf 'Application started\nERROR: database connection failed\nWARNING: cache miss rate high\nProcessing complete\n' | vrk emit --parse-level
{"ts":"2026-03-30T17:48:56.194Z","level":"info","msg":"Application started"}
{"ts":"2026-03-30T17:48:56.195Z","level":"error","msg":"database connection failed"}
{"ts":"2026-03-30T17:48:56.195Z","level":"warn","msg":"cache miss rate high"}
{"ts":"2026-03-30T17:48:56.195Z","level":"info","msg":"Processing complete"}
```

Recognizes `ERROR`, `WARN`, `WARNING`, `INFO`, and `DEBUG` prefixes. Strips the prefix from the message.

### Tag records by pipeline stage

```bash
$ echo 'Fetching data' | vrk emit --tag fetch
{"ts":"2026-03-30T17:48:56.194Z","level":"info","tag":"fetch","msg":"Fetching data"}
```

Tags let you filter logs by stage: `jq 'select(.tag == "fetch")'`.

### Combined: tag + parse-level

```bash
$ run-deploy.sh 2>&1 | vrk emit --tag deploy --parse-level
{"ts":"...","level":"info","tag":"deploy","msg":"Starting deployment"}
{"ts":"...","level":"error","tag":"deploy","msg":"Container health check failed"}
{"ts":"...","level":"warn","tag":"deploy","msg":"Rolling back to previous version"}
```

### Override message field (--msg)

When stdin contains JSON you want to merge as extra fields:

```bash
echo '{"duration_ms":1234,"status":"ok"}' | vrk emit --msg 'Request completed'
```

## Pipeline integration

### Structured logging for a multi-stage pipeline

```bash
# Each stage tags its own output
cat urls.txt | vrk emit --tag input --level debug | \
  while IFS= read -r record; do
    URL=$(echo "$record" | jq -r '.msg')
    vrk grab "$URL" 2>&1 | vrk emit --tag fetch
  done
```

### Log LLM pipeline results

```bash
# Summarize a document and emit structured results
RESULT=$(cat document.md | vrk prompt --system 'Summarize')
echo "$RESULT" | vrk emit --tag summarize --level info
```

### Monitor a batch with mask + emit

```bash
# Run a pipeline, redact secrets from output, emit as structured logs
./nightly-pipeline.sh 2>&1 | vrk mask | vrk emit --tag nightly --parse-level
```

## When it fails

Interactive terminal with no input:

```bash
$ vrk emit
usage error: emit: no input: pipe text to stdin
$ echo $?
2
```

Invalid log level:

```bash
$ echo 'test' | vrk emit --level critical
usage error: emit: invalid level "critical" (use debug, info, warn, error)
$ echo $?
2
```
