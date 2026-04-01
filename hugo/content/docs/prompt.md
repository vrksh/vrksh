---
title: "vrk prompt"
description: "Pipe text to Claude or GPT from your terminal. Schema validation, retries, deterministic output."
og_title: "vrk prompt - pipe text to any LLM from your terminal"
tool: prompt
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

You want to call an LLM from a shell script. So you write a curl command: escape the JSON, set the headers, parse the response, handle errors. The prompt has a newline in it and your JSON breaks. The API returns a 429 and your jq pipeline prints `null`. You've spent more time on HTTP plumbing than on the actual task.

`vrk prompt` pipes text to Claude or GPT from your terminal and prints the response to stdout. Set `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`, pipe in content, and get back plain text. No curl. No JSON escaping. No response parsing. Temperature defaults to 0 for deterministic, reproducible output. Add `--schema` and the response is validated against a JSON schema. Add `--retry` and failed validations are retried with escalating temperature.

## The problem

You need to summarize 200 documents nightly. Each one needs a curl command with proper JSON escaping, content-type headers, API key management, response extraction, and error handling. One document has a backtick in it. Your JSON breaks. The pipeline keeps running. You get 47 empty summaries before anyone notices.

## Before and after

**Before**

```bash
curl -s https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "content-type: application/json" \
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[{"role":"user","content":"Summarize this"}]}'
```

**After**

```bash
cat article.md | vrk prompt --system 'Summarize this' --model claude-sonnet-4-6
```

## Example

```bash
cat article.md | vrk prompt --system 'Summarize the key findings in 3 bullet points'
```

## Exit codes

| Code | Meaning                                          |
|------|--------------------------------------------------|
| 0    | Success                                          |
| 1    | API failure, budget exceeded, or schema mismatch |
| 2    | Usage error - no input, missing flags            |

## Flags

| Flag         | Short | Type   | Description                                                     |
|--------------|-------|--------|-----------------------------------------------------------------|
| `--model`    | -m    | string | LLM model (default from VRK_DEFAULT_MODEL or claude-sonnet-4-6) |
| `--system`   |       | string | System prompt text, or @file.txt to read from file              |
| `--budget`   |       | int    | Exit 1 if prompt exceeds N tokens                               |
| `--fail`     | -f    | bool   | Fail on non-2xx API response or schema mismatch                 |
| `--json`     | -j    | bool   | Emit response as JSON envelope with metadata                    |
| `--quiet`    | -q    | bool   | Suppress stderr output                                          |
| `--schema`   | -s    | string | JSON schema for response validation                             |
| `--explain`  |       | bool   | Print equivalent curl command, no API call                      |
| `--retry`    |       | int    | Retry N times on schema mismatch (escalates temperature)        |
| `--endpoint` |       | string | OpenAI-compatible API base URL                                  |


<!-- notes - edit in notes/prompt.notes.md -->

## Usage

### Simple question

```bash
$ echo 'What is the capital of France?' | vrk prompt
Paris.
```

### System prompt from a file

For prompts longer than a line, store them in a file and reference with `@`:

```bash
cat user-feedback.csv | vrk prompt --system @prompts/analyze-feedback.txt
```

### Schema-validated structured output

Force the LLM to return JSON matching a specific shape:

```bash
$ cat bug-report.txt | vrk prompt \
    --system 'Extract structured fields from this bug report' \
    --schema '{"severity":"string","component":"string","summary":"string","reproducible":"boolean"}'
{"severity":"high","component":"auth","summary":"Login fails after password reset","reproducible":true}
```

If the LLM returns JSON that doesn't match the schema, prompt exits 1. Combine with `--retry` to automatically re-prompt:

```bash
cat bug-report.txt | vrk prompt \
  --system 'Extract structured fields' \
  --schema '{"severity":"string","component":"string","summary":"string"}' \
  --retry 3
```

On each retry, temperature escalates slightly to encourage the model to try a different approach.

### See what would be sent without calling the API

```bash
$ echo 'test input' | vrk prompt --explain
```

The `--explain` flag prints the equivalent curl command and exits without making an API call. Use it to debug what prompt is actually being sent.

