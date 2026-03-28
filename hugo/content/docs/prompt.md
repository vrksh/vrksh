---
title: "vrk prompt"
description: "LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain."
tool: prompt
group: core
mcp_callable: true
noindex: false
---

## The problem

You need to call an LLM from a shell script. You could write a curl command
with the right headers, body format, and error handling - or you could write
a Python script that imports the SDK. Either way, it's 20+ lines for what
should be a one-liner. And if you need structured JSON output, schema
validation, or automatic retries, the complexity multiplies.

## The fix

```bash
echo "Summarize this in one sentence" | vrk prompt
```

<!-- output: verify against binary -->

That sends the input to the default model and prints the response to stdout.
The model is controlled by `VRK_DEFAULT_MODEL` or defaults to `claude-sonnet-4-6`.

## Walkthrough

### Structured output with --schema

When you need JSON back from the LLM, not free-form text:

```bash
echo "Extract the name and age" | vrk prompt --schema '{"name":"string","age":"number"}'
```

<!-- output: verify against binary -->

The `--schema` flag instructs the LLM to respond in JSON matching the given
schema. If the response doesn't validate, the call fails - unless you add
`--retry`.

### Automatic retries on schema mismatch

```bash
echo "Extract fields" | vrk prompt --schema '{"title":"string"}' --retry 3
```

Each retry escalates the temperature slightly, giving the model a better chance
of producing valid output on the next attempt.

### What failure looks like

When the API key is missing or the API returns an error:

```bash
echo "Hello" | vrk prompt
echo $?
# 1
```

<!-- output: verify against binary -->

Exit 1. Stderr gets the error message (or stdout gets JSON if `--json` is active).
The pipeline stops.

### Explain mode

See what would be sent without making the API call:

```bash
echo "Hello" | vrk prompt --explain
```

<!-- output: verify against binary -->

This prints the equivalent curl command. Useful for debugging or auditing
what your pipeline is actually sending.

### JSON envelope

```bash
echo "Hello" | vrk prompt --json
```

<!-- output: verify against binary -->

The `--json` flag wraps the response in a JSON object with metadata (model,
tokens used, latency). It does **not** instruct the LLM to respond in JSON  - 
that's what `--schema` is for.

## Pipeline example

Fetch a page, check it fits the context window, then summarize:

```bash
vrk grab https://example.com/article | vrk tok --budget 8000 && \
vrk grab https://example.com/article | vrk prompt --system "Summarize this article"
```

Extract structured data from a web page:

```bash
vrk grab --text https://example.com/product | \
  vrk prompt --schema '{"name":"string","price":"string","description":"string"}'
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--model` | `-m` | string | `VRK_DEFAULT_MODEL` or `claude-sonnet-4-6` | LLM model |
| `--system` | | string | `""` | System prompt text, or `@file.txt` to read from file |
| `--budget` | | int | `0` | Exit 1 if prompt exceeds N tokens |
| `--fail` | `-f` | bool | `false` | Fail on non-2xx API response or schema mismatch |
| `--json` | `-j` | bool | `false` | Emit response as JSON envelope with metadata |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |
| `--schema` | `-s` | string | `""` | JSON schema for response validation |
| `--explain` | | bool | `false` | Print equivalent curl command, no API call |
| `--retry` | | int | `0` | Retry N times on schema mismatch (escalates temperature) |
| `--endpoint` | | string | `""` | OpenAI-compatible API base URL |

## Two-input model

`vrk prompt` separates **content** from **instruction**:

- **stdin** (or positional argument) is the content — the data to process.
- **`--system`** is the instruction — what to do with the content.

```bash
echo "$CONTENT" | vrk prompt --system "Summarize in 3 bullets."
```

When `--system` is omitted, stdin is sent as the user message with no system prompt.
Use `--system @file.txt` to read a long system prompt from a file.

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | API failure, budget exceeded, or schema mismatch |
| 2 | Usage error - no input, missing flags |
