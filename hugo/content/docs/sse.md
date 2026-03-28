---
title: "vrk sse"
description: "SSE stream parser - text/event-stream to JSONL."
tool: sse
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

LLM APIs stream responses as Server-Sent Events. Consuming an SSE
stream in a shell pipeline means dealing with `data:` prefixes,
blank-line event delimiters, multi-line data fields, and the `[DONE]`
sentinel. That is a parser, not a one-liner. There is no Unix tool for it.

## The fix

```bash
$ curl -sN https://api.example.com/stream | vrk sse
{"event":"message","data":{"id":"chatcmpl-...","choices":[{"delta":{"content":"Hello"}}]}}
{"event":"message","data":{"id":"chatcmpl-...","choices":[{"delta":{"content":" world"}}]}}
```

One JSONL record per SSE event. The `[DONE]` sentinel stops parsing
cleanly and exits 0 without emitting a record. The raw stream becomes
a structured stream you can pipe anywhere.

If stdin is an interactive terminal with no piped input, sse exits 2
immediately rather than hanging:

```
usage error: sse: no input: pipe an SSE stream to stdin
```

## Extracting a specific field

The `--field` flag navigates a dot-path through the parsed record and
prints the scalar value as plain text, one per line:

```bash
$ curl -sN https://api.example.com/stream | vrk sse --field data.delta.text
Hello
 world
```

If the path is absent in an event, that event is silently skipped. This
is intentional -- a mixed stream of event types will only have the target
field in some records.

Different providers nest the text delta at different paths. Anthropic
uses `data.delta.text`, OpenAI uses `data.choices[0].delta.content`.
Check your provider's SSE format and adjust the `--field` path accordingly.

## Full pipeline: stream and accumulate

```bash
curl -sN https://api.example.com/stream | vrk sse --field data.delta.text | tee response.txt
```

Pipe through `tee` to watch the stream live in the terminal and save
the accumulated text to a file at the same time. When the stream ends,
`response.txt` has the full response.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--event` | `-e` | string | `""` | Only emit events of this type; skip all others |
| `--field` | `-F` | string | `""` | Extract dot-path field from the record and print as plain text |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success, including clean `[DONE]` termination and truncated streams |
| 1 | I/O error reading stdin |
| 2 | Usage error - interactive terminal with no piped input, unknown flag |
