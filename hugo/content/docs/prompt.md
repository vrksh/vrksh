---
title: "vrk prompt"
description: "LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain."
tool: prompt
group: core
mcp_callable: true
noindex: false
---

## The problem

You need to call an LLM from a shell script or pipeline. Every other option
requires knowing the API shape, handling streaming, and parsing the response
manually. curl to the Anthropic API is 8 lines of headers and JSON. A Python
one-liner pulls in an SDK, a virtualenv, and a dependency you have to maintain.
There is no simple stdin-to-stdout tool that behaves like a Unix filter.

## The simplest call

```bash
$ echo "What is the capital of France?" | vrk prompt
Paris.
```

stdin is the user message. stdout is the response. No flags, no config file,
no SDK. The default model is `claude-sonnet-4-6` via the Anthropic API. Override
it with `--model` or set `VRK_DEFAULT_MODEL` in your environment.

## Adding a system instruction

`vrk prompt` separates **content** from **instruction**:

- **stdin** (or positional argument) is the content -- the data to process.
- **`--system`** is the instruction -- what to do with the content.

```bash
$ echo "$CONTENT" | vrk prompt --system "Summarize in 3 bullets."
```

When `--system` is omitted, stdin is sent as the user message with no system
prompt. Use `--system @file.txt` to read a long system prompt from a file
instead of inlining it.

## Schema-validated output

When you need structured JSON back from the model, not free-form text:

```bash
$ echo "$CONTENT" | vrk prompt --system "Extract the date." \
    --schema '{"date":"string","confidence":"number"}'
```

The `--schema` flag instructs the LLM to respond in JSON matching the given
schema. If the response does not validate, prompt exits 1.

What failure looks like on stderr:

```
error: prompt: response does not match schema
```

Add `--retry` for automatic retries on schema mismatch. Each retry escalates
the temperature slightly, giving the model a better chance of producing valid
output:

```bash
$ echo "$CONTENT" | vrk prompt --schema '{"title":"string"}' --retry 3
```

## The --explain flag

See what would be sent without making the API call:

```bash
$ echo "Hello world" | vrk prompt --explain
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-sonnet-4-6","max_tokens":4096,"messages":[{"role":"user","content":"Hello world"}]}'
```

With a system prompt:

```bash
$ echo "Hello world" | vrk prompt --explain --system "Summarize this."
curl https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-sonnet-4-6","max_tokens":4096,"system":"Summarize this.","messages":[{"role":"user","content":"Hello world"}]}'
```

No API call is made. Exit code 0. Useful for debugging what your pipeline is
actually sending, or for piping the curl command into a script that runs it
later.

## JSON envelope

```bash
$ echo "Hello" | vrk prompt --json
```

The `--json` flag wraps the response in a JSON object with metadata (model,
tokens used). It does **not** instruct the LLM to respond in JSON -- that is
what `--schema` is for. Use `--json` when a script needs to parse the response
or log metadata alongside it.

## Real pipeline with tok guard

```bash
CONTENT=$(vrk grab https://example.com/article)
echo "$CONTENT" | vrk tok --budget 8000 \
  && echo "$CONTENT" | vrk prompt --system "Summarize in 3 bullets."
```

The `&&` means: only run the right side if the left side exits 0. If tok
exits 1 (over budget), prompt never runs. No API call, no wasted money.

We capture the content in a variable first to avoid fetching the URL twice.
Fetching twice is wasteful and can return different content if the page changes
between requests.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--system` | | string | `""` | System prompt text, or `@file.txt` to read from file |
| `--model` | `-m` | string | `claude-sonnet-4-6` | LLM model (or `VRK_DEFAULT_MODEL` env var) |
| `--schema` | `-s` | string | `""` | JSON schema for response validation |
| `--retry` | | int | `0` | Retry N times on schema mismatch (escalates temperature) |
| `--budget` | | int | `0` | Exit 1 if prompt exceeds N tokens |
| `--explain` | | bool | `false` | Print equivalent curl command, no API call |
| `--json` | `-j` | bool | `false` | Emit response as JSON envelope with metadata |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |
| `--fail` | `-f` | bool | `false` | Fail on non-2xx API response or schema mismatch |
| `--endpoint` | | string | `""` | OpenAI-compatible API base URL |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | API failure, budget exceeded, or schema mismatch after retries |
| 2 | No input, unknown flag |
