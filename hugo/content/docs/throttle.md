---
title: "vrk throttle"
description: "Rate limiter for pipes - --rate N/s or N/m"
tool: throttle
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You are hitting an API rate limit. Or you are feeding a queue of prompts to an LLM and need to stay under token-per-minute caps. Or you are fetching a list of URLs and the server starts returning 429s after the first dozen. The usual fix is a `sleep` call inside a loop - which is fragile, hard to tune, and invisible to the rest of the pipeline.

`vrk throttle` is the composable version. It sits between the source and the consumer, reads lines from stdin, and re-emits them with controlled delays. The rate is explicit, the burst window is configurable, and the whole thing disappears when you remove it from the pipe.

## The fix

```bash
cat urls.txt | vrk throttle --rate 10/s | xargs -I{} vrk grab {}
```

`vrk throttle` is stdin-only. It is a stream filter for an unbounded line stream. There is no positional argument form.

## Walkthrough

**Simple line-rate limiting**

`--rate` accepts `N/s` (lines per second) or `N/m` (lines per minute). N must be a positive integer.

```bash
seq 20 | vrk throttle --rate 5/s
```

Five lines per second. Adjust the denominator to match your API's documented rate limit.

**Allowing an initial burst**

Some APIs allow a burst window: the first N requests can go immediately, then the rate limit kicks in. `--burst` emits the first N lines without delay.

```bash
cat prompts.txt | vrk throttle --rate 20/m --burst 5
```

The first 5 lines go out immediately. Lines 6 onwards are spaced 3 seconds apart.

**Token-based rate limiting for LLM pipelines**

LLM providers cap by tokens per minute, not requests per minute. `--tokens-field` reads a JSONL field from each input line, counts its tokens using the `cl100k_base` tokeniser, and spaces the lines accordingly so the total token flow stays within the limit.

```bash
cat prompts.jsonl | vrk throttle --rate 40000/m --tokens-field content
```

Each line must be valid JSON with the named field. The field value is counted as `cl100k_base` tokens. The interval between lines is set to `tokens / rate` seconds.

**Checking throughput with `--json`**

The `--json` flag appends a metadata record after all output:

```bash
seq 10 | vrk throttle --rate 5/s --json
```

The final line looks like:

```json
{"_vrk":"throttle","rate":"5/s","lines":10,"elapsed_ms":1802}
```

`elapsed_ms` is wall-clock time from first line to last.

**Combining with other pipeline tools**

```bash
cat urls.txt | vrk throttle --rate 10/s | xargs -I{} vrk grab --text {}
```

Ten URL fetches per second. The pipeline is readable, the rate is explicit, and removing `vrk throttle` instantly returns to full speed.

## Pipeline example

```bash
cat urls.txt | vrk throttle --rate 10/s | xargs -I{} vrk grab --text {}
```

Read a list of URLs one per line, limit throughput to 10 per second, and fetch each one as plain text. Stays well within most server rate limits without a single `sleep` in sight.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--rate` | `-r` | string | `""` | Rate limit in `N/s` or `N/m` format (required) |
| `--burst` | | int | 0 | Emit the first N lines immediately before applying the rate limit |
| `--tokens-field` | | string | `""` | Dot-path to a JSONL field; rate is applied by token count of that field's value |
| `--json` | `-j` | bool | false | Append a `{"_vrk":"throttle","rate":"...","lines":N,"elapsed_ms":N}` record after all output |
| `--quiet` | `-q` | bool | false | Suppress stderr output; exit codes are unaffected |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success - all lines emitted at the specified rate |
| 1 | Runtime error - stdin read error, write error, or `--tokens-field` field not found in a record |
| 2 | Usage error - `--rate` is missing or invalid, interactive TTY with no piped input |
