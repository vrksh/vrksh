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

---

## kv — Persistent Key-Value Store

Stores key-value pairs in `~/.vrk.db` (SQLite, WAL mode). Namespaced with `--ns`.
Database path overridden by `VRK_KV_PATH`.
Input: positional arguments. `kv set` reads value from stdin when the value argument is absent.

### Subcommands

| Subcommand | Usage | Description |
|------------|-------|-------------|
| `set` | `vrk kv set [flags] <key> [value]` | Store a value; reads from stdin when value arg is absent |
| `get` | `vrk kv get [flags] <key>` | Print value; exit 1 if not found or expired |
| `del` | `vrk kv del [flags] <key>` | Delete a key; silent if absent |
| `list` | `vrk kv list [flags]` | List all keys in namespace, sorted alphabetically |
| `incr` | `vrk kv incr [flags] <key>` | Increment integer value by 1 (or `--by N`); missing key starts at 0 |
| `decr` | `vrk kv decr [flags] <key>` | Decrement integer value by 1 (or `--by N`); missing key starts at 0 |

### Flags

| Flag | Subcommands | Type | Default | Description |
|------|-------------|------|---------|-------------|
| `--ns` | all | string | `"default"` | Namespace; keyspaces are isolated |
| `--ttl` | `set` | duration | `0` | Expiry duration (`1s`, `5m`, `24h`); 0 = no expiry |
| `--dry-run` | `set` | bool | false | Print intent without writing to db |
| `--by` | `incr`, `decr` | int | `1` | Delta; must be ≥ 1 |

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | `set`, `del`, `list`, `incr`, `decr` — success |
| 0 | `get` — key found and not expired |
| 1 | `get` — key not found or expired |
| 1 | `incr`/`decr` — stored value is not a parseable integer |
| 1 | any — database open or write failure |
| 2 | any — missing subcommand, unknown subcommand, unknown flag, `--by` < 1 |

### Examples

```bash
# Basic set / get / del
vrk kv set mykey myvalue
vrk kv get mykey          # → myvalue
vrk kv del mykey

# Overwrite
vrk kv set mykey newvalue
vrk kv get mykey          # → newvalue

# get on missing key → exit 1, stderr "key not found"
vrk kv get nonexistent

# Empty string is a valid value
vrk kv set mykey ""
vrk kv get mykey          # → (empty line)

# Read value from stdin
echo '{"status":"done"}' | vrk kv set result
vrk kv get result         # → {"status":"done"}

# list — sorted alphabetically, one key per line
vrk kv list

# Namespace isolation
vrk kv set --ns myjob step 3
vrk kv get --ns myjob step   # → 3
vrk kv get step              # → exit 1 (namespaces are isolated)

# TTL expiry
vrk kv set expiring value --ttl 1s
sleep 2
vrk kv get expiring          # → exit 1

# Dry run — prints intent, nothing written
vrk kv set result done --dry-run
# → would set result = done

# incr / decr (missing key starts at 0)
vrk kv incr counter          # → 1
vrk kv incr counter          # → 2
vrk kv incr counter --by 5   # → 7
vrk kv decr counter          # → 6

# incr on non-numeric value → exit 1
vrk kv set counter notanumber
vrk kv incr counter          # → exit 1, stderr "value is not a number"
```

### Compose patterns

```bash
# Cache an LLM response keyed by UUID
REQ=$(vrk uuid)
vrk prompt "Summarise this" < input.txt | vrk kv set "response:$REQ"
vrk kv get "response:$REQ"

# Per-user key from a JWT sub claim
SUB=$(vrk jwt --claim sub "$TOKEN")
vrk kv set "session:$SUB" "$SESSION_DATA"

# Compute expiry timestamp with epoch, then use as TTL reference
vrk kv set session:abc "$TOKEN" --ttl 3600s

# Store prompt token count for auditing
cat prompt.txt | vrk tok --json | vrk kv set last_prompt_tokens

# Job progress tracking across pipeline stages
vrk kv incr --ns job:42 steps_completed

# Gate on step completion before proceeding
DONE=$(vrk kv get --ns job:42 step_1_done 2>/dev/null) || true
if [ "$DONE" = "1" ]; then echo "already done"; fi

# 10 parallel workers, each storing results by worker ID
for i in $(seq 1 10); do
  vrk prompt "process batch $i" | vrk kv set --ns run:$RUN "result:$i" &
done
wait
```

