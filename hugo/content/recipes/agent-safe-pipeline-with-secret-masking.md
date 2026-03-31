---
title: "Agent-safe pipeline with secret masking"
meta_title: "Agent-safe pipeline with secret masking - vrk pipeline recipe"
description: "When agents handle user data, mask before any logging or storage - secrets should never reach an LLM or a kv store unredacted. Full guard pipeline - ..."
why: "When agents handle user data, mask before any logging or storage - secrets should never reach an LLM or a kv store unredacted."
body: "Full guard pipeline - mask secrets, gate on token budget, prompt, validate output."
slug: "agent-safe-pipeline-with-secret-masking"
steps:
  - |-
    cat user-input.txt \
      | vrk mask \
      | vrk tok --check 6000 \
      | vrk prompt --system "Summarise" \
      | vrk validate --schema summary.json
tags:
  - "mask"
  - "tok"
  - "prompt"
  - "validate"
---

## The problem

AI agents process user data. That data might contain API keys, passwords, tokens, or PII. If any of that reaches the LLM, it's in the provider's logs. If it reaches your storage unredacted, it's a data breach waiting to happen.

The safe approach is defense in depth: mask before the LLM sees it, gate on size, validate the output shape, and stop the pipeline if anything is wrong.

## How the pipeline works

This is the full guard pipeline - four tools chained together, each handling one concern:

1. `vrk mask` - scans the input for secrets (API keys, bearer tokens, passwords, high-entropy strings) and replaces them with `[REDACTED]`. The LLM never sees the raw credentials.
2. `vrk tok --check 6000` - counts tokens and passes through only if the input fits. Prevents sending oversized documents that would be truncated.
3. `vrk prompt` - sends the cleaned, size-checked input to the LLM.
4. `vrk validate --schema summary.json` - checks that the LLM's response matches your expected schema. If not, exit 1.

If any step fails, the pipeline stops. No partial results, no bad data downstream.

## When to use this pattern

Any time an agent handles data it didn't generate itself. User uploads, webhook payloads, log files, database exports - anything that might contain secrets. The mask-then-prompt pattern should be the default for agent pipelines, not an afterthought.

## Adding post-LLM masking

For maximum safety, mask the output too - LLMs can hallucinate or reconstruct credentials:

```bash
cat user-input.txt \
  | vrk mask \
  | vrk tok --check 6000 \
  | vrk prompt --system "Summarise" \
  | vrk mask \
  | vrk validate --schema summary.json
```

