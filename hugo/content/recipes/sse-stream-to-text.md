---
title: "SSE stream to text"
meta_title: "SSE stream to text - vrk pipeline recipe"
description: "Raw SSE is unusable without parsing - sse extracts the content field so downstream tools get clean text, not protocol framing. Parse an SSE stream ..."
why: "Raw SSE is unusable without parsing - sse extracts the content field so downstream tools get clean text, not protocol framing."
body: "Parse an SSE stream and extract text tokens."
slug: "sse-stream-to-text"
steps:
  - |-
    curl -sN $API \
      | vrk sse --event content_block_delta \
          --field data.delta.text \
      | tr -d '\n'
tags:
  - "sse"
---

## The problem

Streaming LLM APIs (Anthropic, OpenAI) return responses as Server-Sent Events. The raw SSE stream looks like this:

```
event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}

event: content_block_delta
data: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}
```

You can't pipe this directly into another tool. You need to parse the SSE framing, filter by event type, and extract the text field from the JSON data.

## How the pipeline works

`vrk sse` reads the SSE stream from stdin, parses the `event:` and `data:` lines, and emits one JSONL record per event. `--event content_block_delta` filters to only that event type. `--field data.delta.text` extracts the nested text value and prints it directly.

The `tr -d '\n'` at the end joins the text fragments into a continuous string. Without it, each token would be on its own line.

## Variations

Capture the full stream as JSONL for debugging:

```bash
curl -sN $API | vrk sse --json > stream.jsonl
```

Filter for a specific event and count occurrences:

```bash
curl -sN $API | vrk sse --event content_block_delta --json | wc -l
```

