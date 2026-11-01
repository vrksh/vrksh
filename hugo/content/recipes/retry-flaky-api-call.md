---
title: "Retry flaky API call"
meta_title: "Retry flaky API call - vrk pipeline recipe"
description: "A transient 500 does not stop the run, because coax retries the call with backoff. Retry an LLM call with exponential backoff, and fail the attempt ..."
why: "A transient 500 does not stop the run, because coax retries the call with backoff."
body: "Retry an LLM call with exponential backoff, and fail the attempt when the response does not match a schema."
slug: "retry-flaky-api-call"
steps:
  - |-
    vrk coax --times 3 --backoff exp:1s --on 1 \
      -- bash -c 'set -o pipefail; vrk prompt --system "Return JSON like {\"summary\":\"string\"}" < doc.txt \
        | vrk validate --schema "{\"summary\":\"string\"}" --strict'
tags:
  - "coax"
  - "prompt"
---

## The problem

API calls fail. Rate limits, transient 500s, network blips, cold starts. In a pipeline processing hundreds of documents, a single failure shouldn't kill the entire run. But without retry logic, it does - the first 429 or 503 stops everything.

The usual fix is a retry decorator in Python (tenacity, backoff). But that only works inside Python. If your pipeline is a shell script, a cron job, or an agent calling tools via subprocess, you need retry logic that wraps any command.

## How the pipeline works

`vrk coax` runs the command after `--`. It runs the command directly, not through a shell, so pipes and redirects need a shell, as above.

`set -o pipefail` matters. A shell pipeline exits with the status of its last command. When the API returns a 500, `vrk prompt` exits 1 and prints nothing, `vrk validate` gets empty input and exits 0, and without pipefail coax would see success and never retry. Use `bash -c`, not `sh -c`: some `sh` builds (dash on older Debian and Ubuntu) have no pipefail.

Here the retried unit is the prompt plus the check. `vrk validate --strict` exits 1 when the response is not valid JSON with a string `summary` key. Without `--strict`, validate only warns and exits 0, and coax would never see a failure. If the command exits with code 1 (matching `--on 1`), coax waits and retries. The backoff is exponential starting at 1 second: first retry after 1s, second after 2s, third after 4s.

If all retries fail, `vrk coax` exits with the last command's exit code. The pipeline stops cleanly.

## What --on means

`--on 1` tells coax to only retry on exit code 1 (runtime errors like API failures). Exit code 2 (usage errors like bad flags) is never retried - there's no point retrying a command that will always fail the same way.

## Wrapping a full pipeline

Retry an entire multi-step pipeline by wrapping it in `bash -c`. Check the token budget first, outside the retry: an oversized document fails the same way every time, so retrying it only adds backoff.

```bash
vrk tok --check 8000 < doc.txt > /dev/null && \
vrk coax --times 3 --backoff exp:2s --on 1 -- \
  bash -c 'set -o pipefail; cat doc.txt \
    | vrk mask \
    | vrk prompt --system "Summarise as JSON: {\"summary\":\"string\"}" \
    | vrk validate --schema "{\"summary\":\"string\"}" --strict'
```

