---
title: "vrk tok"
description: "vrk tok counts tokens from stdin and gates pipelines before they fail. Pass --check N to stop the pipeline if input exceeds your context budget. No Python runtime needed."
meta_title: "vrk tok - Token Counter CLI for LLM Pipelines"
meta_lead: "vrk tok is a command-line token counter for LLM pipelines."
og_title: "vrk tok - token counter and pipeline gate for LLM context budgets"
tool: tok
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk tok is a command-line token counter for LLM pipelines.

## The problem

A 15,000-token document goes to a model with a 4,096-token window. No error. The response looks plausible. Three days later a QA reviewer finds inconsistencies. The model only saw the first third of the input. Silent truncation is the most expensive bug in LLM pipelines because it looks like success.

## The solution

`vrk tok` counts tokens and gates pipelines in 5ms. Uses cl100k_base (exact for GPT-4, ~95% for Claude). Without `--check` it prints the count. With `--check N` it passes input through if within budget, or exits 1 with empty stdout if over, killing the pipeline before it wastes an API call.

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
cat system-prompt.txt | vrk tok --check 8000 | vrk prompt --system 'Summarize'
```

## Exit codes

| Code | Meaning                                                     |
|------|-------------------------------------------------------------|
| 0    | Measurement success; or --check within limit                |
| 1    | --check over limit; I/O error; tokenizer error              |
| 2    | Usage error - unknown flag, no stdin, --check without value |

## Flags

| Flag      | Short | Type   | Description                                                              |
|-----------|-------|--------|--------------------------------------------------------------------------|
| `--check` |       | int    | Pass input through if ≤N tokens; exit 1 with empty stdout if over        |
| `--model` |       | string | Tokenizer - cl100k_base (default)                                        |
| `--json`  | -j    | bool   | Emit JSON (measurement) or JSON error (gate). Does not wrap passthrough. |
| `--quiet` | -q    | bool   | Suppress stderr on failure                                               |


<!-- notes - edit in notes/tok.notes.md -->

## How it works

### Measurement mode (default)

Counts tokens and prints the number to stdout:

```bash
$ cat system-prompt.txt | vrk tok
8847

$ cat system-prompt.txt | vrk tok --json
{"tokens":8847,"model":"cl100k_base"}
```

### Gate mode (--check N)

`--check N` turns tok from a measurement tool into a pipeline gate. If the input fits within N tokens, the full input passes through to stdout unchanged - you can pipe it directly to the next stage. If it exceeds N tokens, stdout is empty and the exit code is 1, which stops any pipeline.

```bash
# Within budget - input passes through
$ echo 'short input' | vrk tok --check 4000
short input

# Over budget - empty stdout, exit 1
$ printf 'You are a helpful assistant.' | vrk tok --check 3
# (no stdout)
$ echo $?
1
```

Gate before an LLM call so the pipeline only continues if within budget:

```bash
cat document.txt | vrk tok --check 8000 | vrk prompt --system 'Summarize this'
```

### The --json flag

In measurement mode, `--json` wraps the count in a JSON object:

```bash
$ echo 'Hello, world!' | vrk tok --json
{"tokens":4,"model":"cl100k_base"}
```

When `--check` fails and `--json` is active, the error goes to stdout as JSON (stderr stays empty):

```bash
$ printf 'You are a helpful assistant.' | vrk tok --check 3 --json
{"code":1,"error":"6 tokens exceeds limit of 3","limit":3,"tokens":6}
```

### The --quiet flag

Suppresses the stderr error message on `--check` failure. The exit code is still 1, so pipelines still stop - you just don't get the human-readable message.

### Parsing token counts downstream with jq

```bash
TOKENS=$(cat prompt.txt | vrk tok --json | jq -r '.tokens')
if [ "$TOKENS" -gt 8000 ]; then
  echo "Prompt too large: $TOKENS tokens" >&2
  exit 1
fi
```

## Pipeline integration

### Budget check in CI

Enforce that a system prompt stays within budget across deploys:

```bash
# ci/check-prompt-budget.sh
cat prompts/system.txt | vrk tok --check 6000
if [ $? -ne 0 ]; then
  echo "System prompt exceeds 6000-token budget. Refactor before merging." >&2
  exit 1
fi
```

### Measure, then chunk what's too large

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

Over budget without `--json`:

```bash
$ printf 'You are a helpful assistant.' | vrk tok --check 3
tok: 6 tokens exceeds limit of 3
$ echo $?
1
```

Over budget with `--json` (error goes to stdout, stderr empty):

```bash
$ printf 'You are a helpful assistant.' | vrk tok --check 3 --json
{"code":1,"error":"6 tokens exceeds limit of 3","limit":3,"tokens":6}
$ echo $?
1
```

Unknown flag:

```bash
$ echo 'hi' | vrk tok --verbose
usage error: unknown flag: --verbose
$ echo $?
2
```
