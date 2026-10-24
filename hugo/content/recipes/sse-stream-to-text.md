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
          --field data.delta.text --raw
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

`vrk sse` reads the SSE stream from stdin and parses the `event:` and `data:` lines. `--event content_block_delta` keeps only that event type. `--field data.delta.text` pulls the nested text value out of each event.

By default `sse` prints one value per line. `--raw` writes each value exactly as it is, with no separator added, so the fragments join into the model's real text, newlines and all. No `tr` needed.

## OpenAI streams

OpenAI puts the text in an array, so use an index in the path:

```bash
curl -sN $API | vrk sse --field 'data.choices[0].delta.content' --raw
```

If events arrive but the field matches none of them, `vrk sse` exits 1 and says so on stderr. A wrong path no longer looks like an empty answer.

## Variations

Capture the events as JSONL for debugging:

```bash
curl -sN $API | vrk sse > stream.jsonl
```

Filter for a specific event and count occurrences:

```bash
curl -sN $API | vrk sse --event content_block_delta | wc -l
```

