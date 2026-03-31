---
title: "Token-checked LLM call"
meta_title: "Token-checked LLM call - vrk pipeline recipe"
description: "Prevents silent truncation - the model never sees a prompt it can only half-fit. Count tokens before sending to an LLM - abort if too large."
why: "Prevents silent truncation - the model never sees a prompt it can only half-fit."
body: "Count tokens before sending to an LLM - abort if too large."
slug: "token-checked-llm-call"
steps:
  - |-
    cat prompt.txt \
      | vrk tok --check 4000 \
      | vrk prompt --system "Summarise this."
tags:
  - "tok"
  - "prompt"
---

## The problem

LLMs have context windows. When your input exceeds the limit, the API either truncates silently or returns an error after you've already waited for the response and spent tokens on the request. In a pipeline, this is worse - downstream steps process garbage output without knowing the input was incomplete.

This is the most expensive bug in LLM pipelines because it looks like success. The API returns 200, the model responds, but the answer is based on a truncated document.

## How the pipeline works

`vrk tok --check 4000` reads stdin, counts tokens, and makes a decision. If the count is 4000 or under, it passes the text through unchanged to stdout. If over, it exits 1 with empty stdout. The pipe stops. `vrk prompt` never runs.

No wasted API call. No truncated output. No silent failure.

## When it fails

When the token count exceeds the budget, `vrk tok` exits 1 and the pipeline stops immediately. The exit code propagates - your shell script, CI job, or agent sees a non-zero exit and knows the step failed. You can catch this and handle it (chunk the document, summarize a section, or report the error) instead of processing bad output.

## Variations

Gate before a batch job to skip oversized documents:

```bash
for f in docs/*.md; do
  cat "$f" | vrk tok --check 8000 | vrk prompt --system "Extract key points" >> output.jsonl 2>/dev/null
done
```

Use `--json` for structured error reporting:

```bash
cat prompt.txt | vrk tok --check 4000 --json
# Over budget: {"error":"tok: 6201 tokens exceeds budget of 4000","code":1}
```

