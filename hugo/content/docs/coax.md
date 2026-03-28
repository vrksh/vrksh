---
title: "vrk coax"
description: "Retry wrapper - --times, --backoff, --on, --until."
tool: coax
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

API calls fail. Network requests time out. Rate limits return 429s. Without retry logic, your pipeline dies on the first transient error and leaves you with a partial result and a re-run you didn't want to do manually. Writing retry loops in bash is tedious, brittle (exponential backoff with a cap is not a one-liner), and easy to get wrong in edge cases like stdin not being re-supplied to each attempt.

## The fix

```bash
vrk coax --times 5 --backoff exp:200ms -- vrk grab https://api.example.com/data
```

The command after `--` is re-run up to `--times` additional times if it exits non-zero. Stdout and stderr from the subprocess pass through to the terminal unchanged.

## Walkthrough

**Happy path** - the command succeeds on the first try and coax exits with the same code:

```bash
vrk coax --times 3 -- echo "hello"
# hello
```

Between attempts, coax prints progress to stderr:

```
coax: attempt 1 failed (exit 1), retrying in 200ms (1/3)
coax: attempt 2 failed (exit 1), retrying in 400ms (2/3)
```

**Failure case** - if all attempts fail, coax exits with the last exit code from the command. There is no wrapping or translation. A command that exits 1 on every attempt causes `vrk coax` to exit 1.

```bash
vrk coax --times 2 -- false
echo $?   # 1
```

**Fixed backoff** - `--backoff 500ms` waits the same duration before every retry:

```bash
vrk coax --times 4 --backoff 500ms -- curl -f https://example.com/health
```

**Exponential backoff** - `--backoff exp:100ms` doubles the delay after each attempt: 100ms, 200ms, 400ms. Cap it with `--backoff-max`:

```bash
vrk coax --times 10 --backoff exp:100ms --backoff-max 5s -- ./upload.sh
```

**Retry on specific exit codes** - `--on` makes coax retry only when the exit code matches. Other exit codes abort immediately. Useful when you want to retry on rate-limit responses (exit 2 from your script) but not on auth errors (exit 3):

```bash
vrk coax --times 5 --on 2 -- ./fetch_with_exit_codes.sh
```

`--on` is repeatable: `--on 1 --on 2` retries on either exit code.

**Condition-based retry** - `--until` runs a shell command after each attempt and retries while the condition command exits non-zero. This lets you retry until a file appears, a port is open, or an API returns a certain value:

```bash
vrk coax --times 10 --backoff 1s --until 'test -f /tmp/ready.flag' -- ./start_service.sh
```

**Stdin buffering** - coax reads all of stdin once, buffers it in memory, and re-supplies the same bytes to each attempt. This means piped input works correctly across retries:

```bash
cat payload.json | vrk coax --times 3 -- curl -sf -d @- https://api.example.com/ingest
```

**Quiet mode** - `--quiet` suppresses coax's own progress lines. The subprocess's stderr still passes through. Use this when you want clean output in CI:

```bash
vrk coax --times 5 --backoff exp:100ms --quiet -- ./deploy.sh
```

## Pipeline example

Retry a flaky API endpoint and feed the result into the rest of a pipeline:

```bash
vrk coax --times 5 --backoff exp:200ms -- vrk grab https://flaky-api.example.com/data \
  | vrk jsonl \
  | vrk mask
```

Retry a prompt call that might hit a rate limit, then store the result:

```bash
vrk coax --times 3 --backoff 2s --on 1 -- \
  sh -c 'cat doc.txt | vrk prompt --system "Summarize this document" --model claude-sonnet-4-6' \
  | vrk kv set --ns summaries doc_latest
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--times` | | int | 3 | Number of retries. Total attempts = `--times + 1` (first attempt is free). |
| `--backoff` | | string | `""` | Delay between retries. `500ms` for fixed delay, `exp:100ms` for exponential doubling. |
| `--backoff-max` | | duration | 0 | Cap for exponential backoff. `0` means uncapped. |
| `--on` | | []int | `[]` | Retry only on these exit codes. Repeatable. Default: retry on any non-zero. |
| `--until` | | string | `""` | Shell condition command. Retry while this command exits non-zero. |
| `--quiet` | `-q` | bool | false | Suppress coax's own retry progress lines. Subprocess stderr always passes through. |
| `--json` | `-j` | bool | false | Emit errors as JSON |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Final attempt succeeded, or `--until` condition passed |
| non-zero | Last exit code from the wrapped command after all retries |
| 2 | Usage error - missing command after `--`, `--times` less than 1, invalid `--backoff` spec |
