# prompt - LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain

When to use: send a prompt to an LLM and get the response as text or structured JSON.
Composes with: tok, grab, chunk, validate, mask, kv, coax

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--model` | `-m` | string | Model name (default: claude-sonnet-4-6 or VRK_DEFAULT_MODEL) |
| `--system` | | string | System prompt text, or `@file.txt` to read from file |
| `--budget` | | int | Exit 1 if prompt exceeds N tokens before calling API |
| `--fail` | `-f` | bool | Exit 1 on schema mismatch |
| `--json` | `-j` | bool | JSON envelope: `{response, model, tokens_used, latency_ms, request_hash}` |
| `--schema` | `-s` | string | JSON schema for response validation |
| `--explain` | | bool | Print equivalent curl command, no API call |
| `--retry` | | int | Retry N times on schema mismatch (escalates temperature) |
| `--endpoint` | | string | OpenAI-compatible API base URL |

Exit 0: success
Exit 1: no API key, HTTP error, budget exceeded, schema mismatch
Exit 2: no input in interactive terminal, unknown flag

Example:

    cat doc.txt | vrk tok --budget 4000 && cat doc.txt | vrk prompt --system "summarise"

Anti-pattern:
- Don't confuse --json (metadata envelope) with --schema (structured LLM output). --json wraps the response; --schema tells the LLM to respond in JSON.
