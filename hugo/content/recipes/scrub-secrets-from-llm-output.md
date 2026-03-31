---
title: "Scrub secrets from LLM output"
meta_title: "Scrub secrets from LLM output - vrk pipeline recipe"
description: "Catches secrets the model echoes back before they reach storage - one leaked API key in kv is a breach. Mask any accidentally leaked secrets before ..."
why: "Catches secrets the model echoes back before they reach storage - one leaked API key in kv is a breach."
body: "Mask any accidentally leaked secrets before storing a result."
slug: "scrub-secrets-from-llm-output"
steps:
  - |-
    vrk prompt --system "summarise this" < doc.txt \
      | vrk mask \
      | vrk kv set summary
tags:
  - "prompt"
  - "mask"
  - "kv"
---

## The problem

LLMs echo things back. If your input contains an API key, a password, or a bearer token, the model might include it in the response. If that response goes into a key-value store, a log file, or a downstream API, you've leaked a secret.

This is not hypothetical. Models trained on code regularly reproduce credential patterns. If you feed deployment logs into an LLM for analysis, any secrets in those logs can appear in the output.

## How the pipeline works

The LLM generates a response. `vrk mask` scans it for secrets using pattern matching (bearer tokens, passwords, API keys) and Shannon entropy analysis (high-entropy strings that look like credentials). Anything matched is replaced with `[REDACTED]`. The cleaned output goes into `vrk kv`.

## Why mask after, not just before

You should mask before sending to the LLM too (to avoid sending secrets to an API). But masking after catches a different class of problem: secrets the model generates or reconstructs from context. The LLM might infer a key format and hallucinate a plausible one, or it might reassemble a credential from fragments. Post-LLM masking is the last line of defense before storage.

The full defensive pipeline masks both sides:

```bash
cat deploy.log | vrk mask | vrk prompt --system "What errors occurred?" | vrk mask | vrk kv set analysis
```

