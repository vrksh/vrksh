# vrksh Skills

This file is embedded in the `vrk` binary and served via `vrk --skills`.
It is the agent-facing reference for using vrksh tools in AI pipelines.

---

## jwt — JWT Inspector

Decodes a JWT and prints the payload as JSON. Does not verify signatures.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--claim <name>` | `-c` | Print a single claim value as plain text |
| `--expired` | `-e` | Exit 1 if the token is expired |
| `--valid` | — | Exit 1 if expired, nbf in future, or iat in future |
| `--json` | `-j` | Emit structured JSON output (shape depends on other flags) |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### --json output shapes

| Flags | Shape |
|-------|-------|
| `--json` alone | `{"header":{…},"payload":{…},"signature":"…","expired":bool,"valid":bool}` |
| `--expired --json` | `{"expired":bool}` — exit 1 if expired, 0 if not |
| `--claim <name> --json` | `{"claim":"name","value":"…"}` |
| Any error + `--json` | `{"error":"msg","code":N}` on stdout; stderr empty |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — token decoded, condition met |
| 1 | Runtime error — invalid token, expired (with `--expired`), claim not found |
| 2 | Usage error — no input provided, unknown flag |

### Examples

```bash
# Decode a token
vrk jwt "$TOKEN"

# Extract a single claim
vrk jwt --claim sub "$TOKEN"

# Full envelope with header, payload, signature, expired, valid
vrk jwt --json "$TOKEN"

# Guard: exit 1 if token is expired
vrk jwt --expired "$TOKEN"

# Guard with JSON output: {"expired":true/false}, exit 1 if expired
vrk jwt --expired --json "$TOKEN"

# Pipe form
echo "$TOKEN" | vrk jwt --claim sub
```

### Compose patterns

```bash
# Extract sub and use as a key lookup
SUB=$(vrk jwt --claim sub "$TOKEN")
vrk kv get "user:$SUB"

# Decode token from an env var and check expiry before making an API call
vrk jwt --expired "$AUTH_TOKEN" && curl -H "Authorization: Bearer $AUTH_TOKEN" ...

# Check expiry with structured output — pipe-friendly
vrk jwt --expired --json "$AUTH_TOKEN" | jq '.expired'

# Inspect a token mid-pipeline
echo "$TOKEN" | vrk jwt --json | jq '.payload.exp'
```

### Gotchas

- `--expired` exits 1 only if the `exp` claim is present **and** in the past.
  A token with no `exp` claim is treated as never-expiring and exits 0.
- `--json` alone never exits 1 for an expired token — check `expired` field or use `--expired`.
- `--expired --json`: exits 1 when expired, but stdout still has `{"expired":true}` (not empty).
  This differs from `--expired` without `--json`, where stdout is empty on exit 1.
- When `--json` is active, all errors go to stdout as `{"error":"msg","code":N}` and stderr is empty.
- Default output (no flags) prints the payload only. Use `--json` to also get the header and signature.
- Stdout is always empty on error unless `--json` is active.

---

## epoch — Timestamp Converter

Converts between Unix timestamps and ISO 8601 dates/times.
Default output is always a Unix integer. `--iso` switches to ISO 8601.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--iso` | — | Output as ISO 8601 string instead of Unix integer |
| `--json` | `-j` | Emit structured JSON: `{input?, unix, iso, ref?, tz?}` |
| `--tz <zone>` | — | Timezone for `--iso` or `--json` output; IANA name or `+HH:MM` offset |
| `--now` | — | Print current Unix timestamp and exit |
| `--at <ts>` | — | Override reference time for relative input (unix integer) |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### --json output shape

```json
{"input":"+3d","unix":1740268800,"iso":"2025-02-23T00:00:00Z","ref":"1740009600","tz":"+05:30"}
```

- `input`: the original input string — omitted for `--now`
- `unix`: computed Unix timestamp (integer)
- `iso`: always present — ISO 8601 in UTC or the specified `--tz`
- `ref`: only when `--at` was used
- `tz`: only when `--tz` was used

Errors with `--json` active: `{"error":"msg","code":N}` to stdout; stderr empty.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error — unsupported format, missing sign, ambiguous timezone, no input, `--tz` without `--iso` or `--json` |

### Relative time format

