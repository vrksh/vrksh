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
