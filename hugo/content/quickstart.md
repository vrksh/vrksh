---
title: "Quickstart"
meta_title: "Quickstart - install vrk and run it in 2 minutes"
description: "Install vrk and run four tools in two minutes with no API key: count tokens, gate on a budget, redact secrets, and convert a timestamp."
noindex: false
---

This page shows how to install vrk and run four tools in about two minutes. You do not need an API key for any of these steps. To try vrk without an install, open the [in-browser playground](/#try).

## Install

```bash
brew tap vrksh/vrksh && brew install vrk
```

For other methods, see the [install page](/install/).

## Run four tools

Count the tokens in a text with `vrk tok`. The default tokenizer is o200k_base.

```bash
$ echo 'Hello, world!' | vrk tok --json
{"tokens":4,"model":"o200k_base"}
```

Stop a pipeline if the text is over a token budget. Here the limit is 3 tokens, so `vrk tok` prints an error and exits 1.

```bash
$ printf 'You are a helpful assistant.' | vrk tok --check 3
tok: 6 tokens exceeds limit of 3
```

Redact secrets from a log with `vrk mask`. The line without a secret stays the same.

```bash
$ printf 'API key: sk-proj-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz\nBearer ghp_FAKE0000example0000notreal0000000000\nPassword: s3cr3t-p4ssw0rd-v3ry-l0ng\nNormal text stays untouched\n' | vrk mask
API key: [REDACTED]
Bearer [REDACTED]
Password: [REDACTED]
Normal text stays untouched
```

Convert a Unix timestamp to ISO 8601 with `vrk epoch`.

```bash
$ vrk epoch 1740009600 --iso
2025-02-20T00:00:00Z
```

## Call an LLM

`vrk prompt` is the only tool here that needs an API key. Set one key:

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

Or use OpenAI:

```bash
export OPENAI_API_KEY=sk-...
```

Then send a prompt:

```bash
echo 'Explain exit codes in one sentence.' | vrk prompt
```

The response goes to stdout and errors go to stderr. The exit code is 0 on success and 1 on failure.

## Build a pipeline

This pipeline fetches a web page, strips the markup, redacts secrets, checks the size, and asks an LLM for a summary:

```bash
vrk grab https://example.com \
  | vrk plain \
  | vrk mask \
  | vrk tok --check 8000 \
  | vrk prompt --system 'Summarize this page in three bullet points.'
```

If the page is over 8,000 tokens, `tok` exits 1 and the pipeline stops before the API call. If the page has secrets, `mask` replaces them with `[REDACTED]` before the text reaches the LLM.

## Where to go next

- [All 28 tools](/docs/) - flags, exit codes, examples
- [Recipes](/recipes/) - multi-tool pipeline patterns
- [Agent integration](/agents/) - using vrk from AI agents
