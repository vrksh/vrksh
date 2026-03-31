---
title: "Retry flaky API call"
meta_title: "Retry flaky API call - vrk pipeline recipe"
description: "Transient 500s don't kill the pipeline - coax retries with backoff so one bad request doesn't stop the run. Wrap an LLM prompt in coax for ..."
why: "Transient 500s don't kill the pipeline - coax retries with backoff so one bad request doesn't stop the run."
body: "Wrap an LLM prompt in coax for exponential backoff retries."
slug: "retry-flaky-api-call"
steps:
  - |-
    vrk coax --times 3 --backoff exp:1s --on 1 \
      -- vrk prompt --system "Summarise this" < doc.txt
tags:
  - "coax"
  - "prompt"
---

## The problem

API calls fail. Rate limits, transient 500s, network blips, cold starts. In a pipeline processing hundreds of documents, a single failure shouldn't kill the entire run. But without retry logic, it does - the first 429 or 503 stops everything.

The usual fix is a retry decorator in Python (tenacity, backoff). But that only works inside Python. If your pipeline is a shell script, a cron job, or an agent calling tools via subprocess, you need retry logic that wraps any command.

## How the pipeline works

`vrk coax` runs the command after `--`. If it exits with code 1 (matching `--on 1`), coax waits and retries. The backoff is exponential starting at 1 second: first retry after 1s, second after 2s, third after 4s.

If all retries fail, `vrk coax` exits with the last command's exit code. The pipeline stops cleanly.

## What --on means

`--on 1` tells coax to only retry on exit code 1 (runtime errors like API failures). Exit code 2 (usage errors like bad flags) is never retried - there's no point retrying a command that will always fail the same way.

## Wrapping a full pipeline

Retry an entire multi-step pipeline by wrapping it in `sh -c`:

```bash
vrk coax --times 3 --backoff exp:2s --on 1 -- \
  sh -c 'cat doc.txt \
    | vrk tok --check 8000 \
    | vrk prompt --system "Summarise" \
    | vrk validate --schema out.json'
```

