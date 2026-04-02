---
title: "vrk throttle"
description: "vrk throttle paces LLM batch jobs to stay within API rate limits. Pipe any stream through it - no more failures at job 847 of 10,000. One flag: --rate N/s or N/m."
meta_title: "vrk throttle - Rate Limiter for LLM API Batch Jobs"
meta_lead: "vrk throttle is a rate limiter for LLM API batch jobs - pace your pipeline to stay within API limits."
og_title: "vrk throttle - rate-limit-aware pacing for LLM batch jobs"
tool: throttle
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

vrk throttle is a rate limiter for LLM API batch jobs - pace your pipeline to stay within API limits.

## The problem

A pipeline sends 10,000 requests to an LLM API that allows 60 per minute. It fires as fast as stdin delivers and gets rate-limited after the first 60. Adding `sleep 1` between requests is too conservative and turns a 3-minute job into a 3-hour one. `sleep` is per-iteration, not per-second, and ignores processing time.

## The solution

`vrk throttle` paces pipeline flow to a specified rate. `--rate 10/s` or `--rate 100/m` delays lines to match. `--burst` lets the first N lines through immediately. `--tokens-field` enables token-aware rate limiting when records consume different API quotas.

## Before and after

**Before**

```bash
cat records.jsonl | while read line; do
  process "$line"
  sleep 1  # Too slow. 60/min API allows faster.
done
```

**After**

```bash
cat records.jsonl | vrk throttle --rate 60/m | process-each
```

## Example

```bash
cat records.jsonl | vrk throttle --rate 10/s --burst 5
```

## Exit codes

| Code | Meaning                                                    |
|------|------------------------------------------------------------|
| 0    | All lines emitted at specified rate                        |
| 1    | Stdin read error, write error, or --tokens-field not found |
| 2    | --rate missing or invalid, interactive TTY                 |

## Flags

| Flag             | Short | Type   | Description                                               |
|------------------|-------|--------|-----------------------------------------------------------|
| `--rate`         | -r    | string | Rate limit in N/s or N/m format (required)                |
| `--burst`        |       | int    | Emit first N lines immediately before applying rate limit |
| `--tokens-field` |       | string | Dot-path to JSONL field for token-based rate limiting     |
| `--json`         | -j    | bool   | Append metadata record after all output                   |
| `--quiet`        | -q    | bool   | Suppress stderr output                                    |


<!-- notes - edit in notes/throttle.notes.md -->

## How it works

### Rate limiting

```bash
# Allow 10 lines per second
seq 20 | vrk throttle --rate 10/s

# Allow 100 lines per minute
cat records.jsonl | vrk throttle --rate 100/m
```

Lines are delayed to maintain the target rate. Output is the same as input, just paced.

### Burst (--burst)

Let the first N lines through immediately, then enforce the rate:

```bash
cat records.jsonl | vrk throttle --rate 5/s --burst 10
```

The first 10 lines arrive instantly. After that, 5 per second. Use this for APIs that allow burst traffic but enforce sustained rate limits.

### Token-aware rate limiting (--tokens-field)

When different records consume different amounts of API quota:

```bash
cat chunks.jsonl | vrk throttle --rate 100000/m --tokens-field tokens
```

Instead of counting lines, throttle counts the value of the `tokens` field in each JSONL record. This keeps you under token-per-minute limits even when chunk sizes vary.

### JSON metadata (--json)

```bash
$ seq 5 | vrk throttle --rate 10/s --json
1
2
3
4
5
{"_vrk":"throttle","rate":"10/s","lines":5,"elapsed_ms":500}
```

## Pipeline integration

### Rate-limit LLM calls

```bash
# Process JSONL records through an LLM at 10 requests per second
cat data.jsonl | vrk throttle --rate 10/s | \
  while IFS= read -r record; do
    echo "$record" | jq -r '.text' | \
      vrk prompt --system 'Classify this text'
  done
```

### Throttle web fetches

```bash
# Fetch URLs at a polite rate
cat urls.txt | vrk throttle --rate 2/s | \
  while IFS= read -r url; do
    vrk grab "$url" | vrk tok --json | jq -r '.tokens'
  done
```

### Sample, throttle, then process

```bash
# Take a sample, pace the processing, and log results
cat large-dataset.jsonl | \
  vrk sip --count 100 --seed 42 | \
  vrk throttle --rate 5/s | \
  while IFS= read -r record; do
    RESULT=$(echo "$record" | vrk prompt --system 'Analyze')
    echo "$RESULT" | vrk emit --tag analysis
  done
```

## When it fails

Missing --rate:

```bash
$ seq 10 | vrk throttle
usage error: throttle: --rate is required
$ echo $?
2
```

Invalid rate format:

```bash
$ seq 10 | vrk throttle --rate 10/h
usage error: throttle: invalid rate format (use N/s or N/m)
$ echo $?
2
```
