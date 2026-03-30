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
