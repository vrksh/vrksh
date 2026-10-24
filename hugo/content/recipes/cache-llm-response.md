---
title: "Cache LLM response"
meta_title: "Cache LLM response - vrk pipeline recipe"
description: "Avoids duplicate API calls for identical prompts - the content hash keys the cache so reruns are free. Hash the input, look it up in kv, call the ..."
why: "Avoids duplicate API calls for identical prompts - the content hash keys the cache so reruns are free."
body: "Hash the input, look it up in kv, call the model only on a miss."
slug: "cache-llm-response"
steps:
  - "K=\"llm:$(vrk digest --bare < doc.txt)\""
  - |-
    vrk kv get "$K" 2>/dev/null || {
      R=$(vrk prompt < doc.txt) && printf '%s\n' "$R" | vrk kv set "$K" && echo "$R"
    }
tags:
  - "prompt"
  - "kv"
  - "digest"
---

## The problem

LLM API calls are slow and expensive. If you run the same prompt against the same document twice - during development, in a retry loop, or in a pipeline that restarts - you pay for a response you already have.

## How the pipeline works

The cache key is the content hash. `vrk digest --bare` prints the SHA-256 of `doc.txt` with no `sha256:` prefix. Same bytes in, same key out. Change one character in the document and you get a new key.

`vrk kv get` looks the key up. On a hit it prints the stored answer and exits 0, so the `||` branch never runs. On a miss it exits 1, and the braces run: call `vrk prompt`, store the answer under the key, and print it.

```bash
K="llm:$(vrk digest --bare < doc.txt)"
vrk kv get "$K" 2>/dev/null || {
  R=$(vrk prompt < doc.txt) && printf '%s\n' "$R" | vrk kv set "$K" && echo "$R"
}
```

If `vrk prompt` fails, `R` is never stored and the group exits non-zero. A failed call is never cached.

## What the key covers

The key only hashes the document. If you change the `--system` text or the model, hash those in too:

```bash
K="llm:$(cat doc.txt system.txt | vrk digest --bare)"
```

## Why this works

`vrk kv` is a SQLite-backed key-value store on disk. No Redis, no external service. The cache survives restarts and reruns. A lookup takes microseconds, an API call takes seconds. Add `--ttl 24h` to `kv set` if you want entries to expire.