### Gotchas

- **Empty string is a valid value.** `vrk kv set mykey ""` stores and returns an empty string. `get` exits 0 and prints an empty line. This is not the same as key-not-found.
- **Stdin value for `set`.** When the value positional argument is absent, `kv set` reads from stdin. Exactly one trailing newline is stripped (matching `echo` behaviour). This means `echo 'val' | vrk kv set key` and `vrk kv set key val` produce identical stored values.
- **Namespaces are isolated.** `--ns a` and `--ns b` never share keys. The default namespace is `"default"`. Using `--ns` consistently is required — omitting it on `get` after setting with `--ns` always gives "key not found".
- **`incr`/`decr` on missing key starts at 0.** The first `vrk kv incr counter` on a fresh database stores and prints `1`. The first `vrk kv decr counter` stores `-1`. This mirrors shell counter idioms.
- **`incr`/`decr` on non-numeric value exits 1.** Stored values must be parseable as 64-bit integers. Float strings (`"1.5"`) and alphabetic strings fail with "value is not a number".
- **TTL precision is whole seconds.** `--ttl 1500ms` rounds down to 1 second. Sub-second TTLs are not a real use case for a persistent store.
- **`list` output is sorted alphabetically.** This is deterministic — safe to diff and assert in scripts. No namespace prefix is included.
- **`del` on a missing key exits 0.** This matches filesystem `rm -f` semantics.
- **Concurrent writers serialise cleanly.** `kv` uses SQLite WAL mode with `PRAGMA busy_timeout=5000`. The `incr`/`decr` operations use `BEGIN IMMEDIATE` to take the write lock before reading, preventing lost updates. Running 10 parallel `vrk kv incr counter` processes always yields a final value of 10.
- **Do not store secrets in `kv`.** The database is plaintext SQLite at `~/.vrk.db`. Use env vars or the system keychain for credentials.
- **Stdout is always empty on error.** Errors go to stderr only.

---

## coax — Retry Wrapper

Retries any shell command on failure. Understands exit codes, fixed and exponential
backoff, and condition-based retry (`--until`). Stdin is buffered and re-piped to
the command on every attempt.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--times N` | — | Number of retries (default 3); total attempts = N+1. Must be >= 1. |
| `--backoff <spec>` | — | Delay between retries: `100ms` for fixed, `exp:100ms` for exponential |
| `--backoff-max <d>` | — | Cap for exponential backoff; 0 = uncapped |
| `--on <code>` | — | Retry only when exit code matches; repeatable: `--on 1 --on 2`. Default: any non-zero exit. |
| `--until <cmd>` | — | Shell command; retry until it exits 0 (takes precedence over `--on`) |
| `--quiet` | `-q` | Suppress coax's own retry progress lines to stderr; subprocess stderr always passes through |

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Command exits 0 on some attempt, or `--until` condition exits 0 |
| last cmd code | All retries exhausted — coax passes through the last command's actual exit code |
| 2 | Usage error — `--times < 1`, no command, unknown flag, bad `--backoff` format |

### Backoff formats

| Spec | Behaviour |
|------|-----------|
| (absent) | No delay |
| `100ms` | Fixed 100ms between every retry |
| `exp:100ms` | Exponential: 100ms, 200ms, 400ms, … |
| `exp:100ms` + `--backoff-max 150ms` | Exponential capped: 100ms, 150ms, 150ms, … |

### Examples

```bash
# Retry up to 3 times (default), exit with last command's exit code
vrk coax -- exit 1

# Retry exactly 2 times (3 total attempts)
vrk coax --times 2 -- my-flaky-command

# Retry only when exit code is 42
vrk coax --on 42 -- my-command

# Fixed 500ms between retries
vrk coax --times 5 --backoff 500ms -- curl -sf https://api.example.com/health

# Exponential backoff: 100ms, 200ms, 400ms
vrk coax --times 3 --backoff exp:100ms -- my-command

# Exponential backoff capped at 1s
vrk coax --times 5 --backoff exp:200ms --backoff-max 1s -- my-command

