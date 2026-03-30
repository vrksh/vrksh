# sse - SSE stream parser - text/event-stream to JSONL

When to use: parse a raw SSE stream (text/event-stream) into structured JSONL. Use --field with dot-path syntax to extract nested values like data.delta.text (Anthropic) or data.choices[0].delta.content (OpenAI).
Composes with: prompt, emit, kv, throttle

| Flag      | Short | Type   | Description                            |
|-----------|-------|--------|----------------------------------------|
| `--event` | `-e`  | string | Only emit events of this type          |
| `--field` | `-F`  | string | Extract a dot-path field as plain text |

Exit 0: success (stream parsed, [DONE] or EOF)
Exit 1: I/O error reading stdin
Exit 2: interactive terminal with no stdin, unknown flag

Example:

    curl -sN $API | vrk sse --event content_block_delta --field data.delta.text | tr -d '\n'

Anti-pattern:
- Don't use grep + sed to parse SSE streams. They break on multi-line data fields and lose event types. vrk sse handles all edge cases including [DONE] sentinels.
