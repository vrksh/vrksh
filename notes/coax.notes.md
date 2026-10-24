## How the command runs

coax runs the command directly, not through a shell. For pipes or redirects, use `-- sh -c '...'`.

Arguments reach the command exactly as you typed them, so quoting survives and data is never run as code:

```bash
$ vrk coax --times 1 -- printf '%s\n' 'a b'
a b
$ vrk coax --times 1 -- echo 'x; echo INJECTED'
x; echo INJECTED
```

The exit code is the command's own. A command killed by a signal gives 128 plus the signal number (143 for SIGTERM). coax forwards SIGINT and SIGTERM to the running command and does not retry after it receives one.

### Migrating from v0.1

v0.1 joined everything after `--` into one string and ran it with `sh -c`. One-string commands like `vrk coax -- 'curl x | jq'` no longer work: coax now looks for a program literally named `curl x | jq`. Wrap them in `sh -c`:

```bash
vrk coax --times 3 -- sh -c 'curl -sf https://api.example.com | jq .data'
```

## Stdin replay

coax does not read stdin up front. The first attempt reads stdin as it needs it, and coax keeps a copy. A retry gets the saved bytes followed by any stdin the first attempt never read, so every attempt sees the same input. A command that ignores stdin starts and finishes at once, even when stdin is a pipe that never closes.

coax keeps up to 64 MiB for replay. Bigger input still streams through on the first attempt, but a retry fails with `coax: stdin larger than 64 MiB cannot be replayed` (exit 1).

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

This runs the command, then checks the `--until` condition. If the condition exits 0, coax stops. Unlike the command, `--until` is a shell string and runs through `sh -c`.

### What retry progress looks like

Without `--quiet`, coax prints progress to stderr:

```
coax: attempt 1 failed (exit 1), retrying in 200ms (1/3)
coax: attempt 2 failed (exit 1), retrying in 400ms (2/3)
coax: attempt 3 failed (exit 1), retrying in 800ms (3/3)
coax: all 4 attempts failed (last exit 1)
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
coax: attempt 1 failed (exit 1), retrying in 100ms (1/2)
coax: attempt 2 failed (exit 1), retrying in 100ms (2/2)
coax: all 3 attempts failed (last exit 1)
$ echo $?
1
```

The exit code from the last attempt is returned.

Command not found (not retried):

```bash
$ vrk coax --times 3 -- no-such-command
coax: command not found: no-such-command
$ echo $?
1
```

Missing command:

```bash
$ vrk coax --times 3
usage error: missing command: use -- to separate command from coax flags
$ echo $?
2
```
