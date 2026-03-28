---
title: "vrk assert"
description: "Pipeline condition check - jq conditions, --contains, --matches"
tool: assert
group: utilities
mcp_callable: true
noindex: false
---

## The problem

Your pipeline fetches data, transforms it, and passes it downstream - but you have no gate between steps. A degraded API returns a `200` with `{"status":"error"}` in the body, and your pipeline processes it as if everything is fine. A schema change drops a field and your downstream prompt gets null where it expected a string. Without an assertion step, bad data flows silently and the failure surfaces three steps later in a confusing error.

`assert` is a pipeline gate. It reads input, checks a condition, passes the data through on success, and exits non-zero on failure - stopping `set -e` pipelines immediately. Two modes: jq expressions for structured JSON, and `--contains`/`--matches` for plain text.

## The fix

```bash
vrk grab https://api.example.com/health | vrk assert --contains '"status":"ok"'
```

<!-- output: verify against binary -->

If the response body contains the substring, the body is passed through to stdout and the exit code is 0. If it doesn't, exit 1 and a failure message goes to stderr (or JSON to stdout if `--json`).

## Walkthrough

### Plain text: --contains

```bash
echo "build succeeded" | vrk assert --contains "succeeded"
echo "build failed" | vrk assert --contains "succeeded"
```

<!-- output: verify against binary -->

`--contains` is a literal substring check - no glob, no regex. The full stdin is read, checked, and passed through on success. On failure, exit 1.

### Plain text: --matches

```bash
echo "request_id: abc-123" | vrk assert --matches '^request_id: [a-z]+-[0-9]+'
echo "no id here" | vrk assert --matches '^request_id:'
```

<!-- output: verify against binary -->

`--matches` takes a Go regular expression. Invalid regex exits 2. The match is applied to the full input, not line by line. Data passes through on success.

### Custom failure message

```bash
vrk grab https://api.example.com/health \
  | vrk assert --contains '"status":"ok"' \
      --message "health check failed - API may be down"
```

<!-- output: verify against binary -->

`--message` replaces the default failure message on stderr. Useful in CI scripts where the default "assertion failed" message doesn't have enough context for whoever reads the log.

### jq mode: JSONL conditions

```bash
echo '{"score": 0.9, "label": "positive"}' | vrk assert '.score > 0.5'
echo '{"score": 0.1, "label": "negative"}' | vrk assert '.score > 0.5'
```

<!-- output: verify against binary -->

Positional arguments are jq expressions applied to each line of JSONL input. A line passes if the expression evaluates to something other than `null` or `false`. The passing line is emitted to stdout. A failing line exits 1.

`null` and `false` are the only falsy values. `0`, `""`, and `[]` are truthy - consistent with jq semantics and intentionally different from most languages.

### Multiple jq conditions (AND logic)

```bash
echo '{"score": 0.9, "label": "positive"}' \
  | vrk assert '.score > 0.5' '.label != null'
```

<!-- output: verify against binary -->

Multiple positional arguments are AND-ed. All conditions must pass for a line to be emitted. The first failing condition determines the failure message.

### JSON output on failure

```bash
echo '{"score": 0.1}' | vrk assert --json '.score > 0.5'
```

<!-- output: verify against binary -->

```json
{"error":"assert: condition failed: .score > 0.5","code":1}
```

With `--json`, errors go to stdout as a JSON object. Stderr is empty. Useful when `assert` is part of a pipeline that collects structured output.

### Mode exclusivity

You can't mix jq expressions with `--contains`/`--matches`. They operate on different input models - jq works line-by-line on JSONL, while `--contains`/`--matches` operate on the full stdin as a string. Mixing exits 2.

## Pipeline example

Gate a pipeline on a health check before running an expensive prompt:

```bash
vrk grab https://api.example.com/health \
  | vrk assert --contains '"status":"ok"' \
      --message "API health check failed" \
  && cat user-data.jsonl \
  | vrk prompt "Analyze this data" \
  | vrk kv set --ns results latest
```

Filter a JSONL stream to only high-confidence records before downstream processing:

```bash
cat predictions.jsonl \
  | vrk assert '.confidence > 0.8' \
  | vrk prompt "These are high-confidence predictions. Summarize the key themes."
```

Verify that a generated file has the expected structure before committing:

```bash
cat generated-config.json \
  | vrk assert '.version != null' '.endpoints | length > 0' \
      --message "generated config is missing required fields"
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--contains` | | string | `""` | Assert stdin contains this literal substring |
| `--matches` | | string | `""` | Assert stdin matches this regular expression |
| `--message` | `-m` | string | `""` | Custom message on failure (replaces default stderr output) |
| `--json` | `-j` | bool | `false` | Emit errors as JSON to stdout; keep stderr empty |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output on failure |

**Positional arguments:** jq condition expressions (e.g. `.status == "ok"`). Cannot be combined with `--contains` or `--matches`.

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | All conditions passed; input passed through to stdout |
| 1 | Assertion failed - condition not met, or runtime error (I/O, invalid jq expression) |
| 2 | Usage error - no condition specified, `--contains` and `--matches` mixed with jq args, invalid regex, interactive TTY |