# Retry until a condition is satisfied (service health check)
vrk coax --times 10 --backoff 500ms --until 'curl -sf localhost:8080/health' -- systemctl start myservice

# Re-pipe stdin to each attempt
echo '{"query":"hello"}' | vrk coax --times 3 -- curl -sf -d @- https://api.example.com/

# Suppress coax progress lines (subprocess stderr still passes through)
vrk coax --quiet --times 3 -- my-command
```

### Compose patterns

```bash
# Retry an LLM prompt call — useful when the API returns transient errors
vrk coax --times 3 --backoff exp:1s --on 1 -- \
  vrk prompt "Summarise this document" < doc.txt

# Gate a pipeline: retry the expensive fetch, then process
vrk coax --times 5 --backoff 2s -- \
  curl -sf https://api.example.com/data > data.json
cat data.json | vrk prompt "Extract key facts"

# Wait for a background job to write a sentinel key, then proceed
vrk coax --times 20 --backoff 3s \
  --until 'vrk kv get job:status | grep -q done' \
  -- sh -c 'sleep 1'

# Retry with exit code passthrough — callers can still distinguish outcomes
vrk coax --times 3 -- my-command
echo "last exit: $?"
```

### Gotchas

- **`--` is required** to separate coax flags from the retried command: `vrk coax --times 2 -- cmd args`. Without a command, coax exits 2 with "missing command".
- **Commands run via `sh -c`.** The args after `--` are joined with spaces and passed to `sh -c "..."`. Shell builtins (`exit`, `test`, `[`) work, and you can use pipes and redirects inline. Pass complex commands as a single quoted argument to avoid double-wrapping: `vrk coax -- "cmd1 && cmd2"`.
- **Exit code passthrough.** On exhaustion, coax exits with the last command's actual exit code — not a normalised 1. `vrk coax --times 2 -- exit 42` exits 42. Callers that test `$? -ne 0` still work; callers checking a specific code should account for this.
- **`--on` with no match exits immediately.** If `--on 42` is set and the command exits 1, coax stops after the first attempt. The filter is strict — it does not fall back to "retry on any non-zero".
- **`--until` takes precedence over `--on`.** If both are set, only the condition is checked to decide whether to retry.
- **`--until` condition output is discarded.** The stdout and stderr of the `--until` command are suppressed — it is a side-effect check, not data.
- **Stdin is buffered once and re-piped.** All of stdin is read at startup. Streaming stdin sources (e.g., a slow HTTP body) will block until EOF before the first attempt starts. For streaming use cases, write stdin to a file first and redirect from the file.
- **`--quiet` suppresses only coax's own lines** — the `coax: attempt N failed` progress messages. The subprocess's own stdout and stderr are never suppressed.
- **Stdout is always empty on error.** Coax's own error messages go to stderr only.

---

## prompt — LLM Call for Pipelines

Sends a prompt to an LLM and prints the response. Defaults to Anthropic
(`claude-sonnet-4-5`). Reads from stdin or a positional argument.
Input: positional argument or stdin.

### Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--model` | `-m` | string | `claude-sonnet-4-5` | Model name; overridden by `VRK_DEFAULT_MODEL` env var |
| `--budget <N>` | — | int | 0 | Exit 1 before calling the API if prompt exceeds N tokens (0 = disabled) |
| `--fail` | `-f` | bool | false | Accepted; meaningful with `--schema` (exit 1 on mismatch) |
| `--json` | `-j` | bool | false | Emit response as a JSON envelope with metadata |
| `--schema <val>` | `-s` | string | "" | Inline JSON schema or file path; validates response keys and types |
| `--explain` | — | bool | false | Print equivalent curl command and exit 0; no API call |
| `--retry <N>` | — | int | 0 | Retry up to N times on schema mismatch with temperature escalation |

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success — response on stdout |
| 1 | Runtime error: no API key, HTTP error, budget exceeded, schema mismatch after all retries |
| 2 | Usage error: no input in interactive terminal (no positional arg, no `--explain`), unknown flag |

### Provider selection

| Condition | Provider |
|-----------|----------|
| Only `ANTHROPIC_API_KEY` set | Anthropic |
| Only `OPENAI_API_KEY` set | OpenAI |
| Both set, model starts with `gpt-`, `o1`, `o3`, or `o4` | OpenAI |
| Both set, any other model name | Anthropic |
| Neither set (not `--explain`) | Exit 1: "no API key found: set ANTHROPIC_API_KEY or OPENAI_API_KEY" |

### --json output shape

```json
{
  "response": "pong",
  "model": "claude-sonnet-4-5",
  "tokens_used": 12,
  "latency_ms": 340,
  "request_hash": "<sha256hex>"
}
```

`request_hash` is `sha256(model + "|" + temperature + "|" + prompt)` — stable cache key for `vrk kv`.

### Examples

```bash
# Basic call — stdin form
echo "Summarise this in one sentence." | vrk prompt

# Positional arg form — same result
vrk prompt "Summarise this in one sentence."

# Pick a different model
echo "hello" | vrk prompt --model gpt-4o

# Get full metadata envelope
echo "Reply with: pong" | vrk prompt --json

# Require JSON response matching a schema
echo "Return {name, age} for a fictional person." | vrk prompt --schema '{"name":"string","age":"number"}'

# Guard against large prompts
cat big_doc.txt | vrk prompt --budget 4000

# Dry-run: see what curl would be sent without making an API call
echo "hello" | vrk prompt --explain

# Override model for a session via env var
export VRK_DEFAULT_MODEL=claude-opus-4-5
echo "hello" | vrk prompt
```

### Compose patterns

```bash
# Cache expensive LLM response by request hash
RESULT=$(cat doc.txt | vrk prompt --json)
HASH=$(echo "$RESULT" | jq -r '.request_hash')
echo "$RESULT" | jq -r '.response' | vrk kv set "cache:$HASH"

# Budget gate before sending: count tokens first, then prompt
TOKENS=$(cat doc.txt | vrk tok)
if [ "$TOKENS" -le 4000 ]; then
  cat doc.txt | vrk prompt "Summarise this."
else
  echo "Document too large: $TOKENS tokens" >&2
  exit 1
fi

# Or use the built-in budget flag (same effect, one command)
cat doc.txt | vrk prompt --budget 4000 "Summarise this."

# Schema-validated extraction with retry
echo "Extract name and age from: Alice is 30." | \
  vrk prompt --schema '{"name":"string","age":"number"}' --retry 2

# Thread a request ID through a pipeline
REQ=$(vrk uuid)
cat payload.txt | vrk prompt "Classify this." | vrk kv set "result:$REQ"

# Retry transient API errors using coax
vrk coax --times 3 --backoff exp:1s --on 1 -- \
  sh -c 'echo "Summarise this." | vrk prompt'
```

### Gotchas

- **`--json` means metadata envelope, not "respond in JSON".** To request JSON from the LLM, use `--schema`. `--json` wraps the response in `{response, model, tokens_used, latency_ms, request_hash}`.
- **`--budget` is a hard gate.** It fires before the API call — even if no API key is set. There is no soft warning mode; use `vrk tok --budget N` if you want the token count without stopping the pipeline.
- **Temperature default is 0.** Responses are deterministic by default. `--retry` escalates temperature across attempts (0.0 → 0.1 → 0.2). Do not add a `--temperature` flag unless explicitly extending the tool.
- **API keys are never in output.** The key value is scrubbed from all error messages and `--explain` output before writing to stdout or stderr. `--explain` uses `$ANTHROPIC_API_KEY` / `$OPENAI_API_KEY` as shell variable references.
- **No conversation history.** Each call is stateless — there is no session context between invocations. For multi-turn conversations, build the context into the prompt text before calling.
- **`io.ReadAll` blocks until EOF.** The full prompt is read before the API call. For very large inputs, consider whether the model's context window can handle the token count.
- **`--schema` depth is top-level only.** Validation checks top-level keys and types (`string`, `number`, `boolean`, `array`, `object`). Nested schema structures are not validated.
- **`--schema` with OpenAI uses `response_format.json_schema`.** Validation is API-enforced. With Anthropic, the schema is injected as a system prompt and the response is validated post-call.
- **Stdout is always empty on error.** All error messages go to stderr. Stdout is empty on exit 1 and exit 2.