Must include a sign prefix: `+3d` (3 days from now), `-3d` (3 days ago).
Bare `3d` exits 2 with "sign required". Units: `s` `m` `h` `d` `w` (no months or years).
Negative relative times (`-3d`, `-2h`, etc.) work as positional args or via stdin.

### Examples

```bash
# Current timestamp
vrk epoch --now

# Unix integer passthrough
echo '1740009600' | vrk epoch

# ISO date to unix (midnight UTC)
echo '2025-02-20' | vrk epoch

# 3 days from now
echo '+3d' | vrk epoch

# 3 days ago — positional or stdin, both work
vrk epoch -3d
echo '-3d' | vrk epoch

# 3 days from now as ISO string
echo '+3d' | vrk epoch --iso

# Deterministic: override reference time so pipelines are reproducible
echo '+3d' | vrk epoch --at 1740009600     # always 1740268800

# Convert unix to ISO with timezone offset
echo '1740009600' | vrk epoch --iso --tz +05:30

# Convert unix to ISO with IANA timezone
echo '1740009600' | vrk epoch --iso --tz America/New_York
```

### Compose patterns

```bash
# Expiry timestamp for a KV entry: set TTL 7 days from now
EXPIRY=$(echo '+7d' | vrk epoch)
vrk kv set session:abc "$TOKEN" --ttl "$EXPIRY"

# Convert a stored timestamp back to human-readable
vrk kv get created_at | vrk epoch --iso

# Deterministic timestamp in CI scripts
CUTOFF=$(vrk epoch -30d --at "$BASELINE")
```

### Gotchas

- Relative times **must** be signed: `+3d` or `-3d`. Bare `3d` exits 2.
- Timezone abbreviations (IST, EST, PST) exit 2 — they are ambiguous across regions.
  Use full IANA names (`America/New_York`) or numeric offsets (`+05:30`).
- `--tz` requires `--iso` or `--json`; using it with plain integer output exits 2.
- Unix integer input is passed through unchanged — timezone affects only `--iso` / `--json` output.
- Use `--at <ts>` to make pipelines involving relative times deterministic.
- `--now` is a boolean flag (prints current timestamp and exits). Use `--at` to set a reference.
- Negative integers (`-1000`) are valid pre-epoch Unix timestamps — pass via stdin to avoid flag parsing.
- When `--json` is active, errors go to stdout as `{"error":"msg","code":N}` and stderr is empty.
- Stdout is always empty on error unless `--json` is active.

---

## uuid — UUID Generator

Generates UUIDs. v4 (random) by default, v7 (time-ordered) with `--v7`.
Reads no stdin — input is never required or consumed.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--v7` | — | Generate a v7 (time-ordered) UUID instead of v4 |
| `--count <n>` | `-n` | Number of UUIDs to generate (default 1, must be >= 1) |
| `--json` | `-j` | Emit each UUID as a JSON object: `{uuid, version, generated_at}` |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error — `--count` less than 1, unknown flag |

### Output format

Plain text (default): one UUID per line, lowercase, with hyphens. Format: `8-4-4-4-12` hex characters.

JSON (`--json`): one object per line (JSONL), always with these three fields:
```json
{"uuid":"a3f2c1d4-8e7b-4f2a-9c1d-3e5f7a8b9c0d","version":4,"generated_at":1740000000}
```
`generated_at` is a Unix timestamp (seconds) computed once per invocation — all UUIDs in a batch share the same value.

### Examples

```bash
# Single v4 UUID (default)
vrk uuid

# Single v7 UUID (time-ordered)
vrk uuid --v7

# Five v4 UUIDs
vrk uuid --count 5

# Five v7 UUIDs — lexicographically ordered (each >= previous)
vrk uuid --v7 --count 5

# JSON output
vrk uuid --json
# → {"uuid":"...","version":4,"generated_at":1740000000}

# JSONL output for a batch
vrk uuid --count 5 --json

# Use as a correlation ID in a pipeline
ID=$(vrk uuid)
vrk prompt "Summarise this" | vrk kv set "result:$ID"
```

### Compose patterns

```bash
# Generate a request ID and thread it through a pipeline
REQ=$(vrk uuid)
cat payload.json | vrk prompt "process this" | vrk kv set "response:$REQ"

