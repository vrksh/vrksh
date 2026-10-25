---
title: "vrk prompt"
description: "vrk prompt sends text to Claude or GPT and prints the reply. It can check the reply against a --schema and retry on a mismatch."
meta_lead: "vrk prompt pipes text to Claude or GPT and prints the response to stdout."
og_title: "vrk prompt - pipe text to any LLM from your terminal"
tool: prompt
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk prompt pipes text to Claude or GPT and prints the response to stdout.

## The problem

Calling an LLM from a shell script means a curl command with JSON escaping, content-type headers, API key management, and response extraction. A backtick in the input breaks the JSON. A 429 response makes the jq pipeline print `null`. You spend more time on HTTP than on the task itself.

## The solution

`vrk prompt` pipes text to Claude or GPT and prints the response to stdout. Set `ANTHROPIC_API_KEY` or `OPENAI_API_KEY` and pipe in the content. You get plain text back, with no curl, JSON escaping, or response parsing. Temperature defaults to 0 for deterministic output. `--schema` checks the response has the keys and types you list, on Anthropic, OpenAI, and any `--endpoint`. Without `--fail` a mismatch still prints the response and exits 0. `--fail` makes it exit 1, and `--retry` re-sends with escalating temperature, then exits 1 if no attempt matches. An empty prompt is never sent: it exits 1.

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

| Code | Meaning                                                                                                            |
|------|--------------------------------------------------------------------------------------------------------------------|
| 0    | Success                                                                                                            |
| 1    | API failure, budget exceeded, empty prompt, schema mismatch with --fail or --retry, invalid JSONL, field not found |
| 2    | Usage error - no input, missing flags, --field with --explain                                                      |

## Flags

| Flag         | Short | Type   | Description                                                                |
|--------------|-------|--------|----------------------------------------------------------------------------|
| `--model`    | -m    | string | LLM model (default from VRK_DEFAULT_MODEL or claude-sonnet-4-6)            |
| `--system`   |       | string | System prompt text, or @file.txt to read from file                         |
| `--field`    |       | string | Dot-path field in each JSONL line to use as prompt text                    |
| `--budget`   |       | int    | Exit 1 if prompt exceeds N tokens                                          |
| `--fail`     | -f    | bool   | Exit 1 on schema mismatch (API errors always exit 1)                       |
| `--json`     | -j    | bool   | Emit response as JSON envelope with metadata                               |
| `--quiet`    | -q    | bool   | Suppress stderr output                                                     |
| `--schema`   | -s    | string | Flat key-to-type JSON schema (inline or file path) for response validation |
| `--explain`  |       | bool   | Print equivalent curl command, no API call                                 |
| `--retry`    |       | int    | Retry N times on schema mismatch (escalates temperature), then exit 1      |
| `--endpoint` |       | string | OpenAI-compatible API base URL                                             |


<!-- notes - edit in notes/prompt.notes.md -->

## Usage

`vrk prompt` sends text from stdin to an LLM (Anthropic, OpenAI or any OpenAI-compatible endpoint) and prints the reply to stdout.

### Simple question

<!-- doccheck: skip (real LLM output, the mock endpoint cannot answer questions) -->
```bash
$ echo 'What is the capital of France?' | vrk prompt
Paris.
```

### System prompt from a file

If the system prompt is longer than a line, store it in a file and reference the file with `@`:

```bash
cat user-feedback.csv | vrk prompt --system @prompts/analyze-feedback.txt
```

### Schema-validated structured output

Use `--schema` to check that the LLM returns JSON in a given shape:

<!-- doccheck: skip (real LLM output, the mock endpoint cannot fill a schema sent as system text) -->
```bash
$ cat bug-report.txt | vrk prompt \
    --system 'Extract structured fields from this bug report' \
    --schema '{"severity":"string","component":"string","summary":"string","reproducible":"boolean"}'
{"severity":"high","component":"auth","summary":"Login fails after password reset","reproducible":true}
```

If the response does not match the schema, the result depends on `--fail` and `--retry`:

- Neither flag set: prompt prints the response anyway, warns on stderr, and exits 0. `--schema` alone does not fail the pipeline.
- `--fail`: prompt exits 1 and prints nothing to stdout.
- `--retry N`: prompt re-sends up to N more times. If no attempt matches, it exits 1 with `response does not match schema after N+1 attempts`.

Add `--retry` to send the prompt again when the response does not match:

```bash
cat bug-report.txt | vrk prompt \
  --system 'Extract structured fields' \
  --schema '{"severity":"string","component":"string","summary":"string"}' \
  --retry 3
```

On each retry, the temperature rises by 0.1 (0, 0.1, 0.2, and so on, up to 1). This pushes the model to try a different answer.

