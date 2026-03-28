# sse - SSE stream parser - text/event-stream to JSONL

When to use: parse a Server-Sent Events stream into structured JSONL for pipeline processing.
Composes with: prompt, emit, kv, throttle

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--event` | `-e` | string | Only emit events of this type |
| `--field` | `-F` | string | Extract a dot-path field as plain text |

Exit 0: success (stream parsed, [DONE] or EOF)
Exit 1: I/O error reading stdin
Exit 2: interactive terminal with no stdin, unknown flag

Example:

    curl -sN $API | vrk sse --event content_block_delta --field data.delta.text | tr -d '\n'

Anti-pattern:
- Don't store SSE output in a bash variable -- `$()` strips trailing `\n\n` which breaks SSE dispatch.