# Use v7 UUIDs as time-ordered database keys (sortable without a separate created_at column)
vrk uuid --v7 --count 100 | while read id; do
  echo "$id"
done

# Extract uuid field from JSON output
vrk uuid --json | jq -r '.uuid'

# Batch generation with metadata preserved
vrk uuid --v7 --count 10 --json | jq -r '.uuid'
```

### Gotchas

- `uuid` reads **no stdin**. Piping anything into it is silently ignored — the tool generates UUIDs regardless.
- v7 UUIDs are **lexicographically ordered** within a batch because the library's monotonic counter guarantees each successive UUID is greater than the last, even within the same millisecond.
- `--count 0` exits 2 — it is a usage error, not a no-op.
- `generated_at` is computed **once before the generation loop**, so all UUIDs in a `--count N` batch share the same timestamp. This is intentional — it reflects when the batch was requested, not each individual generation.
- Stdout is always empty on error — errors go to stderr only.

---

## tok — Token Counter and Budget Guard

Counts tokens in stdin using the cl100k_base tokenizer. Exact for GPT-4 family,
~95% accurate for Claude. Optionally fails if over a budget.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--budget <N>` | — | Exit 1 if token count exceeds N |
| `--model <name>` | `-m` | Tokenizer model label (default: `cl100k_base`; only cl100k_base is currently implemented) |
| `--json` | `-j` | Emit output as `{"tokens": N, "model": "cl100k_base"}` |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — count printed (or JSON emitted), within budget if `--budget` was set |
| 1 | Over budget — `--budget N` was set and token count exceeds N |
| 2 | Usage error — unknown flag; running interactively with no piped input |

### Examples

```bash
# Count tokens in a file
cat prompt.txt | vrk tok

# Count a string directly
vrk tok "hello world"
# → 2

# JSON output
echo "hello world" | vrk tok --json
# → {"tokens":2,"model":"cl100k_base"}

# Budget guard — exit 1 if over 4000 tokens
cat prompt.txt | vrk tok --budget 4000

# Budget guard with --fail (identical to --budget alone on tok)
cat prompt.txt | vrk tok --budget 4000 --fail

# Count with explicit model label
echo "hello world" | vrk tok --model cl100k_base
# → 2
```

### Compose patterns

```bash
# Pre-flight check before sending to an LLM — abort if too large
cat prompt.txt | vrk tok --budget 4000 && cat prompt.txt | vrk prompt "summarise this"

# Gate in a pipeline — nothing downstream runs if over budget
cat big_context.txt | vrk tok --budget 8000 | vrk prompt "answer: $QUESTION"
# (vrk tok exits 1 and passes nothing to vrk prompt when over budget)

# Count tokens and store result
TOKENS=$(cat prompt.txt | vrk tok)
echo "Sending $TOKENS tokens to the model"

# JSON output for structured logging
cat prompt.txt | vrk tok --json | vrk kv set "last_prompt_tokens"

# CI size gate — fail build if generated prompt is too large
cat generated_prompt.txt | vrk tok --budget 100000 || { echo "Prompt too large"; exit 1; }
```

### Gotchas

- **cl100k_base is approximate for Claude (~95% accurate).** The exact Claude tokenizer is not publicly available. Set `--budget` at 90% of the model's actual context limit to absorb the error margin.
- **`--budget` is the only guard flag on `tok`** — it exits 1 when exceeded. `tok` has no `--fail` flag; passing it is a usage error (exit 2).
- **Empty pipe is 0 tokens, not an error.** `cat /dev/null | vrk tok` exits 0 and prints `0`. Only running `vrk tok` interactively in a terminal (no pipe) exits 2.
- **When budget is exceeded, stdout is empty.** The count is reported only on success. On exit 1, only stderr contains the message. This makes `vrk tok --budget N | next-command` safe — `next-command` receives no input when the budget check fails.
- **`--json` does not change error format.** When budget is exceeded, stderr is always plain text regardless of `--json`. Stdout is still empty on exit 1.
- Stdout is always empty on error — errors go to stderr only.

---

## sse — SSE Stream Parser

