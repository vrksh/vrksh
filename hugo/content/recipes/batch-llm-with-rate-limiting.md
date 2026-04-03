---
title: "Batch LLM with rate limiting"
meta_title: "Batch LLM with rate limiting - vrk pipeline recipe"
description: "Prevents failure at job 847 of 10,000 - throttle paces filenames so each API call respects the rate limit. Process a large document set without ..."
why: "Prevents failure at job 847 of 10,000 - throttle paces filenames so each API call respects the rate limit."
body: "Process a large document set without hitting API rate limits. Safe to rerun."
slug: "batch-llm-with-rate-limiting"
steps:
  - |-
    ls docs/*.md | vrk throttle --rate 60/m \
      | while read -r f; do
          cat "$f" \
            | vrk tok --check 8000 \
            | vrk prompt --system 'Summarize this document.' \
            | vrk kv set "result:$(basename "$f")"
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

`ls` emits one filename per line. `vrk throttle --rate 60/m` releases filenames at 60 per minute. The `while` loop reads each filename as it arrives and processes it:

1. `vrk tok --check 8000` verifies the document fits in the context window. If not, exit 1 and the loop continues to the next file.
2. `vrk prompt` sends the document to the LLM and prints the response.
3. `vrk kv set` stores the result keyed by filename.

Because throttle controls the rate at which filenames enter the loop, each API call is paced. Because results are stored in `vrk kv`, you can check which documents have already been processed and skip them on rerun.

## Making it resumable

Add a cache check at the start of each iteration:

```bash
ls docs/*.md | vrk throttle --rate 60/m \
  | while read -r f; do
      KEY="result:$(basename "$f")"
      vrk kv get "$KEY" >/dev/null 2>&1 && continue
      cat "$f" | vrk tok --check 8000 \
        | vrk prompt --system 'Summarize this document.' \
        | vrk kv set "$KEY"
    done
```

If the script crashes at document 5,000, restart it. The first 4,999 documents are already in kv and get skipped in milliseconds.

