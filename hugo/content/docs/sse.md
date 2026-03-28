---
title: "vrk sse"
description: "SSE stream parser - text/event-stream to JSONL."
tool: sse
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

LLM APIs stream responses as Server-Sent Events. The raw format is human-readable but machine-unfriendly: `data: {"choices":[{"delta":{"content":"Hello"}}]}` with blank-line event delimiters. Extracting the content deltas from that stream without writing a parser - one that handles multi-line data fields, the `[DONE]` sentinel, and named events - is the kind of glue code that belongs in a tool, not in every script.

## The fix

```bash
curl -sN https://api.example.com/stream | vrk sse
```

Each SSE event becomes one JSONL record:

```
{"event":"message","data":{"id":"chatcmpl-...","choices":[{"delta":{"content":"Hello"}}]}}
{"event":"message","data":{"id":"chatcmpl-...","choices":[{"delta":{"content":" world"}}]}}
```

The `[DONE]` sentinel stops parsing and exits 0 without emitting a record.

## Walkthrough

**Happy path** - pipe any SSE stream and get JSONL out:

```bash
curl -sN --header "Authorization: Bearer $ANTHROPIC_API_KEY" \
  --header "Content-Type: application/json" \
  --data '{"model":"claude-sonnet-4-6","max_tokens":100,"stream":true,"messages":[{"role":"user","content":"Hi"}]}' \
  https://api.anthropic.com/v1/messages \
  | vrk sse
```

<!-- output: verify against binary -->

Events without an explicit `event:` field are named `"message"` - the SSE spec default.

**Failure case** - if the stream is cut before a complete event (no trailing blank line), the in-progress block is silently dropped per the SSE spec. The exit code is still 0. This is correct behavior: truncated streams are common in proxied environments.

If stdin is an interactive terminal with no piped input, sse exits 2 immediately rather than hanging:

```bash
vrk sse
# usage error: sse: no input: pipe an SSE stream to stdin
```

**Filtering by event type** - Anthropic's API sends several event types (`message_start`, `content_block_delta`, `message_stop`). Pass `--event` to see only the ones you care about:

```bash
curl -sN ... | vrk sse --event content_block_delta
```

**Extracting a nested field** - `--field` navigates a dot-path through `{event, data}` and prints the scalar value as plain text. This is useful when you want only the content deltas:

```bash
curl -sN ... | vrk sse --field data.delta.text
```

If the path is absent in an event, that event is silently skipped. This is intentional - a mixed stream of event types will only have the target field in some records.

## Pipeline example

Stream a response, extract content deltas, and count the tokens used:

```bash
curl -sN ... | vrk sse --event content_block_delta --field data.delta.text | vrk tok
```

Stream, extract the full data JSON for each delta, and validate it against a schema:

```bash
curl -sN ... | vrk sse --event content_block_delta | vrk validate --schema delta.schema.json
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--event` | `-e` | string | `""` | Only emit events of this type; skip all others |
| `--field` | `-F` | string | `""` | Extract dot-path field from `{event, data}` and print as plain text. Missing paths are silently skipped. |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success, including clean `[DONE]` termination and truncated streams |
| 1 | I/O error reading stdin |
| 2 | Usage error - interactive terminal with no piped input, unknown flag |