The schema check works the same on every provider. Only the request differs:

- Anthropic: the schema keys and types are added to the system prompt as instructions.
- OpenAI: the schema is sent as `response_format` with a JSON Schema built from your keys (`type: object`, every key `required`, `additionalProperties: false`). `strict` is on only when every type is `string`, `number`, `integer`, or `boolean`.
- `--endpoint`: the schema instructions are sent as a system message (after your `--system` text, separated by a blank line), with `response_format: {"type":"json_object"}`.

In every case vrk checks the reply itself, so `--fail` and `--retry` work the same on every provider.

### See what would be sent without calling the API

<!-- doccheck: skip (output holds the provider endpoint, which differs per setup) -->
```bash
$ echo 'test input' | vrk prompt --explain
```

The `--explain` flag prints the equivalent curl command and exits without an API call. Use it to see the exact request.

## Flag details

### --model / -m

Selects the LLM. The default is `claude-sonnet-4-6` or the value of `VRK_DEFAULT_MODEL`.

```bash
# Use GPT-4o via OpenAI
cat article.md | vrk prompt -m gpt-4o --system 'Summarize this'

# Use a local model via Ollama
cat article.md | vrk prompt -m llama3 --endpoint http://localhost:11434/v1
```

### --budget

Checks the token count before the call. If the prompt is over N tokens, prompt exits 1 and does not call the API. This saves money on inputs that are too large.

```bash
$ cat huge-document.txt | vrk prompt --budget 10 --system 'Summarize'
error: prompt: 35 tokens exceeds budget of 10
$ echo $?
1
```

### --schema / -s

Checks the LLM response against a JSON schema. The value is an inline JSON string or a path to a `.json` file:

```bash
# Inline schema
echo 'Is this positive or negative: I love this product' | \
  vrk prompt --schema '{"sentiment":"string","confidence":"number"}'

# Schema from file
echo 'Extract entities' | vrk prompt --schema entities-schema.json
```

### --retry

Use `--retry` with `--schema`. If the response does not match the schema, prompt retries up to N times and raises the temperature on each attempt. If the last attempt still does not match, prompt exits 1, even without `--fail`:

```bash
cat input.txt | vrk prompt \
  --schema '{"answer":"string","confidence":"number"}' \
  --retry 3 --fail
```

### --fail / -f

If the response does not match the schema, prompt exits 1 and prints nothing. An API error (non-2xx) always exits 1, with or without `--fail`. Use `--fail` in CI or in pipelines where a wrong-shaped result is worse than no result.

### --json / -j

Puts the response in a JSON envelope with metadata (model used, token counts, timing). This flag does not tell the model to reply in JSON. Use `--schema` for that.

### --endpoint

Points prompt at any OpenAI-compatible API, such as Ollama, vLLM or LiteLLM. Any provider that speaks the OpenAI chat completions format works. `--schema`, `--fail`, and `--retry` work here too. `--model` is required.

Set `VRK_LLM_KEY` if the server needs a bearer token. The key goes only to the `VRK_LLM_URL` origin (scheme, host, port). If `--endpoint` points anywhere else, prompt exits 2 before it sends anything. Unset `VRK_LLM_KEY` for a server that needs no key. If you use plain `http` with a key, prompt exits 2 unless the host is loopback. Prompt does not follow redirects:

```bash
cat notes.txt | vrk prompt \
  --endpoint http://localhost:11434/v1 \
  --model llama3 \
  --system 'Summarize these meeting notes'
```

## Pipeline integration

### Fetch, measure and summarize

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

If no API key is set, prompt exits 1:

<!-- doccheck: skip (doccheck always sets a mock endpoint and key) -->
```bash
$ echo 'hello' | ANTHROPIC_API_KEY= vrk prompt
error: prompt: no API key found: set ANTHROPIC_API_KEY or OPENAI_API_KEY
$ echo $?
1
```

If the prompt is over the budget, prompt exits 1:

```bash
$ cat huge-file.txt | vrk prompt --budget 20 --system 'Summarize'
error: prompt: 35 tokens exceeds budget of 20
$ echo $?
1
```

If the response does not match the schema and you set `--fail` without `--retry`, prompt exits 1:

```bash
$ echo 'Tell me a joke' | vrk prompt --schema '{"setup":"string","punchline":"string"}' --fail
error: prompt: response does not match schema
$ echo $?
1
```

If the prompt is empty (nothing on stdin, or only whitespace), prompt sends nothing and exits 1:

```bash
$ printf '' | vrk prompt
error: prompt: empty prompt, nothing sent
$ echo $?
1
```

With `--field`, prompt skips a record whose field is empty and warns `prompt: line N: empty prompt, skipped`. It then continues with the next record.
