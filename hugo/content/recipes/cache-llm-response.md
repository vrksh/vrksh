---
title: "Cache LLM response"
meta_title: "Cache LLM response - vrk pipeline recipe"
description: "Avoids duplicate API calls for identical prompts - the hash keys the cache so reruns are free. Send a prompt, get the request hash, and store the ..."
why: "Avoids duplicate API calls for identical prompts - the hash keys the cache so reruns are free."
body: "Send a prompt, get the request hash, and store the response in kv."
slug: "cache-llm-response"
steps:
  - "RESULT=$(cat doc.txt | vrk prompt --json)"
  - "HASH=$(echo \"$RESULT\" | jq -r '.request_hash')"
  - "echo \"$RESULT\" | jq -r '.response' | vrk kv set \"cache:$HASH\""
tags:
  - "prompt"
  - "kv"
---

## The problem

LLM API calls are slow and expensive. If you're running the same prompt against the same document twice - during development, in a retry loop, or in a pipeline that restarts - you're paying for a response you already have.

## How the pipeline works

`vrk prompt --json` returns the LLM response as a JSON envelope that includes a `request_hash` - a deterministic hash of the model, system prompt, and input. You use this hash as the cache key in `vrk kv`. On the next run, check the cache first:

```bash
HASH=$(cat doc.txt | vrk tok --json | jq -r '.hash // empty')
CACHED=$(vrk kv get "cache:$HASH" 2>/dev/null)
if [ -n "$CACHED" ]; then
  echo "$CACHED"
else
  RESULT=$(cat doc.txt | vrk prompt --json)
  HASH=$(echo "$RESULT" | jq -r '.request_hash')
  echo "$RESULT" | jq -r '.response' | vrk kv set "cache:$HASH"
fi
```

## Why this works

`vrk kv` is a SQLite-backed key-value store that lives on disk. No Redis, no external service. The cache survives process restarts, script reruns, and pipeline failures. It's local and instant - a kv lookup takes microseconds compared to seconds for an API call.

