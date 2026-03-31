---
title: "Batch LLM with rate limiting"
meta_title: "Batch LLM with rate limiting - vrk pipeline recipe"
description: "Prevents failure at job 847 of 10,000 - throttle paces the pipeline, tok gates each doc before the API call wastes a request. Process a large ..."
why: "Prevents failure at job 847 of 10,000 - throttle paces the pipeline, tok gates each doc before the API call wastes a request."
body: "Process a large document set without hitting API rate limits. Safe to rerun."
slug: "batch-llm-with-rate-limiting"
steps:
  - |-
    for f in docs/*.md; do
      cat "$f" \
        | vrk tok --check 8000 \
        | vrk throttle --rate 60/m \
        | vrk prompt --json \
        | vrk kv set "result:$(basename $f)"
    done
tags:
  - "tok"
  - "throttle"
  - "prompt"
  - "kv"
---

## The problem

You have 10,000 documents to process with an LLM. Without rate limiting, you'll hit the API's requests-per-minute limit around document 60 and get 429 errors for the rest. Without token gating, oversized documents waste API calls that fail or return truncated results. Without result storage, a crash at document 5,000 means starting over.

## How the pipeline works

The loop processes one document at a time. For each file:

1. `vrk tok --check 8000` verifies the document fits in the context window. If not, exit 1 - skip this doc.
2. `vrk throttle --rate 60/m` enforces a maximum of 60 requests per minute. If you're going too fast, it sleeps until the next slot opens.
3. `vrk prompt --json` sends the document to the LLM and returns the response as JSON.
4. `vrk kv set` stores the result keyed by filename.

Because results are stored in `vrk kv`, you can check which documents have already been processed and skip them on rerun. The pipeline is idempotent.

## Making it resumable

Add a cache check at the start of each iteration:

```bash
for f in docs/*.md; do
  KEY="result:$(basename $f)"
  vrk kv get "$KEY" >/dev/null 2>&1 && continue
  cat "$f" | vrk tok --check 8000 \
    | vrk throttle --rate 60/m \
    | vrk prompt --json \
    | vrk kv set "$KEY"
done
```

If the script crashes at document 5,000, restart it. The first 4,999 documents are already in kv and get skipped in milliseconds.

