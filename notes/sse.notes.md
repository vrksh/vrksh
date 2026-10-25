## How it works

`vrk sse` parses a Server-Sent Events (SSE) stream from stdin into JSONL records, so you can read an LLM stream in a pipeline.

### Parse SSE into JSONL records

```bash
$ printf 'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}\n\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}\n\ndata: [DONE]\n\n' | vrk sse
{"event":"message","data":{"delta":{"text":"Hello","type":"text_delta"},"type":"content_block_delta"}}
{"event":"message","data":{"delta":{"text":" world","type":"text_delta"},"type":"content_block_delta"}}
```

Each SSE event becomes one JSONL record with `event` and `data` fields. The `[DONE]` sentinel ends the parse, and sse exits 0.

### Extract nested text with --field

`--field` takes a dot path into the JSON data and prints only that value.

**Anthropic streams** - text lives at `data.delta.text`:

```bash
$ printf 'data: {"type":"content_block_delta","delta":{"type":"text_delta","text":"Hello"}}\n\ndata: {"type":"content_block_delta","delta":{"type":"text_delta","text":" world"}}\n\ndata: [DONE]\n\n' | vrk sse --field data.delta.text --raw
Hello world
```

**OpenAI streams** - text lives at `data.choices[0].delta.content`:

```bash
curl -sN https://api.openai.com/v1/chat/completions \
  -H "Authorization: Bearer $OPENAI_API_KEY" \
  -H "content-type: application/json" \
  -d '{"model":"gpt-4","stream":true,...}' | \
  vrk sse --field 'data.choices[0].delta.content' --raw
```

An array index is `[0]` or `.0`, so `data.choices.0.delta.content` is the same path. If the stream has events but `--field` matches none of them, `vrk sse` prints `sse: --field <path> matched no events` to stderr and exits 1. If the stream has no events at all (for example only `[DONE]`), it exits 0.

### Reassemble streamed text (--raw)

By default `--field` prints one value per line. Streamed text has its own newlines, so this adds a break after every token. `--raw` writes each value as it arrived, with nothing after it. It flushes after every event, so the output stays live:

```bash
$ printf 'event: ping\ndata: {}\n\nevent: content_block_delta\ndata: {"delta":{"text":"Hello"}}\n\nevent: content_block_delta\ndata: {"delta":{"text":" world"}}\n\n' | vrk sse --event content_block_delta --field data.delta.text --raw
Hello world
```

### Filter by event type (--event)

If the stream has more than one event type and you want only one, use `--event`:

```bash
curl -sN $STREAM_URL | vrk sse --event content_block_delta
```

sse emits only the events of that type and drops all others.

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
RESPONSE=$(curl -sN $API_URL | vrk sse --field data.delta.text --raw)
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

If you run sse on an interactive terminal with no piped input, it exits 2:

<!-- doccheck: skip (needs an interactive terminal, doccheck runs with empty stdin) -->
```bash
$ vrk sse
usage error: sse: no input: pipe an SSE stream to stdin
$ echo $?
2
```

If the input is not SSE, sse prints nothing and exits 0, because there are no events to parse.

If the `data:` lines of one event pass 64 MiB before the closing blank line, sse exits 1 with `sse: event exceeds 64 MiB limit`. This stops a server that never ends an event from using all your memory.
