---
title: "vrk sse"
description: "Parse Server-Sent Event streams into JSONL. Turns text/event-stream into structured records you can pipe."
og_title: "vrk sse - parse SSE streams into structured JSONL"
tool: sse
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## The problem

A streaming LLM endpoint returns raw Server-Sent Events: `data:` prefixes, blank lines as delimiters, `[DONE]` sentinels. `grep '^data:' | sed 's/^data: //'` works until a data field spans multiple lines. The sed breaks and the streamed response has gaps.

## The solution

`vrk sse` parses `text/event-stream` into clean JSONL records. It handles `data:` prefixes, blank-line event delimiters, multi-line data concatenation, and `[DONE]` sentinel detection used by both Anthropic and OpenAI. `--field` with dot-path syntax drills into nested JSON to extract just the text fragments.

## Before and after

**Before**

```bash
curl -sN https://api.example.com/stream | \
  grep '^data: ' | sed 's/^data: //' | grep -v '^\[DONE\]$'
# breaks on multi-line data fields
# loses event types and IDs
```

**After**

```bash
curl -sN https://api.example.com/stream | vrk sse
```

## Example

```bash
curl -sN $API_URL | vrk sse --field data.delta.text
```

## Exit codes

| Code | Meaning                                                              |
|------|----------------------------------------------------------------------|
| 0    | Success, including clean [DONE] termination                          |
| 1    | I/O error reading stdin                                              |
| 2    | Usage error - interactive terminal with no piped input, unknown flag |

## Flags

| Flag      | Short | Type   | Description                                      |
|-----------|-------|--------|--------------------------------------------------|
| `--event` | -e    | string | Only emit events of this type                    |
| `--field` | -F    | string | Extract dot-path field from record as plain text |


<!-- notes - edit in notes/sse.notes.md -->

## How it works

### Parse SSE into JSONL records

```bash
$ printf 'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}\n\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}\n\ndata: [DONE]\n\n' | vrk sse
{"event":"message","data":{"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}}
{"event":"message","data":{"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}}
```

Each SSE event becomes one JSONL record with `event` and `data` fields. The `[DONE]` sentinel terminates parsing cleanly (exit 0).

### Extract nested text with --field

The real power is `--field`, which uses dot-path syntax to drill into the JSON data and print just the value you need.

**Anthropic streams** - text lives at `data.delta.text`:

```bash
$ printf 'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}\n\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}\n\ndata: [DONE]\n\n' | vrk sse --field data.delta.text
Hello
 world
```

**OpenAI streams** - text lives at `data.choices[0].delta.content`:

```bash
curl -sN https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "content-type: application/json" \
  -d '{"model":"gpt-4","stream":true,...}' | \
  vrk sse --field data.choices[0].delta.content
```

### Filter by event type (--event)

If the stream contains multiple event types and you only want one:

```bash
curl -sN $STREAM_URL | vrk sse --event content_block_delta
```

Only events matching that type are emitted. All others are silently dropped.

## Pipeline integration

### Stream an LLM response and log it

```bash
# Stream a response, extract text tokens, log each line as structured JSONL
curl -sN $API_URL | \
  vrk sse --field data.delta.text | \
  vrk emit --tag llm-stream --level info
```

### Stream, collect, then validate

```bash
# Stream a response, concatenate all text, validate as JSON
RESPONSE=$(curl -sN $API_URL | vrk sse --field data.delta.text | tr -d '\n')
echo "$RESPONSE" | vrk validate --schema '{"answer":"string","confidence":"number"}' --strict
```

### Rate-limited stream processing

```bash
# Parse SSE events but throttle downstream processing to 10/s
curl -sN $API_URL | \
  vrk sse | \
  vrk throttle --rate 10/s | \
  while IFS= read -r record; do
    echo "$record" | jq -r '.data' | process-event
  done
```

## When it fails

Interactive terminal with no piped input:

```bash
$ vrk sse
usage error: sse: no input: pipe an SSE stream to stdin
$ echo $?
2
```

Non-SSE input produces no output and exits 0 - there are simply no events to parse.
