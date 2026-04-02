---
title: "vrk coax"
description: "Retry flaky commands with exponential backoff. Wrap any pipeline stage to handle transient API failures."
meta_lead: "vrk coax retries a command with configurable backoff, buffering stdin so each attempt gets the same input."
og_title: "vrk coax - retry wrapper with backoff for flaky commands"
tool: coax
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

vrk coax retries a command with configurable backoff, buffering stdin so each attempt gets the same input.

## The problem

A bash retry loop around a flaky API call works until the API starts returning 500s. The loop hammers it with no backoff, making the outage worse. Stdin was consumed on the first attempt, so every retry sends empty input. The hardcoded `sleep 2` is either too aggressive or too conservative.

## The solution

`vrk coax` retries a command with configurable backoff. It buffers stdin so each attempt gets the same input. Fixed or exponential backoff, retry count caps, and exit-code filtering are all built in. It solves the three things bash retry loops always get wrong: stdin replay, backoff strategy, and retry limits.

## Before and after

**Before**

```bash
for i in 1 2 3 4 5; do
  curl -sf https://api.example.com && break
  sleep $((i * 2))
done
```

**After**

```bash
vrk coax --times 5 --backoff exp:200ms -- curl -sf https://api.example.com
```

## Example

```bash
echo 'Summarize this' | vrk coax --times 5 --backoff exp:200ms -- vrk prompt
```

## Exit codes

| Code | Meaning                                                     |
|------|-------------------------------------------------------------|
| 0    | Command succeeded (first attempt or a retry)                |
| 1    | All retries exhausted (last exit code from wrapped command) |
| 2    | Missing command after --, invalid --backoff spec            |

## Flags

| Flag            | Short | Type     | Description                                                       |
|-----------------|-------|----------|-------------------------------------------------------------------|
| `--times`       |       | int      | Number of retries (total attempts = N+1)                          |
| `--backoff`     |       | string   | Delay between retries: 500ms for fixed, exp:100ms for exponential |
| `--backoff-max` |       | duration | Cap for exponential backoff; 0 means uncapped                     |
| `--on`          |       | []int    | Retry only on these exit codes; repeatable                        |
| `--until`       |       | string   | Shell condition; retry while this exits non-zero                  |
| `--quiet`       | -q    | bool     | Suppress retry progress lines                                     |
| `--json`        | -j    | bool     | Emit errors as JSON                                               |


<!-- notes - edit in notes/coax.notes.md -->

## Backoff strategies

### Fixed delay

Retry up to 3 times with a 500ms pause between attempts:

```bash
vrk coax --times 3 --backoff 500ms -- curl -sf https://flaky-api.example.com/data
```

Every retry waits the same amount. Up to 4 total attempts (1 initial + 3 retries).

### Exponential backoff

Start at 200ms, doubling each time: 200ms, 400ms, 800ms, 1600ms...

```bash
vrk coax --times 5 --backoff exp:200ms -- curl -sf https://api.example.com/query
```

This is the right strategy for rate-limited APIs. Early retries are fast. Later retries back off to let the rate limit window reset.

### Capped exponential

Exponential without a cap can grow to absurd delays. Cap it with `--backoff-max`:

```bash
vrk coax --times 10 --backoff exp:100ms --backoff-max 30s -- curl -sf https://api.example.com
```

Delays: 100ms, 200ms, 400ms, 800ms, 1.6s, 3.2s, 6.4s, 12.8s, 25.6s, 30s. The cap prevents the last retries from waiting minutes.

## Flag details

### --on (retry only on specific exit codes)

Only retry when the command exits 1 (HTTP error in curl -f), not on exit 2 (usage error):

```bash
vrk coax --times 3 --backoff exp:200ms --on 1 -- curl -sf https://api.example.com
```

The `--on` flag is repeatable: `--on 1 --on 137` retries on exit 1 or 137 (killed by signal). Without `--on`, any non-zero exit code triggers a retry.

### --until (retry until a condition is met)

Retry until a condition is true rather than until the command succeeds:

```bash
vrk coax --until 'vrk kv get --ns deploy status | grep -q complete' \
  --times 30 --backoff 10s -- echo 'checking...'
```

This runs the command, then checks the `--until` condition. If the condition exits 0, coax stops.

### What retry progress looks like

Without `--quiet`, coax prints progress to stderr:

```
coax: attempt 1/4 failed (exit 1), waiting 200ms
coax: attempt 2/4 failed (exit 1), waiting 400ms
coax: attempt 3/4 failed (exit 1), waiting 800ms
coax: attempt 4/4 failed (exit 1), all retries exhausted
```

With `--quiet`, stderr from coax is silent (subprocess stderr still passes through). The exit code still reflects success or failure.

## Pipeline integration

### Retry an LLM call that might hit rate limits

```bash
# Wrap an LLM call with retry + exponential backoff
cat document.txt | \
  vrk coax --times 5 --backoff exp:500ms --backoff-max 30s -- \
    vrk prompt --system 'Summarize this document'
```

stdin is buffered, so each retry sends the same document content.

### Retry within a chunk-processing loop

```bash
# Process each chunk with retries, track successes
cat large-document.md | vrk chunk --size 4000 --overlap 200 | \
  while IFS= read -r record; do
    echo "$record" | jq -r '.text' | \
      vrk coax --times 3 --backoff exp:200ms -- \
        vrk prompt --schema '{"summary":"string"}' --retry 2
    if [ $? -eq 0 ]; then
      vrk kv incr --ns batch-run completed
    fi
  done
```

### Retry a flaky web fetch

```bash
# Fetch a flaky URL with retries, then extract links
vrk coax --times 3 --backoff exp:1s -- vrk grab https://unstable-site.example.com | \
  vrk links --bare
```

## When it fails

All retries exhausted:

```bash
$ vrk coax --times 2 --backoff 100ms -- false
coax: attempt 1/3 failed (exit 1), waiting 100ms
coax: attempt 2/3 failed (exit 1), waiting 100ms
coax: attempt 3/3 failed (exit 1), all retries exhausted
$ echo $?
1
```

The exit code from the last attempt is returned.

Missing command:

```bash
$ vrk coax --times 3
usage error: coax: no command specified (use -- before the command)
$ echo $?
2
```
