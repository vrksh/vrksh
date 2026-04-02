---
title: "Quickstart"
meta_title: "Quickstart - zero to working pipeline in five minutes"
description: "Install vrksh, run your first tool, build your first pipeline. Five minutes, no boilerplate."
noindex: false
---

## Install

```bash
brew tap vrksh/vrksh && brew install vrk
```

Other methods: [install page](/install/).

## Check it works

No API key needed. This runs locally in milliseconds.

```bash
vrk uuid
# 745be599-18d1-47ad-9821-bcdb34b784a8
```

```bash
echo 'Hello world' | vrk tok
# 2
```

## Set your API key

```bash
export ANTHROPIC_API_KEY=sk-ant-...
```

Or use OpenAI instead:

```bash
export OPENAI_API_KEY=sk-...
```

## First LLM call

```bash
echo 'Explain exit codes in one sentence.' | vrk prompt
```

The response prints to stdout. Errors go to stderr. Exit 0 on success, 1 on failure.

## First pipeline

Fetch a web page, strip the markup, count the tokens, and redact any secrets before sending to an LLM:

```bash
vrk grab https://example.com \
  | vrk plain \
  | vrk mask \
  | vrk tok --check 8000 \
  | vrk prompt --system 'Summarize this page in three bullet points.'
```

Five tools, one line. If the page exceeds 8,000 tokens, `tok` exits 1 and the pipeline stops before the API call. If the page contains secrets, `mask` replaces them with `[REDACTED]` before anything reaches the LLM.

## Where to go next

- [All 26 tools](/docs/) - flags, exit codes, examples
- [Recipes](/recipes/) - multi-tool pipeline patterns
- [Agent integration](/agents/) - using vrk from AI agents
