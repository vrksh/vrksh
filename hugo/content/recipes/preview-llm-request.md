---
title: "Preview LLM request"
meta_title: "Preview LLM request - vrk pipeline recipe"
description: "You see the exact request, after masking, before any money or secrets leave your machine. Add --explain to vrk prompt to print the request as a curl ..."
why: "You see the exact request, after masking, before any money or secrets leave your machine."
body: "Add --explain to vrk prompt to print the request as a curl command instead of sending it."
slug: "preview-llm-request"
steps:
  - "cat deploy.log | vrk mask | vrk prompt --system \"What failed?\" --explain"
tags:
  - "mask"
  - "prompt"
---

## The problem

A pipeline builds a prompt from files, logs and other tools. By the time it reaches the model you have lost sight of what is in it. Did the secret get masked? Did the system prompt land where you wanted? Is the log really in there? The only way to find out used to be sending it and reading the bill.

## How the pipeline works

`vrk prompt --explain` builds the full request, prints it as a `curl` command, and stops. It makes no API call and needs no network. The key is shown as `$ANTHROPIC_API_KEY` or `$OPENAI_API_KEY`, never the real value.

Put `vrk mask` in front and the printed request shows the log with secrets replaced by `[REDACTED]`. That is the text the model would have received.

```
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  ...
  -d '{"model":"claude-sonnet-4-6", ... "system":"What failed?", ... [REDACTED] ...}'
```

## Going live

Take `--explain` off the same command and it runs for real. Nothing else changes, so what you reviewed is what gets sent.

```bash
cat deploy.log | vrk mask | vrk prompt --system "What failed?"
```

## When to use it

Use it the first time you wire up a pipeline, when you change the system prompt, and in CI to check that masking is still in the path. `--explain` also works with `--schema`, so you can see what the schema turns into for each provider.