## Flag details

### --model / -m

Selects the LLM. Defaults to `claude-sonnet-4-6` or the value of `VRK_DEFAULT_MODEL`.

```bash
# Use GPT-4o via OpenAI
cat article.md | vrk prompt -m gpt-4o --system 'Summarize this'

# Use a local model via Ollama
cat article.md | vrk prompt -m llama3 --endpoint http://localhost:11434/v1
```

### --budget

Pre-flight token check. If the prompt exceeds N tokens, prompt exits 1 without calling the API. Saves money on obviously-too-large inputs.

```bash
$ cat huge-document.txt | vrk prompt --budget 4000 --system 'Summarize'
error: prompt: 23847 tokens exceeds budget 4000
$ echo $?
1
```

### --schema / -s

Validates the LLM response against a JSON schema. Can be an inline JSON string or a path to a `.json` file:

```bash
# Inline schema
echo 'Is this positive or negative: I love this product' | \
  vrk prompt --schema '{"sentiment":"string","confidence":"number"}'

# Schema from file
echo 'Extract entities' | vrk prompt --schema entities-schema.json
```

### --retry

Only meaningful with `--schema`. Retries N times when the response doesn't match the schema, escalating temperature on each attempt:

```bash
cat input.txt | vrk prompt \
  --schema '{"answer":"string","confidence":"number"}' \
  --retry 3 --fail
```

### --fail / -f

Exit 1 on non-2xx API response or schema mismatch instead of printing partial output. Use in CI or pipelines where partial results are worse than no results.

### --json / -j

Wraps the response in a JSON envelope with metadata (model used, token counts, timing). This is for wrapping the LLM's response - it does NOT instruct the model to respond in JSON. Use `--schema` for that.

### --endpoint

Point prompt at any OpenAI-compatible API. Works with Ollama, vLLM, LiteLLM, or any provider that speaks the OpenAI chat completions format:

```bash
cat notes.txt | vrk prompt \
  --endpoint http://localhost:11434/v1 \
  --model llama3 \
  --system 'Summarize these meeting notes'
```

## Pipeline integration

### Fetch, measure, and summarize

```bash
# Grab a web page, check it fits in context, summarize it
vrk grab https://blog.example.com/post | \
  vrk tok --check 12000 | \
  vrk prompt --system 'Summarize the key points in 3 bullets'
```

### Redact secrets before sending to an LLM

```bash
# Mask credentials from log output, then analyze
cat deploy.log | vrk mask | \
  vrk prompt --system 'What errors occurred in this deployment?'
```

### Structured extraction with validation

```bash
# Extract entities from each chunk, validate the schema, log results
cat long-document.md | vrk chunk --size 4000 | \
  vrk prompt --field text \
    --schema '{"entities":"array","summary":"string"}' \
    --retry 2 \
    --system 'Extract named entities and a one-line summary' \
    --json | \
  vrk validate --schema '{"entities":"array","summary":"string"}' --strict
```

### Nightly batch with retry and state tracking

```bash
# Process new articles, track progress in kv
for url in $(cat urls.txt); do
  CONTENT=$(vrk grab "$url" | vrk mask)
  SUMMARY=$(echo "$CONTENT" | vrk prompt --system @prompts/summarize.txt --retry 2)
  if [ $? -eq 0 ]; then
    vrk kv set --ns summaries "$(echo "$url" | vrk slug)" "$SUMMARY" --ttl 168h
    vrk kv incr --ns summaries processed
  fi
done
```

## When it fails

API key missing:

```bash
$ echo 'hello' | ANTHROPIC_API_KEY= vrk prompt
error: prompt: ANTHROPIC_API_KEY or OPENAI_API_KEY must be set
$ echo $?
1
```

Budget exceeded:

```bash
$ cat huge-file.txt | vrk prompt --budget 100 --system 'Summarize'
error: prompt: 23847 tokens exceeds budget 100
$ echo $?
1
```

Schema mismatch without --retry:

```bash
$ echo 'Tell me a joke' | vrk prompt --schema '{"setup":"string","punchline":"string"}' --fail
error: prompt: response does not match schema
$ echo $?
1
```
