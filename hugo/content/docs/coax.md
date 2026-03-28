---
title: "vrk coax"
description: "Retry wrapper - --times, --backoff, --on, --until."
tool: coax
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

A command fails. You want to retry it with backoff. Writing a retry loop
in bash is 10 lines of boilerplate every time -- sleep calculations,
attempt counters, exit code handling. It's tedious, brittle, and easy
to get wrong.

## The fix

```bash
vrk coax -- vrk prompt --system "Extract structured data."
```

The command after `--` is run once. If it exits non-zero, coax retries
up to 3 times (the default) with no delay. Stdout and stderr from the
subprocess pass through unchanged.

If the command succeeds on the first try, coax exits with the same code.
If all attempts fail, coax exits with the last exit code from the command.

## Configuring retries

```bash
vrk coax --times 5 --backoff 2s -- curl -sf https://api.example.com/data
```

`--times 5` means 5 retries (6 total attempts). `--backoff 2s` waits
two seconds between each retry. For exponential backoff:

```bash
vrk coax --times 5 --backoff exp:200ms -- vrk grab https://flaky-api.example.com
```

`exp:200ms` doubles the delay after each attempt: 200ms, 400ms, 800ms.
Cap it with `--backoff-max 5s`.

Between attempts, coax prints progress to stderr:

```
coax: attempt 1 failed (exit 1), retrying in 200ms (1/5)
coax: attempt 2 failed (exit 1), retrying in 400ms (2/5)
```

## In a pipeline

Stdin is buffered once and re-supplied to each attempt, so piped input
works correctly across retries:

```bash
echo "$CONTENT" | vrk coax --times 3 --backoff 1s -- vrk prompt --schema person.json
```

If the prompt call fails (rate limit, network timeout, schema validation),
coax retries the whole command with the same input.

Retry only on specific exit codes with `--on`:

```bash
vrk coax --times 5 --on 1 -- ./deploy.sh
```

This retries on exit 1 (transient error) but not exit 2 (usage error).

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--times` | | int | `3` | Number of retries (total attempts = N+1) |
| `--backoff` | | string | `""` | Delay between retries: `500ms` for fixed, `exp:100ms` for exponential |
| `--backoff-max` | | duration | `0` | Cap for exponential backoff; `0` means uncapped |
| `--on` | | []int | `[]` | Retry only on these exit codes; repeatable |
| `--until` | | string | `""` | Shell condition; retry while this exits non-zero |
| `--quiet` | `-q` | bool | `false` | Suppress coax's retry progress lines |
| `--json` | `-j` | bool | `false` | Emit errors as JSON |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Command succeeded (first attempt or a retry) |
| non-zero | Last exit code from the wrapped command after all retries exhausted |
| 2 | Usage error - missing command after `--`, invalid `--backoff` spec |