Reads a raw `text/event-stream` from stdin, parses it, and emits one JSON object
per event to stdout (JSONL). No input source other than stdin — pipe your HTTP
stream directly into it.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--event <name>` | `-e` | Only emit events of this type; skip all others |
| `--field <path>` | `-F` | Extract a dot-path field from the record and print as plain text |

No `--json`: output is already JSONL by default.
No `--quiet`: `sse` produces no informational stderr in normal operation.

### Output format

Each emitted record is a JSON object on its own line (JSONL):

```json
{"event":"message","data":{"text":"hello"}}
{"event":"content_block_delta","data":{"delta":{"text":"hi"}}}
```

- `event`: the SSE event type. Defaults to `"message"` when no `event:` field is present.
- `data`: the parsed JSON value from the `data:` field. If the value is not valid JSON, it is emitted as a raw string.

### --field path

The dot-path navigates from the top-level record (which has `event` and `data` keys):

```bash
vrk sse --field data.delta.text   # extracts the nested text token
vrk sse --field event             # extracts the event name itself
```

- String values are printed as-is (no quotes).
- Number and boolean values are printed as their JSON representation (`42`, `true`).
- If the path is not found, the record is skipped silently.
- If `data` is not a JSON object, the record is skipped silently.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — stream parsed, `[DONE]` encountered, or stdin closed cleanly |
| 1 | Runtime error — I/O error reading stdin |
| 2 | Usage error — no stdin when running interactively, unknown flag |

### Examples

```bash
# Parse a full SSE stream → JSONL
curl -N https://api.example.com/stream | vrk sse

# Filter to one event type
curl -N ... | vrk sse --event content_block_delta

# Extract text tokens as plain text, one per line
curl -N ... | vrk sse --event content_block_delta --field data.delta.text

# Accumulate streaming text tokens into a single string
curl -N ... | vrk sse --event content_block_delta --field data.delta.text | tr -d '\n'

# Pipe from a saved SSE log
cat stream.log | vrk sse --event message
```

### Compose patterns

```bash
# Anthropic streaming: extract text tokens then join
curl -sN https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "anthropic-version: 2023-06-01" \
  -H "content-type: application/json" \
  -d '{"model":"claude-sonnet-4-6","max_tokens":256,"stream":true,"messages":[{"role":"user","content":"hi"}]}' \
  | vrk sse --event content_block_delta --field data.delta.text \
  | tr -d '\n'

# Store each token in kv as it arrives (live streaming to state)
curl -sN ... | vrk sse --event content_block_delta --field data.delta.text \
  | while read -r token; do vrk kv set last_token "$token"; done

# Count events by type
curl -sN ... | vrk sse --field event | sort | uniq -c

# Pipe sse output into jq for further filtering
curl -sN ... | vrk sse | jq 'select(.event == "content_block_delta") | .data.delta.text'
```

### Gotchas

- **Trailing blank line required for dispatch.** SSE blocks are only dispatched when a blank line is encountered. If stdin closes mid-block (no trailing `\n\n`), the pending block is silently dropped. Real HTTP streaming servers always send the trailing blank line.
- **`[DONE]` stops the stream regardless of `--event`.** The `[DONE]` sentinel (used by Anthropic and OpenAI) is a protocol signal, not a data event. It terminates parsing immediately even if an `--event` filter is active.
- **No stdin in a terminal exits 2.** Running `vrk sse` interactively with no pipe is a usage error. Pipe a stream or redirect a file.
- **SSE space stripping follows the spec.** Exactly one leading space is stripped from field values after the colon: `data: hello` → `hello`, `data:  hello` → ` hello` (one stripped, one remains).
- **Multi-line `data:` fields are concatenated with `\n`.** Two consecutive `data:` lines in one block join as `val1\nval2`. The resulting string is then parsed as JSON if valid, otherwise kept as a raw string.
- **`data:` with no value contributes an empty string** to the multi-line accumulation buffer. This is per the SSE spec.
- **Non-JSON `data:` values are emitted as strings**, not dropped. This means comment-like lines (`data: [DONE]`, `data: ping`) appear in the output as `{"event":"message","data":"[DONE]"}` — except `[DONE]` which is intercepted as the termination sentinel before emission.
- **`--field` skips silently on missing paths or non-JSON data.** No error, no output for that record. This is intentional — real streams mix JSON and non-JSON events.
- **Stdout is always empty on error.** Errors go to stderr only.
