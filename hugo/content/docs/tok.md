---
title: "vrk tok"
description: "vrk tok counts the tokens in text (o200k_base by default). Use --check N to stop a pipeline when the input is over your context budget."
meta_title: "vrk tok - Token Counter CLI for LLM Pipelines"
meta_lead: "vrk tok counts the tokens in text and can stop a pipeline when the text is over a budget."
og_title: "vrk tok - token counter and pipeline gate for LLM context budgets"
tool: tok
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk tok counts the tokens in text and can stop a pipeline when the text is over a budget.

## The problem

A 15,000-token document goes to a model with a 4,096-token window. No error. The response looks plausible. Three days later a QA reviewer finds inconsistencies. The model only saw the first third of the input. Silent truncation is the most expensive bug in LLM pipelines because it looks like success.

## The solution

`vrk tok` counts tokens and gates pipelines in about 25 ms. It counts with o200k_base by default, the tokenizer of GPT-4o, GPT-4.1, GPT-5, and the o-series. `--model gpt-4` or `gpt-3.5-turbo` switches to cl100k_base. The count is exact for OpenAI models and an estimate for Claude. Without `--check`, tok prints the count. With `--check N`, tok passes the input through if it is within the budget. If the input is over the budget, tok exits 1 with empty stdout, so the pipeline stops before it wastes an API call.

## Before and after

**Before**

```bash
# "Probably fine" - counting words as a proxy for tokens
wc -w system-prompt.txt
# 1,847 words... is that under 4,096 tokens? Who knows.
```

**After**

```bash
cat system-prompt.txt | vrk tok
```

## Example

```bash
cat system-prompt.txt | vrk tok --check 8000
```

## Exit codes

| Code | Meaning                                                                      |
|------|------------------------------------------------------------------------------|
| 0    | Measurement success; or --check within limit                                 |
| 1    | --check over limit; I/O error; tokenizer error                               |
| 2    | Usage error - unknown flag, unknown --model, no stdin, --check without value |

## Flags

| Flag      | Short | Type   | Description                                                                                                   |
|-----------|-------|--------|---------------------------------------------------------------------------------------------------------------|
| `--check` |       | int    | Pass input through if ≤N tokens; exit 1 with empty stdout if over                                             |
| `--model` | -m    | string | OpenAI model or encoding to count for: gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo, o200k_base (default), cl100k_base |
| `--json`  | -j    | bool   | Emit JSON (measurement) or JSON error (gate). Does not wrap passthrough.                                      |
| `--quiet` | -q    | bool   | Suppress stderr on failure                                                                                    |


<!-- notes - edit in notes/tok.notes.md -->

## How it works

`vrk tok` counts the tokens in text with the o200k_base tokenizer and can stop a pipeline when the text is over a budget.

### Measurement mode (default)

By default, tok counts the tokens and prints the number to stdout:

```bash
$ cat system-prompt.txt | vrk tok
11

$ cat system-prompt.txt | vrk tok --json
{"tokens":11,"model":"o200k_base"}
```

### Gate mode (--check N)

`--check N` makes tok a pipeline gate. If the input is N tokens or fewer, tok passes the full input to stdout unchanged, and you can pipe it to the next stage. If the input is over N tokens, stdout is empty and the exit code is 1, which stops the pipeline.

```bash
# Within budget - input passes through
$ echo 'short input' | vrk tok --check 4000
short input

# Over budget - empty stdout, exit 1
$ printf 'You are a helpful assistant.' | vrk tok --check 3
tok: 6 tokens exceeds limit of 3
$ echo $?
1
```

Put the gate before an LLM call, so the pipeline continues only if the input is within the budget:

```bash
cat document.txt | vrk tok --check 8000 | vrk prompt --system 'Summarize this'
```

### The --json flag

In measurement mode, `--json` puts the count in a JSON object:

```bash
$ echo 'Hello, world!' | vrk tok --json
{"tokens":4,"model":"o200k_base"}
```

If `--check` fails and `--json` is set, the error goes to stdout as JSON and stderr stays empty:

```bash
$ printf 'You are a helpful assistant.' | vrk tok --check 3 --json
{"code":1,"error":"6 tokens exceeds limit of 3","limit":3,"tokens":6}
```

### The --quiet flag

`--quiet` hides the stderr error message when `--check` fails. The exit code is still 1, so the pipeline still stops.

### Read the token count with jq

```bash
TOKENS=$(cat prompt.txt | vrk tok --json | jq -r '.tokens')
if [ "$TOKENS" -gt 8000 ]; then
  echo "Prompt too large: $TOKENS tokens" >&2
  exit 1
fi
```

## Pipeline integration

### Budget check in CI

Use `--check` to keep a system prompt within its budget on every deploy:

```bash
# ci/check-prompt-budget.sh
cat prompts/system.txt | vrk tok --check 6000
if [ $? -ne 0 ]; then
  echo "System prompt exceeds 6000-token budget. Refactor before merging." >&2
  exit 1
fi
```

### Measure, then chunk what is too large

```bash
# Process a directory of markdown files for summarization.
# Skip anything that fits in one call; chunk anything that doesn't.
for f in docs/*.md; do
  TOKENS=$(cat "$f" | vrk tok --json | jq -r '.tokens')
  if [ "$TOKENS" -le 8000 ]; then
    cat "$f" | vrk prompt --system 'Summarize this document'
  else
    cat "$f" | vrk chunk --size 4000 --overlap 200 | \
      while IFS= read -r chunk; do
        echo "$chunk" | jq -r '.text' | vrk prompt --system 'Summarize this section'
      done
  fi
done
```

### Gate before prompt with mask

```bash
# Redact secrets, check budget, then send to an LLM
cat debug-output.log | vrk mask | vrk tok --check 12000 | \
  vrk prompt --system 'What went wrong in this log output?'
```

## When it fails

If the input is over the budget, tok exits 1:

```bash
$ printf 'You are a helpful assistant.' | vrk tok --check 3
tok: 6 tokens exceeds limit of 3
$ echo $?
1
```

If the input is over the budget and you set `--json`, the error goes to stdout, stderr is empty, and tok exits 1:

```bash
$ printf 'You are a helpful assistant.' | vrk tok --check 3 --json
{"code":1,"error":"6 tokens exceeds limit of 3","limit":3,"tokens":6}
$ echo $?
1
```

If you pass an unknown flag, tok exits 2:

```bash
$ echo 'hi' | vrk tok --verbose
usage error: unknown flag: --verbose
$ echo $?
2
```
