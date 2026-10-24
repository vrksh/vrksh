---
title: "Structured secret-free logs"
meta_title: "Structured secret-free logs - vrk pipeline recipe"
description: "Plain script output becomes searchable JSON logs, and the secrets never reach the log file. Mask the output of a script, then wrap each line as a ..."
why: "Plain script output becomes searchable JSON logs, and the secrets never reach the log file."
body: "Mask the output of a script, then wrap each line as a JSON record with a level and a tag."
slug: "structured-secret-free-logs"
steps:
  - "./deploy.sh 2>&1 | vrk mask | vrk emit --tag deploy --parse-level"
tags:
  - "mask"
  - "emit"
---

## The problem

Deploy scripts, cron jobs and build steps print plain text. You want it in a log system as JSON with a level and a source. You also know that somewhere in that text there is a token or a password, and that log files live longer and travel further than you think.

## How the pipeline works

`2>&1` joins stderr into the stream so errors are logged too. `vrk mask` replaces secrets with `[REDACTED]` before the text goes anywhere else. `vrk emit` turns each line into one JSON record.

`--tag deploy` adds `"tag":"deploy"` to every record. `--parse-level` reads the level from the start of each line (`ERROR`, `WARN`, `WARNING`, `INFO`, `DEBUG`), sets the `level` field, and removes the prefix from the message. Lines with no prefix get `info`.

The output:

```
{"ts":"2026-10-24T10:01:02.118Z","level":"info","tag":"deploy","msg":"starting deploy of web-7"}
{"ts":"2026-10-24T10:01:02.119Z","level":"info","tag":"deploy","msg":"api_key=[REDACTED] loaded"}
{"ts":"2026-10-24T10:01:02.119Z","level":"warn","tag":"deploy","msg":"health check slow"}
{"ts":"2026-10-24T10:01:02.120Z","level":"error","tag":"deploy","msg":"migration 0042 failed"}
```

## Order matters

Mask first, then wrap. If `emit` ran first, the secret would sit inside a JSON string, and you would be relying on the masker to find it there.

## Keeping the exit code

A pipe returns the exit code of its last command. `./deploy.sh` can fail and the pipeline still exits 0. In bash, turn on `pipefail` if the script's status matters:

```bash
set -o pipefail
./deploy.sh 2>&1 | vrk mask | vrk emit --tag deploy --parse-level >> deploy.jsonl
```

