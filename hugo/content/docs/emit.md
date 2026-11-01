---
title: "vrk emit"
description: "vrk emit turns plain text lines into JSONL log records with timestamps, levels, and tags. Pipe the output of any command through it."
meta_lead: "vrk emit wraps plain text lines into structured JSONL log records with ISO timestamps and log levels."
og_title: "vrk emit - structured JSONL logging for shell pipelines"
tool: emit
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk emit wraps plain text lines into structured JSONL log records with ISO timestamps and log levels.

## The problem

A five-stage pipeline prints 10,000 lines of unstructured text. Stage 3 fails. There are no timestamps and no way to tell which stage produced which line. `echo "[$(date)] ERROR: ..."` adds inconsistent formatting that is hard to grep and impossible to aggregate.

## The solution

`vrk emit` wraps plain text lines into structured JSONL log records with ISO timestamps, log levels, and tags. Pipe the output of any command through it to get logs a program can parse. `--parse-level` auto-detects ERROR/WARN/INFO prefixes from existing output. `--tag` labels records by pipeline stage so you can filter by stage later.

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

`vrk emit` turns each line of stdin into a structured JSONL log record with a timestamp and a log level.

### Basic structured logging

```bash
$ echo 'Application started' | vrk emit
{"ts":"...","level":"info","msg":"Application started"}
```

Every line becomes a JSONL record (one JSON object per line) with an ISO timestamp and a log level.

### Set log level

```bash
$ echo 'Connection refused' | vrk emit --level error
{"ts":"...","level":"error","msg":"Connection refused"}
```

Levels: `debug`, `info` (default), `warn`, `error`.

### Auto-detect levels from existing output (--parse-level)

```bash
$ printf 'Application started\nERROR: database connection failed\nWARNING: cache miss rate high\nProcessing complete\n' | vrk emit --parse-level
{"ts":"...","level":"info","msg":"Application started"}
{"ts":"...","level":"error","msg":"database connection failed"}
{"ts":"...","level":"warn","msg":"cache miss rate high"}
{"ts":"...","level":"info","msg":"Processing complete"}
```

emit recognizes the prefixes `ERROR`, `WARN`, `WARNING`, `INFO`, and `DEBUG`. It removes the prefix from the message.

### Tag records by pipeline stage

```bash
$ echo 'Fetching data' | vrk emit --tag fetch
{"ts":"...","level":"info","tag":"fetch","msg":"Fetching data"}
```

Use tags to filter logs by stage: `jq 'select(.tag == "fetch")'`.

### Combined: tag + parse-level

```bash
$ printf 'INFO Starting deployment\nERROR Container health check failed\nWARN Rolling back to previous version\n' | vrk emit --tag deploy --parse-level
{"ts":"...","level":"info","tag":"deploy","msg":"Starting deployment"}
{"ts":"...","level":"error","tag":"deploy","msg":"Container health check failed"}
{"ts":"...","level":"warn","tag":"deploy","msg":"Rolling back to previous version"}
```

### Override message field (--msg)

If stdin is JSON, emit merges its fields into the record as extra fields:

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

<!-- doccheck: skip (TTY-only behaviour) -->
```bash
$ vrk emit
usage error: emit: no input: pipe lines to stdin
$ echo $?
2
```

Invalid log level:

```bash
$ echo 'test' | vrk emit --level critical
usage error: emit: unknown level "critical": must be debug, info, warn, or error
$ echo $?
2
```
