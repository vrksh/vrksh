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
vrk prompt --system "Summarise this" < input.txt | vrk kv set "result:$ID"
```

### Compose patterns

```bash
# Generate a request ID and thread it through a pipeline
REQ=$(vrk uuid)
cat payload.json | vrk prompt --system "process this" | vrk kv set "response:$REQ"

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

## tok — Token Counter

Counts tokens in stdin using the cl100k_base tokenizer. Exact for GPT-4 family,
~95% accurate for Claude. With `--check N`, acts as a pipeline gate: passes input
through if within the token limit, exits 1 if over.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--check <N>` | - | Pass input through if within N tokens; exit 1 with empty stdout if over. Reads all stdin before deciding. |
| `--model <name>` | `-m` | Tokenizer model label (default: `cl100k_base`; only cl100k_base is currently implemented) |
| `--json` | `-j` | Emit output as `{"tokens": N, "model": "cl100k_base"}` (measurement mode) or JSON error (check mode) |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success - count printed (or JSON emitted); `--check` within limit |
| 1 | Over limit - `--check N` was set and token count exceeds N |
| 2 | Usage error - unknown flag; running interactively with no piped input |

### Examples

```bash
# Count tokens in a file
cat prompt.txt | vrk tok

# Count a string directly
vrk tok "hello world"
# -> 2

# JSON output
echo "hello world" | vrk tok --json
# -> {"tokens":2,"model":"cl100k_base"}

# Gate: pass through if within 8000 tokens
echo "hello world" | vrk tok --check 8000
# -> hello world  (exits 0, input passes through unchanged)

# Gate: exit 1 if over limit
cat big_context.txt | vrk tok --check 100
# -> (nothing on stdout, exits 1)

# Count with explicit model label
echo "hello world" | vrk tok --model cl100k_base
# -> 2
```

### Compose patterns

```bash
# Gate in a pipeline - input passes through to prompt if within limit
cat big_context.txt | vrk tok --check 8000 | vrk prompt --system "answer: $QUESTION"
# (within 8000: full pipeline runs, doc reaches prompt unchanged)
# (over 8000: pipeline stops at tok, nothing reaches prompt)

# Count tokens and store result
TOKENS=$(cat prompt.txt | vrk tok)
echo "Sending $TOKENS tokens to the model"

# JSON output for structured logging
cat prompt.txt | vrk tok --json | vrk kv set "last_prompt_tokens"

# CI size gate - fail build if generated prompt is too large
cat generated_prompt.txt | vrk tok --check 100000 > /dev/null || { echo "Prompt too large"; exit 1; }
```

### Gotchas

- **`--check` buffers all stdin** before deciding whether to pass through - O(input size) memory. For inputs >100MB, sample first with `vrk sip`.
- **`--json` + `--check` within limit** passes raw input through, not a JSON envelope. `--json` only affects the error path.
- **`--check 0`** exits 1 for any non-empty input. Valid but rarely useful.
- **Positional arg passthrough** joins with space, no trailing newline. Stdin passthrough preserves raw bytes exactly including trailing newlines.
- **`--model` is a label, not a tokenizer switch.** Passing `--model claude-3-opus` or any other value still uses cl100k_base internally - the flag exists for forward compatibility and `--json` output labelling. Only `cl100k_base` is currently implemented; any other string is accepted without error but does not change which tokenizer runs.
- **cl100k_base is approximate for Claude (~95% accurate).** The exact Claude tokenizer is not publicly available. Set `--check` at 90% of the model's actual context limit to absorb the error margin.
- **Empty pipe is 0 tokens, not an error.** `cat /dev/null | vrk tok` exits 0 and prints `0`. Only running `vrk tok` interactively in a terminal (no pipe) exits 2.

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
vrk prompt --system "Summarise this" < input.txt | vrk kv set "response:$REQ"
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
  vrk prompt --system "process batch $i" < batch_$i.txt | vrk kv set --ns run:$RUN "result:$i" &
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
  vrk prompt --system "Summarise this document" < doc.txt

# Gate a pipeline: retry the expensive fetch, then process
vrk coax --times 5 --backoff 2s -- \
  curl -sf https://api.example.com/data > data.json
cat data.json | vrk prompt --system "Extract key facts"

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
(`claude-sonnet-4-6`). Reads from stdin or a positional argument.
Input: positional argument or stdin.

### Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--model` | `-m` | string | `claude-sonnet-4-6` | Model name; overridden by `VRK_DEFAULT_MODEL` env var |
| `--endpoint <url>` | — | string | `""` | OpenAI-compatible API base URL; overrides provider detection |
| `--field <path>` | — | string | `""` | Dot-path field in each JSONL line to use as prompt text |
| `--budget <N>` | — | int | 0 | Exit 1 before calling the API if prompt exceeds N tokens (0 = disabled) |
| `--fail` | `-f` | bool | false | Accepted; meaningful with `--schema` (exit 1 on mismatch) |
| `--json` | `-j` | bool | false | Emit response as a JSON envelope with metadata |
| `--schema <val>` | `-s` | string | "" | Inline JSON schema or file path; validates response keys and types |
| `--explain` | — | bool | false | Print equivalent curl command and exit 0; no API call |
| `--retry <N>` | — | int | 0 | Retry up to N times on schema mismatch with temperature escalation |
| `--system <val>` | — | string | `""` | System prompt text, or `@path` to read from file |

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success - response on stdout |
| 1 | Runtime error: no API key, HTTP error, budget exceeded, schema mismatch after all retries, invalid JSONL, field not found |
| 2 | Usage error: no input in interactive terminal, unknown flag, `--field` with `--explain`, `--field` with TTY stdin |

### Provider resolution order

| Priority | Condition | Behaviour |
|----------|-----------|-----------|
| 1 | `--endpoint` flag set | OpenAI-compatible format to that URL |
| 2 | `VRK_LLM_URL` env var set | OpenAI-compatible format to that URL |
| 3 | Only `ANTHROPIC_API_KEY` set | Anthropic API |
| 4 | Only `OPENAI_API_KEY` set | OpenAI API |
| 5 | Both keys set, model starts with `gpt-`/`o1`/`o3`/`o4` | OpenAI API |
| 6 | Both keys set, any other model | Anthropic API |
| — | No key and no endpoint (not `--explain`) | Exit 1: "no API key found" |

When `--endpoint` or `VRK_LLM_URL` is set, `--model` is required (exit 2 if absent and `VRK_DEFAULT_MODEL` is not set). `OPENAI_API_KEY` and `ANTHROPIC_API_KEY` are ignored for custom endpoint requests.

### --json output shape

```json
{
  "response": "pong",
  "model": "claude-sonnet-4-6",
  "prompt_tokens": 8,
  "response_tokens": 4,
  "elapsed_ms": 340
}
```

With `--field`, input record fields are preserved and response fields are merged in:

```json
{"index": 0, "text": "The vendor shall...", "tokens": 30, "response": "Vendor must deliver within 30 days.", "model": "claude-sonnet-4-6", "prompt_tokens": 38, "response_tokens": 11, "elapsed_ms": 412}
```

### Examples

```bash
# Basic call — stdin form
echo "Summarise this in one sentence." | vrk prompt

# System prompt form — instruction separate from content
echo "$CONTENT" | vrk prompt --system "Summarise this in one sentence."

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
export VRK_DEFAULT_MODEL=claude-opus-4-6
echo "hello" | vrk prompt

# Custom endpoint (Ollama, LM Studio, vLLM, LocalAI, Jan)
echo "hello" | vrk prompt --endpoint http://localhost:11434/v1 --model llama3.2
VRK_LLM_URL=http://localhost:11434/v1 vrk prompt --model llama3.2

# Process JSONL chunks with --field (one API call per record)
cat docs/*.md | vrk chunk --size 4000 | vrk prompt --field text --system 'Summarize' --json

# Batch with rate limiting and structured output
cat docs/*.md \
  | vrk chunk --size 4000 \
  | vrk throttle --rate 60/m \
  | vrk prompt --field text --system 'Extract key claims' --schema '{"claims":"array"}' --json
```

### Custom endpoints (--endpoint / VRK_LLM_URL)

Works with any OpenAI-compatible server: Ollama, LM Studio, vLLM, LocalAI, Jan.

`--endpoint` always uses OpenAI chat completions format, never Anthropic format.
`--endpoint` takes precedence over `ANTHROPIC_API_KEY` and `OPENAI_API_KEY`.
`--model` is required when using `--endpoint` (exit 2 if absent and `VRK_DEFAULT_MODEL` not set).

**Auth:** set `VRK_LLM_KEY` if the server requires a Bearer token. Omit it for local models that need no auth. `OPENAI_API_KEY` and `ANTHROPIC_API_KEY` are never used for custom endpoints.

**Gotcha:** if the server returns a non-standard response shape, `prompt` exits 1 with the raw response body in the error. Use `--explain` to inspect the exact request being sent.

**Manual verification (requires Ollama running locally):**
```bash
VRK_LLM_URL=http://localhost:11434/v1 echo 'Reply with: pong' | vrk prompt --model llama3.2
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
  cat doc.txt | vrk prompt --system "Summarise this."
else
  echo "Document too large: $TOKENS tokens" >&2
  exit 1
fi

# Or use the built-in budget flag (same effect, one command)
cat doc.txt | vrk prompt --budget 4000 --system "Summarise this."

# Schema-validated extraction with retry
echo "Extract name and age from: Alice is 30." | \
  vrk prompt --schema '{"name":"string","age":"number"}' --retry 2

# Thread a request ID through a pipeline
REQ=$(vrk uuid)
cat payload.txt | vrk prompt --system "Classify this." | vrk kv set "result:$REQ"

# Retry transient API errors using coax
vrk coax --times 3 --backoff exp:1s --on 1 -- \
  sh -c 'echo "Summarise this." | vrk prompt'
```

### Gotchas

- **`--json` means metadata envelope, not "respond in JSON".** To request JSON from the LLM, use `--schema`. `--json` wraps the response in `{response, model, prompt_tokens, response_tokens, elapsed_ms}`.
- **`--field` and `--explain` are mutually exclusive.** `--explain` is for inspecting a single call; it does not make sense for a stream. Exit 2 if both are set.
- **`--field` exits on first error.** If line 5 has invalid JSON, lines 1-4 are already written to stdout. Callers that need atomicity should buffer output and check exit code.
- **`--field` overwrites `response` in input records.** If the input JSONL has a field named `response`, it is overwritten and a warning is emitted to stderr.
- **`--budget` is a hard gate.** It fires before the API call - even if no API key is set. There is no soft warning mode; use `vrk tok --check N` to gate the pipeline upstream.
- **Temperature default is 0.** Responses are deterministic by default. `--retry` escalates temperature across attempts (0.0 → 0.1 → 0.2). Do not add a `--temperature` flag unless explicitly extending the tool.
- **API keys are never in output.** The key value is scrubbed from all error messages and `--explain` output before writing to stdout or stderr. `--explain` uses `$ANTHROPIC_API_KEY` / `$OPENAI_API_KEY` as shell variable references.
- **No conversation history.** Each call is stateless — there is no session context between invocations. For multi-turn conversations, build the context into the prompt text before calling.
- **`io.ReadAll` blocks until EOF.** The full prompt is read before the API call. For very large inputs, consider whether the model's context window can handle the token count.
- **`--schema` depth is top-level only.** Validation checks top-level keys and types (`string`, `number`, `boolean`, `array`, `object`). Nested schema structures are not validated.
- **`--schema` with OpenAI uses `response_format.json_schema`.** Validation is API-enforced. With Anthropic, the schema is injected as a system prompt and the response is validated post-call.
- **Stdout is always empty on error.** All error messages go to stderr. Stdout is empty on exit 1 and exit 2.
- **`--system @file` path resolution** — the path is relative to the working directory where `vrk` is invoked, not to the script containing the command. In CI, always use absolute paths or paths relative to the repo root.

---

## chunk — Token-Aware Text Splitter

Splits text from stdin into chunks, each within a token budget. Emits JSONL.
Uses cl100k_base tokenization (exact for GPT-4 family, ~95% accurate for Claude).
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--size N` | — | Max tokens per chunk (required, must be >= 1) |
| `--overlap N` | — | Token overlap between adjacent chunks (default 0; must be < --size) |
| `--by <mode>` | — | Chunking strategy; only `paragraph` is currently supported |

### Output format

One JSONL record per chunk:

```json
{"index":0,"text":"...","tokens":998}
{"index":1,"text":"...","tokens":1000}
{"index":2,"text":"...","tokens":312}
```

- `index`: 0-based, sequential across all emitted records
- `text`: decoded token window — may split mid-word at token boundaries (see Gotchas)
- `tokens`: exact cl100k_base token count for the emitted text; never exceeds `--size`

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty input — no records emitted) |
| 1 | Runtime error — I/O error reading stdin, tokenizer failure |
| 2 | Usage error — `--size` absent or < 1, `--overlap` >= `--size`, unknown `--by` mode, unknown flag |

### Splitting modes

**Default (no `--by`):** Sliding window over token IDs. Step = `size - overlap`. Each window is decoded back to text. The invariant `tokens <= size` is structurally guaranteed.

**`--by paragraph`:** Splits text at double-newlines (`\n\n`), then greedily packs paragraphs into chunks. A paragraph that exceeds `--size` tokens falls back to token-level splitting. Overlap is applied by prepending the last `--overlap` token IDs from the previous chunk at the start of the next.

### Examples

```bash
# Split a file into 1000-token chunks
cat doc.txt | vrk chunk --size 1000

# Overlapping chunks — useful for RAG to avoid losing context at boundaries
cat doc.txt | vrk chunk --size 1000 --overlap 100

# Respect paragraph boundaries
cat doc.txt | vrk chunk --size 1000 --by paragraph

# Positional arg form — identical to stdin
vrk chunk --size 1000 "some text to split"

# Empty input → exit 0, no output
printf '' | vrk chunk --size 1000

# Count chunks produced
cat doc.txt | vrk chunk --size 500 | wc -l

# Extract just the text fields
cat doc.txt | vrk chunk --size 500 | jq -r '.text'

# Verify invariant holds
cat doc.txt | vrk chunk --size 500 | jq -e '[.tokens] | max <= 500'
```

### Compose patterns

```bash
# Embed each chunk separately — classic RAG pipeline
cat corpus.txt | vrk chunk --size 512 --overlap 64 | while read -r line; do
  text=$(echo "$line" | jq -r '.text')
  idx=$(echo "$line" | jq -r '.index')
  echo "$text" | vrk prompt --system "Embed this passage." | vrk kv set "embed:$idx"
done

# Budget-safe chunked summarisation
cat long_doc.txt | vrk chunk --size 3000 | jq -r '.text' | while read -r chunk; do
  echo "$chunk" | vrk tok --check 3000 | vrk prompt --system "Summarise."
done

# Store all chunks in kv, keyed by document ID and chunk index
DOC_ID=$(vrk uuid)
cat doc.txt | vrk chunk --size 1000 | while read -r line; do
  idx=$(echo "$line" | jq -r '.index')
  text=$(echo "$line" | jq -r '.text')
  vrk kv set "doc:${DOC_ID}:chunk:${idx}" "$text"
done

# Paragraph-aware chunking with overlap for better boundary handling
cat article.txt | vrk chunk --size 800 --overlap 80 --by paragraph | jq '.text'
```

### Gotchas

- **`--size` is required.** There is no default. Omitting it exits 2: `chunk: --size is required`.
- **Chunks may split mid-word.** The default mode works at the token-ID level. Tiktoken token boundaries do not always align with word boundaries, so a chunk boundary can fall in the middle of a word. The decoded text is emitted as-is — boundaries are never adjusted, because adjusting them would change token counts and risk breaking the size invariant.
- **`--by paragraph` uses double newline as the separator.** A single newline is not a paragraph break. If your text uses single newlines as paragraph separators, pre-process it with `sed` or `awk` before piping to `chunk`.
- **`--by paragraph` oversized paragraphs fall back to token-level split.** A paragraph that exceeds `--size` tokens is split at token boundaries, not at sentence or word boundaries. The mid-word split caveat applies.
- **`--overlap` is in tokens, matching `--size` units.** Overlap is not a percentage.
- **Empty input exits 0 with no output.** `printf '' | vrk chunk --size 1000` emits nothing and exits 0. This is intentional — empty input is not an error.
- **`tokens` field is exact.** It reflects the actual token count of the emitted `text`, not an approximation. Use it for downstream budget checks.
- **cl100k_base is approximate for Claude (~95% accurate).** Set `--size` at 90% of the model's actual context limit to absorb the error margin.
- **Stdout is always empty on error.** Errors go to stderr only.

---

## grab — URL Fetcher

Fetches a URL and returns clean markdown (default), plain text, or raw HTML.
Applies Readability-style content extraction — strips navigation, ads, boilerplate.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--text` | `-t` | Plain prose output, no markdown syntax |
| `--raw` | - | Raw HTML, no processing or extraction |
| `--json` | `-j` | Emit a JSON envelope with metadata |
| `--quiet` | `-q` | Suppress stderr output |
| `--max-size` | - | Max response body size in bytes (default 10MB) |
| `--allow-internal` | - | Allow requests to private/loopback/link-local addresses |

`--text`, `--raw`, and `--json` are mutually exclusive. Combining any two exits 2.
Requests to internal network addresses (127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, link-local) are blocked by default. Pass `--allow-internal` to allow them.

### --json output shape

```json
{"url":"https://example.com","title":"Example Domain","content":"# Example Domain\n\n...","fetched_at":1740000000,"status":200,"token_estimate":180}
```

- `url`: final URL after any redirects
- `title`: text content of `<title>` element (empty string if absent)
- `content`: extracted content in the active mode (markdown by default)
- `fetched_at`: Unix timestamp (integer seconds) of when the response was received
- `status`: HTTP status code (always 200 on exit 0)
- `token_estimate`: cl100k_base token count of `content` (~95% accurate for Claude)

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success — output on stdout |
| 1 | Fetch failed: DNS error, connection refused, timeout |
| 1 | HTTP status >= 400 |
| 1 | More than 5 redirect hops |
| 1 | Response body exceeded `--max-size` limit |
| 1 | Request to internal address blocked (without `--allow-internal`) |
| 2 | No URL provided (interactive terminal or empty stdin) |
| 2 | Invalid URL format (no scheme, non-http/https scheme, unparseable) |
| 2 | Unknown flag |
| 2 | Mutually exclusive flags combined |

### Examples

```bash
# Fetch a page as clean markdown (default)
vrk grab https://example.com

# Plain text — no markdown syntax
vrk grab https://example.com --text

# Raw HTML — no processing
vrk grab https://example.com --raw

# JSON envelope with metadata
vrk grab https://example.com --json

# Pipe a URL via stdin
echo https://example.com | vrk grab

# Count tokens in fetched content
vrk grab https://example.com | vrk tok

# Chunk fetched content for RAG
vrk grab https://example.com | vrk chunk --size 1000
```

### Compose patterns

```bash
# Fetch, count tokens, guard before sending to LLM
vrk grab https://example.com | vrk tok --check 8000 | \
  vrk grab https://example.com | vrk prompt --system "Summarise this page."

# Store fetched content in kv keyed by URL
PAGE=$(vrk grab https://example.com --json)
vrk kv set "page:$(echo "$PAGE" | jq -r '.url')" "$(echo "$PAGE" | jq -r '.content')"

# Fetch multiple URLs, extract JSON metadata for each
cat urls.txt | xargs -I{} vrk grab {} --json | jq '.title'

# RAG pipeline: fetch → chunk → store chunks
DOC_ID=$(vrk uuid)
vrk grab https://example.com | vrk chunk --size 512 --overlap 64 | while read -r chunk; do
  idx=$(echo "$chunk" | jq -r '.index')
  text=$(echo "$chunk" | jq -r '.text')
  vrk kv set "doc:${DOC_ID}:chunk:${idx}" "$text"
done

# Retry a flaky fetch
vrk coax --times 3 --backoff exp:1s -- vrk grab https://example.com
```

### Gotchas

- **Best-effort extraction, not full Readability.** `grab` extracts content using a simple scoring heuristic. Output quality varies by page structure: well-formed articles with semantic HTML (`<main>`, `<article>`, `<p>`) extract cleanly. Complex layouts (SPAs, dashboards, heavily nested tables) may include noise or miss content. For reliable extraction, prefer pages with semantic HTML.
- **JavaScript is not executed.** `grab` makes a static HTTP request. Pages that render content client-side via JavaScript will return the pre-render HTML skeleton, not the final page content.
- **Invalid URL is a usage error (exit 2), not a runtime error (exit 1).** A URL with no scheme (`example.com`) or a non-http/https scheme (`ftp://`) exits 2 — the caller gave bad input. A valid URL that fails to fetch (DNS, timeout, 404) exits 1.
- **Non-HTML responses pass through unchanged.** If the server returns `Content-Type: application/json` or any non-`text/html` type, the raw body is emitted to stdout without extraction. `--text` is a no-op for non-HTML. `--json` still wraps the raw body in the envelope.
- **Max 5 redirect hops.** Chains longer than 5 redirects exit 1 with "too many redirects (> 5)". Cookies are never stored between redirects or invocations.
- **`token_estimate` uses cl100k_base (~95% accurate for Claude).** The estimate reflects the extracted `content` field, not the raw HTML. Set downstream budgets at 90% of the actual model limit to absorb the error margin.
- **`--text` output is whitespace-normalised.** `grab --text` runs the HTML extractor then `vrk plain`'s markdown stripper. Consecutive blank lines are collapsed to one, and multiple spaces on a line are collapsed to one. The prose content is unchanged, but whitespace structure may differ from the raw page.
- **Stdout is always empty on error.** All error messages go to stderr. Stdout is empty on exit 1 and exit 2.
- **Internal addresses are blocked by default.** Requests to 127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 169.254.0.0/16, ::1, and link-local IPv6 exit 1 unless `--allow-internal` is passed. This includes addresses reached via redirects - a public URL that redirects to localhost is also blocked.
- **Response size is capped at 10MB by default.** Use `--max-size` to raise or lower the limit. Responses that exceed the limit exit 1.

---

## plain — Markdown Stripper

Strips markdown formatting from stdin or a positional argument. Preserves all content — only syntax is removed.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | `-j` | JSON envelope: `{"text":"…","input_bytes":N,"output_bytes":M}` |

No `--quiet`. `plain` produces no informational stderr in normal operation.

### --json output shape

```json
{"text":"hello world","input_bytes":20,"output_bytes":11}
```

- `text`: stripped plain text
- `input_bytes`: raw bytes read from stdin (including any trailing newline from `echo`)
- `output_bytes`: byte length of the stripped text (not including the trailing newline added on stdout)

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success (including empty input) |
| 1 | I/O error reading stdin |
| 2 | No input in interactive terminal |
| 2 | Unknown flag |

### Examples

```bash
# Strip formatting from a README
vrk plain < README.md

# Strip bold/italic/links from inline markdown
echo '**bold** _italic_ [link](https://example.com)' | vrk plain
# → bold italic link

# JSON envelope with byte counts
echo '**hello** _world_' | vrk plain --json
# → {"text":"hello world","input_bytes":18,"output_bytes":11}

# Empty input exits 0 with no output
printf '' | vrk plain

# Positional arg form — identical to stdin
vrk plain '**bold** _italic_'
# → bold italic
```

### Compose patterns

```bash
# Strip markdown before sending to a model that expects plain text
vrk plain < doc.md | vrk prompt --system "summarise this"

# Check token count of stripped prose
vrk plain < README.md | vrk tok

# Strip markdown from a fetched page
vrk grab https://example.com | vrk plain

# Fetch plain text in one step (HTML extraction + markdown strip)
vrk grab https://example.com --text
```

### Gotchas

- **`plain` uses goldmark's AST** — it handles nested emphasis, reference-style links, and fenced code blocks correctly. Character-level regex strippers do not.
- **Raw HTML embedded in markdown is dropped silently.** Block-level `<div>` and inline `<span>` tags and their content are not extracted. For HTML-heavy input, use `grab --text` instead (which runs the HTML extractor first, then `StripMarkdown`).
- **Output whitespace may differ from input.** Consecutive blank lines are collapsed to one. Leading and trailing whitespace is trimmed.

---

## links — Hyperlink Extractor

Extracts all hyperlinks from markdown, HTML, or plain text as JSONL.
One record per link: `{"text":"...","url":"...","line":N}`.
Input: positional argument or stdin. Empty input exits 0 with no output.

### Record shape

```
{"text":"Homebrew","url":"https://brew.sh","line":1}
```

`text` is the anchor text (Markdown or HTML) or the URL itself for bare URLs.
`line` is the 1-based line number in the input.

### Supported input formats (auto-detected)

| Format | Example | Notes |
|--------|---------|-------|
| Markdown inline | `[text](url)` | Standard inline link |
| Markdown reference | `[text][label]` + `[label]: url` | Resolved at usage site; definition line not emitted |
| HTML anchor | `<a href="url">text</a>` | Case-insensitive; inner tags stripped from text |
| Bare URL | `https://...` or `http://...` | text == url; only when not inside another pattern |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--bare` | `-b` | Output URLs only, one per line |
| `--json` | `-j` | Append `{"_vrk":"links","count":N}` after all records |

### --json output shapes

| Flags | Shape |
|-------|-------|
| `--json` alone | link records + `{"_vrk":"links","count":N}` |
| `--bare --json` | bare URL lines + `{"_vrk":"links","count":N}` |
| Any error + `--json` | `{"error":"msg","code":N}` on stdout; stderr empty |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — links found or not, including empty input |
| 1 | I/O error reading stdin |
| 2 | Usage error — interactive terminal with no stdin, unknown flag |

### Examples

```bash
# Extract links from a markdown file
cat README.md | vrk links

# Get URLs only — one per line
cat README.md | vrk links --bare

# With metadata count
cat README.md | vrk links --json | tail -1

# Extract from fetched HTML
vrk grab https://example.com | vrk links

# Spider links: fetch a page and extract all URLs
vrk grab https://example.com | vrk links --bare | while read url; do echo "$url"; done

# Count links in a document
cat README.md | vrk links --json | tail -1 | jq '.count'

# Positional arg form — identical to stdin
vrk links '[Homebrew](https://brew.sh)'
# → {"text":"Homebrew","url":"https://brew.sh","line":1}
```

### Compose patterns

```bash
# Audit dead links — extract URLs then check each one
cat docs.md | vrk links --bare | while read url; do
  vrk grab "$url" > /dev/null && echo "ok $url" || echo "dead $url"
done

# Seed a crawl queue with links from multiple pages
for url in "${PAGES[@]}"; do
  vrk grab "$url" | vrk links --bare
done | sort -u > crawl-queue.txt

# Extract links and store in kv for later processing
cat README.md | vrk links | while read -r rec; do
  url=$(printf '%s' "$rec" | jq -r '.url')
  vrk kv set "link:$url" "$rec"
done

# Combine with plain for clean markdown → links pipeline
vrk grab https://example.com | vrk plain | vrk links
```

### Gotchas

- **Relative URLs are emitted as-is** — `href="/about"` produces `{"url":"/about",...}`. The caller knows the base URL; `links` does not resolve relative hrefs.
- **Markdown ref definitions are not emitted as links** — `[label]: url` lines are collected internally but only appear in output when a `[text][label]` usage references them. The emitted `line` is the line of the usage, not the definition.
- **Multi-line `<a>` tags are not supported** — the tool processes line by line. An `<a href="...">` that spans multiple lines will not be matched.
- **No deduplication** — the same URL appearing twice produces two records. Pipe through `sort -u` or `jq -r '.url'` if you need unique URLs.
- **Empty stdin exits 0 with empty output** — this is not an error. Only an interactive terminal (no pipe) exits 2.

---

## validate — JSONL Schema Validator

Validates JSONL records against a simplified type schema. Valid lines pass through
to stdout unchanged. Invalid lines are warned to stderr and skipped (or cause exit 1
with `--strict`). Input: stdin only (no positional argument — schema comes from `--schema`).

### Schema format

`{"key":"type"}` map. Supported types: `string | number | boolean | array | object`.
Schema keys are **required fields** — a record missing a schema key is invalid.
Extra keys in the record that are not in the schema are **ignored** (subset check).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--schema <spec>` | `-s` | Inline JSON schema or file path (required) |
| `--strict` | — | Exit 1 on first invalid line (no shorthand — `-s` is taken) |
| `--fix` | — | Attempt to repair invalid lines via `vrk prompt` before re-validating |
| `--json` | `-j` | Append metadata record `{"_vrk":"validate","total":N,"passed":N,"failed":N}` at end |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | All lines valid, OR invalid lines found but `--strict` not set |
| 1 | `--strict` and at least one invalid line; or I/O error (with `--json`: error record to stdout) |
| 2 | `--schema` missing or invalid, unknown schema type, unknown flag |

### Examples

```bash
# Validate a single record
echo '{"name":"alice","age":30}' | vrk validate --schema '{"name":"string","age":"number"}'
# → {"name":"alice","age":30}  (stdout, exit 0)

# Invalid record — warned to stderr, nothing on stdout
echo '{"name":"alice","age":"wrong"}' | vrk validate --schema '{"name":"string","age":"number"}'
# → (empty stdout, stderr: "warning: validation failed: age expected number, got string", exit 0)

# --strict: exit 1 on first invalid
cat records.jsonl | vrk validate --schema '{"name":"string"}' --strict

# File-based schema
vrk validate --schema ./schema.json < records.jsonl

# --json: append metadata record at end
cat records.jsonl | vrk validate --schema '{"name":"string"}' --json
# → <valid records>
#   {"_vrk":"validate","total":10,"passed":9,"failed":1}

# Pipeline: validate → token count
echo '{"name":"alice","age":30}' | vrk validate --schema '{"name":"string","age":"number"}' | vrk tok

# Pipeline: validate then store only valid records in kv
cat records.jsonl | vrk validate --schema '{"name":"string"}' | while read -r rec; do
  vrk kv set "record:$(echo "$rec" | jq -r '.name')" "$rec"
done
```

### Gotchas

- **`--strict` has no shorthand** — `-s` is reserved for `--schema`. Use `--strict` in full.
- **`--fix` requires an API key** — it shells out to `vrk prompt` to repair invalid lines. If no API key is configured, `--fix` degrades silently: the line stays invalid and a warning is emitted to stderr. It never exits 2 for a missing key — that would be a pipeline footgun.
- **`--json` on empty input still emits the metadata record** — `{"_vrk":"validate","total":0,"passed":0,"failed":0}`. This is consistent with other `--json` tools.
- **`--json` + `--fix`: repaired lines count as `passed`** — there is no separate `repaired` counter.
- **Schema is a subset check** — extra keys in a record are not an error. Only keys declared in the schema are validated.
- **Large integers are safe** — records are decoded with `UseNumber()` so integers above 2^53 retain full precision.
- **Empty lines in the stream are skipped** — they do not count toward `total`.

---

## mask — Secret Redactor

Redacts secrets from stdin using entropy-based detection and built-in pattern
matching. Passes text through with secrets replaced by `[REDACTED]`. Streaming
line-by-line — safe on large inputs. Best-effort — not a security boundary.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--pattern <regex>` | — | Additional pattern; repeatable. Entire match → `[REDACTED]` |
| `--entropy <float>` | — | Shannon entropy threshold (default: 4.0; lower = more aggressive) |
| `--json` | `-j` | Append metadata record after text output |

### Built-in patterns

Applied before entropy scanning. The key prefix is preserved; the value is replaced.

| Name | Matches |
|------|---------|
| `bearer` | `Bearer <token>` (case-insensitive) |
| `password` | `password=<value>` or `password:<value>` |
| `secret` | `secret=<value>` or `secret:<value>` |
| `api_key` | `api_key=<value>`, `api-key=<value>`, etc. |
| `token` | `token=<value>` or `token:<value>` |

### Entropy scanning

Tokens shorter than 8 characters are never entropy-checked. Tokens that already
contain `[REDACTED]` (from pattern substitution) are skipped. Shannon entropy is
calculated per whitespace-delimited token: `H = -sum(p * log2(p))`.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — filter ran, text (redacted or not) written to stdout |
| 1 | I/O error with `--json` active — `{"error":"...","code":1}` written to stdout |
| 2 | Usage error — interactive terminal, unknown flag, invalid `--pattern` regex |

### `--json` output shape

Text lines are written first (streaming); the metadata record is appended after EOF:

```json
{"_vrk":"mask","lines":N,"redacted":N,"patterns_matched":["bearer","entropy"]}
```

- `lines`: total lines processed
- `redacted`: lines where ≥ 1 substitution was made
- `patterns_matched`: deduplicated list — built-in names (`"bearer"`, `"entropy"`, …) or literal regex strings for `--pattern` values

### `--json` error shape

```json
{"error":"<message>","code":1}
```

Written to stdout; stderr is empty. `Run()` returns 1. The envelope `code` and
`$?` always match — an agent can safely check either.

### Examples

```bash
# Strip Bearer token from an Authorization header
echo 'Authorization: Bearer sk-ant-abc123def456' | vrk mask
# → Authorization: Bearer [REDACTED]

# Strip a password field
echo 'password=hunter2' | vrk mask
# → password=[REDACTED]

# Clean text passes through unchanged
echo 'no secrets here' | vrk mask
# → no secrets here

# Custom pattern for a known key prefix
echo 'key: sk-ant-AAAA' | vrk mask --pattern 'sk-ant-[A-Za-z0-9]+'
# → key: [REDACTED]

# Lower entropy threshold to catch repetitive secrets
echo 'sk-ant-AAABBBCCC111222333444555' | vrk mask --entropy 3.0
# → [REDACTED]

# Multi-line: only secret lines are changed
printf 'line1\nBearer abc123xyz\nline3\n' | vrk mask
# → line1
#    Bearer [REDACTED]
#    line3

# --json: metadata record appended after text output
echo 'token: sk-abc123XYZ' | vrk mask --json
# → token: [REDACTED]
#   {"_vrk":"mask","lines":1,"redacted":1,"patterns_matched":["token"]}

# Pipeline: scrub LLM output before storing
vrk prompt --system "summarise this" < doc.txt | vrk mask | vrk kv set summary
```

### Gotchas

- **Best-effort only — not a security boundary.** Do not rely on `mask` as a compliance or audit control. Use it to reduce accidental exposure in logs and pipelines.
- **UUIDs and SHA-256 hashes are high-entropy — expect false positives.** A UUID like `550e8400-e29b-41d4-a716-446655440000` (36 chars, entropy ≈ 3.8) may or may not be redacted depending on the threshold. SHA-256 hex strings will almost always be redacted at the default 4.0 threshold.
- **Short passwords are below the 8-char token floor — expect false negatives.** `hunter2` (7 chars) is never entropy-checked. Pattern-based detection (`password=hunter2`) still catches it, but a bare short password with no keyword prefix will not be redacted.
- **Token floor is 8 characters.** Tokens shorter than 8 chars are skipped by entropy analysis regardless of `--entropy` setting.
- **`--pattern` replaces the entire match.** Unlike built-in patterns (which preserve the key prefix via a capture group), `--pattern` replaces the whole regex match with `[REDACTED]`. To preserve a prefix, use a lookahead or adjust the pattern to match only the value portion.
- **`patterns_matched` uses literal regex strings for `--pattern` values.** The field `"sk-ant-[A-Za-z0-9]+"` in `patterns_matched` means that exact regex fired, not a named category.
- **Ordering in `patterns_matched`:** built-ins in declaration order (bearer → password → secret → api_key → token), then `"entropy"`, then custom `--pattern` values in argument order.

---

## emit — Structured Logger

Wraps stdin lines as JSONL log records with timestamps and levels.
Input: positional argument or stdin (line-by-line streaming).
emit is JSONL-native — every output line is already a JSON object. No `--json` flag.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--level <level>` | `-l` | `info` | Log level for all lines: debug, info, warn, error |
| `--tag <tag>` | — | — | Add `"tag"` field to every record |
| `--msg <msg>` | — | — | Override message; stdin treated as JSON to merge extra fields |
| `--parse-level` | — | — | Auto-detect level from line prefix; strip prefix from msg |

### Output shape

```
{"ts":"2026-03-01T10:00:00.123Z","level":"info","msg":"<text>"}
{"ts":"...","level":"error","tag":"agent-run","msg":"<text>"}
{"ts":"...","level":"error","msg":"Job failed","job_id":"abc"}
```

Field order is deterministic: `ts` → `level` → `tag` (if set) → `msg` → merged extra fields (alphabetical).
Timestamps are UTC RFC3339 with exactly 3 millisecond digits, always ending in `Z`.

### --parse-level detection

Recognised prefixes (case-insensitive): `ERROR`, `WARN`, `WARNING`, `INFO`, `DEBUG`.
The prefix must be at a word boundary: followed by `:`, ` `, `\t`, or end of line.
After matching: prefix + optional colon + leading whitespace are stripped from the message.
Unknown prefix → falls back to the `--level` value (default `info`).

```
"ERROR: disk full"   → level=error, msg="disk full"
"WARN low memory"    → level=warn,  msg="low memory"
"DEBUGGER: verbose"  → level=info,  msg="DEBUGGER: verbose"  (no match — not a word boundary)
"[ERROR] crash"      → level=info,  msg="[ERROR] crash"      (no match — bracket prefix)
```

### --msg + JSON stdin merge

When `--msg` is set, each stdin line is attempted as a JSON object. If valid, its fields are
merged into the record after `msg` (alphabetically sorted). Non-JSON lines are silently ignored
(no extra fields added — the record still emits with the `--msg` value). Core fields
(`ts`, `level`, `tag`, `msg`) in stdin JSON are always suppressed; the flag-provided values win.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | I/O error (scanner error, stdout write error) |
| 2 | Usage error — interactive stdin, unknown flag, invalid `--level` value |

### Examples

```bash
# Wrap script output as structured logs
./deploy.sh 2>&1 | vrk emit --tag deploy

# Wrap with error level
echo 'Job failed' | vrk emit --level error

# Add a tag to every record
echo 'Starting job' | vrk emit --level info --tag agent-run

# Merge JSON context into a record
echo '{"job_id":"abc"}' | vrk emit --level error --msg "Job failed"
# → {"ts":"...","level":"error","msg":"Job failed","job_id":"abc"}

# Auto-detect level from log prefix
printf 'ERROR: disk full\nWARN: low memory\nall good\n' | vrk emit --parse-level

# Single message as positional arg
vrk emit 'Starting job'

# Pipeline: emit LLM output as structured log, then store
vrk prompt --system "run analysis" < data.txt | vrk emit --parse-level --tag llm | vrk kv set last-run
```

### Gotchas

- **JSONL-native — no `--json` flag.** Every output line is already a JSON record. I/O errors go to stderr as plain text (exit 1).
- **Empty lines are silently skipped** — no record is emitted, no error. `echo '' | vrk emit` exits 0 with no output.
- **`--tag ""` omits the field** — passing an empty string is the same as not passing `--tag`.
- **`--parse-level` + `--msg`**: level is still detected from the raw line prefix; `--msg` overrides the message. The two flags are independent.
- **Extra fields from `--msg` JSON merge are alphabetically sorted** — predictable order for agents parsing the output.
- **Large integers are preserved** — `json.RawMessage` stores exact bytes; no float64 precision loss on merge.
- **`WARNING` and `WARN` both map to level `"warn"`** — the output level string is always the short form.

## throttle — Rate Limiter

Delays lines from stdin to match a rate constraint. Prevents hitting API rate
limits when calling LLMs or external services in a loop. Input: stdin only.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--rate <N/s\|N/m>` | `-r` | Rate limit — required. N must be a positive integer |
| `--burst N` | — | Emit first N lines without delay, then rate-limit the rest |
| `--tokens-field <field>` | — | Rate by token count of a JSONL field value (dot-path) |
| `--json` | `-j` | Append `{"_vrk":"throttle","rate":"...","lines":N,"elapsed_ms":N}` after all lines |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty input) |
| 1 | I/O error, token count failure, invalid JSON with `--tokens-field` |
| 2 | Usage error — missing `--rate`, rate ≤ 0, bad format, unknown flag, TTY stdin |

### Examples

```bash
# Limit to 5 lines per second
seq 100 | vrk throttle --rate 5/s

# 1 line per second (60/m)
seq 10 | vrk throttle --rate 60/m

# First 3 lines immediately, then 1/s
seq 10 | vrk throttle --rate 1/s --burst 3

# Rate by token count of a JSONL field (10 tokens/s)
printf '{"prompt":"hi"}\n{"prompt":"hello world"}\n' | vrk throttle --rate 10/s --tokens-field prompt

# Emit metadata trailer
seq 5 | vrk throttle --rate 2/s --json
# → <5 lines>
# → {"_vrk":"throttle","rate":"2/s","lines":5,"elapsed_ms":2500}

# Combine with prompt to stay under an API rate limit
cat prompts.jsonl | vrk throttle --rate 10/m | vrk prompt --system "process" --model gpt-4o
```

### Gotchas

- **`--rate` is required** — omitting it exits 2 with a usage error. There is no default rate.
- **N must be a positive integer** — `--rate 0.5/s` exits 2 with "positive integer" error. Use `--rate 1/m` for sub-1/s rates.
- **Empty lines are skipped** — a truly empty line (`""`) produces no output. A whitespace-only line (`"   "`) is content and passes through unchanged.
- **`--burst` counts lines, not tokens** — with `--tokens-field`, burst still applies per-line regardless of token count.
- **`--tokens-field` on non-JSON input exits 1** — use only with JSONL streams. Bad JSON or a missing field is a fatal error, not a skip.
- **`elapsed_ms` is wall time from first to last line** — it includes all sleep time. Use it for pipeline instrumentation, not performance tuning.

## jsonl — JSON Array ↔ JSONL Converter

Converts a JSON array to JSONL (one record per line) or collects JSONL lines into a JSON array.
The bridge between JSON-land tools and line-oriented pipeline tools.
Input: stdin only.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--collect` | `-c` | JSONL → JSON array mode |
| `--json` | `-j` | Append `{"_vrk":"jsonl","count":N}` after all records (split mode only; no-op in collect mode) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty input, empty array, `--collect` on empty stdin) |
| 1 | Invalid JSON input; non-array input in split mode; invalid JSON line in collect mode |
| 2 | Usage error — interactive TTY with no stdin, unknown flag |

### Examples

```bash
# JSON array → JSONL (default split mode)
echo '[{"a":1},{"a":2},{"a":3}]' | vrk jsonl

# JSONL → JSON array
printf '{"a":1}\n{"a":2}\n{"a":3}\n' | vrk jsonl --collect

# Empty array → no output, exit 0
echo '[]' | vrk jsonl

# Primitives are emitted as JSON values
echo '[1,2,3]' | vrk jsonl
# → 1
#   2
#   3

# Add metadata trailer (split mode)
echo '[{"a":1},{"a":2}]' | vrk jsonl --json
# → {"a":1}
#   {"a":2}
#   {"_vrk":"jsonl","count":2}

# Round-trip pipeline
echo '[{"a":1},{"b":2}]' | vrk jsonl | vrk jsonl --collect
# → [{"a":1},{"b":2}]

# Bridge between array tools and line tools
vrk grab https://api.example.com/items | vrk jsonl | vrk mask | vrk jsonl --collect
```

### Gotchas

- **Empty stdin in split mode exits 0 with no output** — `printf '' | vrk jsonl` is valid, not an error.
- **Empty stdin in collect mode outputs `[]`** — `printf '' | vrk jsonl --collect` outputs `[]` and exits 0.
- **`--json` is a no-op in collect mode** — the output is a single JSON array; a metadata trailer after `]` would produce invalid JSON and break downstream parsers.
- **Invalid JSON in collect mode exits 1 immediately** — with a line number in the error message. No partial output is emitted.
- **Streaming in split mode** — uses `json.Decoder`, so arrays larger than memory are handled safely.
- **Round-trip is structurally equal, not byte-identical** — `json.Marshal` sorts object keys alphabetically. `{"b":2,"a":1}` round-trips as `{"a":1,"b":2}`.
- **`--collect` skips blank lines** — empty lines between JSONL records are silently skipped.

## digest — Universal Hasher

Hashes stdin or files. SHA-256 by default. Outputs `algo:hash`. Supports HMAC for message authentication and file comparison.
Input: positional argument or stdin, or one or more `--file` paths.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--algo <name>` | `-a` | Hash algorithm: `sha256` (default), `md5`, `sha512` |
| `--bare` | `-b` | Output hash only — no `algo:` prefix; mutually exclusive with `--json` |
| `--file <path>` | none | File to hash; repeatable for multiple files |
| `--compare` | none | Compare hashes of all `--file` inputs; outputs `match: true` or `match: false`; exits 0 either way |
| `--hmac` | none | Compute HMAC instead of plain hash; requires `--key` |
| `--key <secret>` | `-k` | HMAC secret key |
| `--verify <hex>` | none | Compare computed HMAC against this hex string; exits 0 on match, 1 on mismatch (constant-time) |
| `--json` | `-j` | Emit JSON object with metadata |

### --json output shapes

| Mode | Shape |
|------|-------|
| stdin / positional | `{"input_bytes":N,"algo":"sha256","hash":"..."}` |
| `--file` | `{"file":"<path>","algo":"sha256","hash":"..."}` |
| `--hmac` | `{"input_bytes":N,"algo":"sha256","hmac":"..."}` |
| `--compare` | `{"files":[...],"algo":"sha256","hashes":[...],"match":true}` |
| any error | `{"error":"digest: ...","code":N}` |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — hash produced, `--compare` result (match or mismatch), `--hmac --verify` match |
| 1 | Runtime error — `--hmac --verify` mismatch, file not found, I/O error |
| 2 | Usage error — interactive terminal with no `--file`; unknown flag; unknown `--algo`; `--hmac` without `--key`; `--verify` without `--hmac`; `--compare` with fewer than 2 `--file` values; `--bare` + `--json` together |

### Examples

```bash
# Hash stdin (default SHA-256)
echo 'hello' | vrk digest
# → sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824

# Positional arg form — identical result
vrk digest 'hello'

# Different algorithms
echo 'hello' | vrk digest --algo md5
echo 'hello' | vrk digest --algo sha512

# No prefix — useful when piping the hash onward
echo 'hello' | vrk digest --bare

# JSON output with metadata
echo 'hello' | vrk digest --json

# Hash a file
vrk digest --file /path/to/file

# Compare two files (exits 0 whether they match or not)
vrk digest --file a.txt --file b.txt --compare
# → match: true   or   match: false

# HMAC — produce and verify a message authentication code
echo 'payload' | vrk digest --hmac --key mysecret
HMAC=$(echo 'payload' | vrk digest --hmac --key mysecret --bare)
echo 'payload' | vrk digest --hmac --key mysecret --verify "$HMAC"
# → exit 0 (match); exit 1 on mismatch

# Pipeline: hash → store
echo 'hello' | vrk digest | vrk kv set last_hash
```

### Gotchas

- **`echo` appends a newline — `echo 'hello' | vrk digest` hashes `"hello\n"` (6 bytes), not `"hello"` (5 bytes).** To hash a string without a trailing newline, use either of these instead:
  ```bash
  vrk digest 'hello'          # positional arg — no newline added
  printf 'hello' | vrk digest # printf does not append \n
  ```
- **`--file` and `printf`-stdin produce identical hashes** — both stream bytes verbatim with no modification. `echo`-stdin and `--file` will differ if the file does not end with `\n`.
- **`--compare` exits 0 for both match and mismatch** — the result is informational (stdout says `match: true`/`match: false`). The caller decides what to do. Use the stdout content, not the exit code, to detect mismatches.
- **`--verify` uses constant-time comparison** — `hmac.Equal` prevents timing attacks. Never use `==` to compare HMACs.
- **`--bare` and `--json` are mutually exclusive** — combining them exits 2 with a clear error.
- **`input_bytes` in JSON output counts bytes actually hashed** — for `echo 'hello'`, `input_bytes` is 6 (the newline is included).
- **MD5 is available but not recommended for security** — offer it only when compatibility with existing MD5 checksums is required.

---

## base — Encoding Converter

Encodes and decodes between base64, base64url, hex, and base32.
Input: positional argument or stdin.

### Subcommands

| Subcommand | Required flag | Description |
|------------|---------------|-------------|
| `encode` | `--to <encoding>` | Encode input to the target format |
| `decode` | `--from <encoding>` | Decode input from the source format |

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--to <encoding>` | — | Target encoding for `encode`: `base64`, `base64url`, `hex`, `base32` |
| `--from <encoding>` | — | Source encoding for `decode`: same set |
| `--quiet` | `-q` | Suppress stderr; exit codes unaffected |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty input) |
| 1 | Runtime error — invalid input data for the chosen decoding |
| 2 | Usage error — no subcommand, missing `--to`/`--from`, unsupported encoding name, unknown flag, interactive TTY with no input |

### Examples

```bash
# Encode
echo 'hello' | vrk base encode --to base64      # aGVsbG8=
echo 'hello' | vrk base encode --to base64url   # aGVsbG8  (no padding)
echo 'hello' | vrk base encode --to hex         # 68656c6c6f
echo 'hello' | vrk base encode --to base32      # NBSWY3DP

# Decode
echo 'aGVsbG8=' | vrk base decode --from base64
echo '68656c6c6f' | vrk base decode --from hex

# Binary round-trip
printf '\x00\x01\x02\xff' | vrk base encode --to hex   # 000102ff
printf '\x00\x01\x02\xff' | vrk base encode --to hex | vrk base decode --from hex

# Pipeline: encode a secret for safe transport, then decode at destination
echo "$SECRET" | vrk base encode --to base64url
echo "$ENCODED" | vrk base decode --from base64url
```

### Gotchas

- **`base` strips one trailing newline from stdin before encoding.** `echo 'hello' | vrk base encode --to hex` encodes `hello` (5 bytes), not `hello\n` (6 bytes). Use `printf 'hello\n'` if you need to encode the newline itself. This differs intentionally from `vrk digest`, which does not strip — `base` is an encoding tool where callers expect string semantics.
- **Multi-line wrapped base64 is not supported.** Input like `base64 -w76` output (line-wrapped) will fail decoding. Strip internal newlines first: `tr -d '\n' | vrk base decode --from base64`.
- **base64url uses no padding and URL-safe alphabet.** The output uses `-` and `_` instead of `+` and `/`, with no trailing `=`. This is the correct format for JWT headers and URL query parameters.
- **hex output is lowercase.** `echo 'hello' | vrk base encode --to hex` produces `68656c6c6f`, not `68656C6C6F`. Decoders are case-sensitive — use the output of `vrk base encode --to hex` as the input to `vrk base decode --from hex`.
- **base32 output is uppercase.** Decoding is case-sensitive; lowercase base32 input exits 1.
- **`--quiet` does not suppress the no-subcommand error.** That error fires before flag parsing; `--quiet` takes effect only inside a subcommand.

---

## recase — Naming Convention Converter

Converts text between naming conventions. Auto-detects the input convention from
separators and casing. Reads stdin line by line — one line in, one line out.
Input: stdin (streaming, line by line).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--to <convention>` | — | Target naming convention (required): `camel`, `pascal`, `snake`, `kebab`, `screaming`, `title`, `lower`, `upper` |
| `--json` | `-j` | Emit a JSON object per line: `{"input":"…","output":"…","from":"…","to":"…"}` |
| `--quiet` | `-q` | Suppress stderr; exit codes unchanged |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty stdin) |
| 1 | I/O error reading stdin |
| 2 | Usage error — `--to` missing or unknown, unknown flag, interactive TTY with no input |

### Examples

```bash
# Basic conversions
echo 'hello_world'  | vrk recase --to camel      # helloWorld
echo 'hello_world'  | vrk recase --to pascal     # HelloWorld
echo 'hello_world'  | vrk recase --to kebab      # hello-world
echo 'hello_world'  | vrk recase --to screaming  # HELLO_WORLD
echo 'hello_world'  | vrk recase --to title      # Hello World
echo 'helloWorld'   | vrk recase --to snake      # hello_world
echo 'HELLO_WORLD'  | vrk recase --to camel      # helloWorld

# Acronyms
echo 'userID'    | vrk recase --to snake   # user_id
echo 'parseHTML' | vrk recase --to snake   # parse_html

# Multiline batch — one conversion per line, preserves line count
printf 'hello_world\nfoo_bar\n' | vrk recase --to camel
# helloWorld
# fooBar

# JSON output for pipeline inspection
echo 'hello_world' | vrk recase --to camel --json
# {"input":"hello_world","output":"helloWorld","from":"snake","to":"camel"}

# Rename variable names in a list
cat vars.txt | vrk recase --to camelCase
```

### Gotchas

- **`lower` and `upper` are prose-level transforms, not identifier formats.** Output is space-separated: `hello_world → lower` produces `"hello world"`, not `"hello_world"`. Use `snake` or `kebab` for identifier output.
- **Single words with no separators and no case changes are treated as one word.** `helloworld → camel` stays `helloworld` — there are no word boundaries to detect. Use a separator (`hello_world`) to get `helloWorld`.
- **Digits are not word boundaries.** `oauth2` is one token; `base64url` is one token. `oauth2 → pascal` = `Oauth2`.
- **Two consecutive acronyms with no separator cannot be split.** `getHTTPSURL → snake` produces `get_httpsurl`, not `get_https_url`. The algorithm needs a lowercase letter to know where one acronym ends and the next begins. Workaround: use an explicit separator in the input: `getHTTPS_URL` or `get-https-url`.
- **Empty lines are preserved.** A blank line in produces a blank line out. This maintains line counts in batch pipelines.
- **Auto-detection priority:** separators (`_`, `-`, ` `) take precedence over case. `hello_world` is always detected as `snake`, even if it could theoretically be read another way.

---

## slug — URL/Filename Safe Slug Generator

Converts text to URL/filename-safe slugs. Lowercase, hyphen-separated, unicode normalised to ASCII. One slug per input line; empty slugs (empty input or all-punctuation) produce no output.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--separator <s>` | — | Word separator (default: `-`). Any string, including multi-character or empty. |
| `--max <n>` | — | Max output length (0 = unlimited). Truncates at last word boundary at or before `n`. |
| `--json` | `-j` | Emit `{"input":"...","output":"..."}` per input line (JSONL). Empty-slug lines suppressed. |
| `--quiet` | `-q` | Suppress stderr; exit codes unchanged. |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty output) |
| 1 | I/O error reading stdin |
| 2 | Usage error — interactive terminal with no stdin, unknown flag |

### Examples

```bash
# Basic slug
echo 'Hello World' | vrk slug
# → hello-world

# Positional arg form
vrk slug 'Hello, World! (2026)'
# → hello-world-2026

# Unicode normalisation
echo 'Ünïcödé Héró' | vrk slug
# → unicode-hero

# Custom separator
echo 'Hello World' | vrk slug --separator _
# → hello_world

# Max length at word boundary
echo 'A very long title' | vrk slug --max 12
# → a-very-long

# Multiline batch
printf 'Hello World\nFoo Bar\n' | vrk slug
# → hello-world
# → foo-bar

# JSON output
echo 'Hello World' | vrk slug --json
# → {"input":"Hello World","output":"hello-world"}

# Pipeline: slugify page titles fetched from links
cat README.md | vrk links --bare | vrk slug
```

### Gotchas

- **`--max` truncates at word boundary — if the first word is longer than `--max`, output is empty.** For example, `echo 'helloworld' | vrk slug --max 3` produces no output, not a truncated `hel`. Use a value larger than the longest expected word to avoid silent empty output.
- **Empty slug lines are suppressed.** Inputs that produce no alphanumeric content (empty string, all-punctuation) emit no output line. This is intentional — slug treats empty output as a no-op, consistent with `echo '' | vrk slug` exiting 0 with no output.
- **Non-Latin characters are dropped.** Only Latin-script characters with NFD decomposition to ASCII (`é → e`, `ü → u`, etc.) are preserved. Characters with no ASCII base (Cyrillic, CJK, Arabic) are treated as word boundaries and dropped. The spec is ASCII-only slug output.
- **`--separator` allows any string, including empty.** `--separator ''` joins words without any separator (`hello-world → helloworld`). Multi-character separators (`--separator --`) are allowed but unusual.

---

## Breaking changes

### digest — stdin no longer strips trailing newline (streaming fix)

Previously, `echo 'foo' | vrk digest` hashed `"foo"` (3 bytes).
After this change it hashes `"foo\n"` (4 bytes, the literal pipe content).

This aligns stdin with `--file` behaviour (which never stripped bytes) and
fixes an OOM crash on binary pipes. `printf 'foo' | vrk digest` and
`vrk digest foo` are unaffected — neither ever produced a trailing newline.

Migration: replace `echo 'value' |` with `printf 'value' |` anywhere the old
hash is expected.

---

## moniker — Memorable Name Generator

Generates human-readable adjective-noun names for run IDs, job labels, and temporary
directory names. Like Docker container names and Heroku dyno names.
Input: none (generates names from embedded wordlists; stdin is ignored).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--count` | `-n` | Number of names to generate (default: 1) |
| `--separator` | — | Word separator (default: `-`) |
| `--words` | — | Number of words per name, minimum 2 (default: 2) |
| `--seed` | — | Fix random seed for deterministic output |
| `--json` | `-j` | Emit one JSON record per name |
| `--quiet` | `-q` | Suppress stderr error messages; exit codes unchanged |

### --json record shape

The shape is identical regardless of `--words`:

```json
{"name":"misty-mountain","words":["misty","mountain"]}
```

`words` always has exactly `--words` elements. `name` is always `words` joined by `--separator`.

| Flags | Shape |
|-------|-------|
| `--json` (any `--words`) | `{"name":"...","words":["w1","w2",...]}` |
| Any error + `--json` | `{"error":"msg","code":N}` on stdout; stderr empty |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Requested count exceeds available unique combinations |
| 2 | Usage error — `--count 0`, `--words < 2`, unknown flag |

### Examples

```bash
# Generate one name (default)
vrk moniker

# Generate 5 names
vrk moniker --count 5

# Use underscore separator
vrk moniker --separator _

# Three-word name
vrk moniker --words 3

# Deterministic output (same name every run)
vrk moniker --seed 42

# JSON output for pipeline consumption
vrk moniker --json

# 1000 unique names — guaranteed no duplicates within the batch
vrk moniker --count 1000
```

### Compose patterns

```bash
# Use a generated name as a run ID stored in kv
RUN_ID=$(vrk moniker --seed "$EPOCH")
vrk kv set "run:$RUN_ID" "active"

# Generate a batch of names and store each one
vrk moniker --count 5 | while read name; do
  vrk kv set "job:$name" "pending"
done

# Extract name field from JSON output
vrk moniker --json | python3 -c 'import sys,json; print(json.load(sys.stdin)["name"])'

# Create a temp directory with a memorable name
tmpdir=$(mktemp -d "/tmp/$(vrk moniker).XXXXXX")
```

### Gotchas

- **stdin is always ignored.** Moniker generates names from embedded wordlists. Piping input to it has no effect. It is safe to use in any pipeline position.
- **`--seed 0` is a valid seed.** Seed zero is distinct from "no seed". Without `--seed`, each run uses a random seed. With `--seed 0`, the output is deterministic.
- **`--count 1000` guarantees 1000 unique names.** The 2-word combination pool has 87,000+ entries. Requesting more than the pool size exits 1 with a clear message.
- **`--json` always emits a `words` array.** The shape `{"name":"...","words":[...]}` is identical for all `--words` values. Agent code can always access `words[0]`, `words[1]`, etc. without branching on word count.
- **Separator does not affect uniqueness.** Uniqueness is guaranteed at the word-index level. Changing `--separator` produces different strings but the same underlying combinations.

---

## pct — Percent Encoder/Decoder

Encodes and decodes per RFC 3986. Processes input line by line — each input line produces one output line.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--encode` | — | Percent-encode input (RFC 3986 strict) |
| `--decode` | — | Percent-decode input |
| `--form` | — | Form encoding mode: spaces↔`+` instead of `%20` |
| `--json` | `-j` | Emit one JSON object per line: `{"input":…,"output":…,"op":…,"mode":…}` |
| `--quiet` | `-q` | Suppress stderr; exit codes unchanged |

### --json record shape

```json
{"input":"hello world","output":"hello%20world","op":"encode","mode":"percent"}
```

Fields: `input` (original line), `output` (encoded/decoded result), `op` (`"encode"` or `"decode"`), `mode` (`"percent"` or `"form"`).

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, including empty input |
| 1 | Invalid percent-encoded sequence during decode |
| 2 | Usage error — neither or both mode flags set, unknown flag, interactive terminal |

### Examples

```bash
# Encode a string
echo 'hello world & more' | vrk pct --encode
# → hello%20world%20%26%20more

# Decode
echo 'hello%20world%20%26%20more' | vrk pct --decode
# → hello world & more

# Form encoding (spaces as +)
echo 'hello world' | vrk pct --encode --form
# → hello+world

# Form decoding (+ as space)
echo 'hello+world' | vrk pct --decode --form
# → hello world

# Positional arg
vrk pct --encode 'hello world'
# → hello%20world

# JSON output
echo 'hello world' | vrk pct --encode --json
# → {"input":"hello world","output":"hello%20world","op":"encode","mode":"percent"}

# Multiline batch
printf 'a b\nc d\n' | vrk pct --encode
# → a%20b
# → c%20d

# Round-trip pipeline
echo 'hello world & more' | vrk pct --encode | vrk pct --decode
# → hello world & more
```

### Compose patterns

```bash
# URL-encode a query parameter before passing to curl
Q=$(echo "$USER_QUERY" | vrk pct --encode)
curl "https://api.example.com/search?q=$Q"

# Decode a percent-encoded value from a JWT claim
vrk jwt --claim redirect_uri "$TOKEN" | vrk pct --decode

# Encode each line of a file independently
cat urls.txt | vrk pct --encode > encoded.txt

# Guard: fail if decoding produces invalid output (non-zero exit propagates)
echo "$ENCODED_VALUE" | vrk pct --decode | vrk validate --schema '{"type":"string"}'
```

### Gotchas

- **Double-encode is correct behaviour.** `echo 'hello%20world' | vrk pct --encode` produces `hello%2520world` — the `%` is encoded as `%25`. This is RFC 3986 conformant. If you want to avoid double-encoding, decode first.
- **`+` is a literal character in non-form decode.** `echo 'hello+world' | vrk pct --decode` produces `hello+world` unchanged. To treat `+` as a space, use `--form`.
- **`%2B` decodes to `+` in both modes.** `echo 'hello%2Bworld' | vrk pct --decode` → `hello+world`. The percent-encoded form is always authoritative.
- **In `--form` mode, `~` is encoded as `%7E`.** This is browser-compatible behaviour (`url.QueryEscape`). Strictly, `~` is an RFC 3986 unreserved character and does not need encoding, but browsers encode it in form submissions. Fighting this creates more confusion than it solves.
- **`--encode` and `--decode` are mutually exclusive.** Passing both exits 2.

---

## urlinfo — URL Parser

Parses a URL string into its components as JSON. Pure string operation — no network calls, no HTTP. Use `grab` to fetch a URL; use `urlinfo` to inspect one.
Input: positional argument or stdin (single line or multiline batch).

### Record shape

```json
{"scheme":"https","host":"api.example.com","port":0,"path":"/v1/users","query":{"page":"2","limit":"10"},"fragment":"","user":""}
```

All fields always present, including zero values. Port is 0 when not explicit in the URL string.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--field <path>` | `-F` | Extract a single field; dot-path for query params (e.g. `query.page`) |
| `--json` | `-j` | Append `{"_vrk":"urlinfo","count":N}` after all records |
| `--quiet` | `-q` | Suppress stderr; exit codes unchanged |

### `--field` paths

| Path | Returns |
|------|---------|
| `scheme` | `https` |
| `host` | `api.example.com` |
| `port` | `8080` (empty string if not present in URL) |
| `path` | `/v1/users` |
| `fragment` | `anchor` (empty string if absent) |
| `user` | username (empty string if absent) |
| `query` | full raw query string |
| `query.<key>` | value of that query parameter (empty string if absent) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — URL parsed (including scheme-relative `//host/path`) |
| 1 | Invalid URL — both scheme and host are empty after parsing |
| 2 | Usage error — interactive terminal with no input, unknown flag |

### Examples

```bash
# Parse a URL into JSON
vrk urlinfo 'https://api.example.com/v1/users?page=2&limit=10'

# Extract a single field
vrk urlinfo --field host 'https://api.example.com/path'

# Extract a nested query parameter
vrk urlinfo --field query.page 'https://example.com?page=2'

# Batch parse from stdin
printf 'https://example.com\nhttps://api.example.com\n' | vrk urlinfo

# Stdin form (identical to positional arg)
echo 'https://example.com/path' | vrk urlinfo
```

### Compose patterns

```bash
# Extract host from a fetched URL's redirect chain
vrk grab https://bit.ly/xyz --json | jq -r .url | vrk urlinfo --field host

# Validate all URLs in a file have https scheme
cat urls.txt | vrk urlinfo --field scheme | grep -v '^https$'

# Extract all query params as structured JSON
echo 'https://example.com?page=2&limit=10' | vrk urlinfo | jq .query

# Pipeline: parse URLs from a page then extract their hosts
cat page.html | vrk links --bare | vrk urlinfo --field host
```

### Gotchas

- **Password is never output** — inspect the raw string directly if you need credential validation.
- **`--field port` returns empty string when port is not explicit in the URL** — `https://example.com` has no port in the string even though HTTPS implies 443.
- **`urlinfo` accepts any string `url.Parse` accepts** — Go's parser is lenient; a string with no scheme parses without error but produces an empty scheme field. Exit 1 only when both scheme and host are empty.
- **`--field query.<key>` returns empty string if the key is absent** — not an error.
- **`--field` suppresses the `--json` metadata trailer** — plain-text field output and a JSON trailer are incoherent; only field values are printed.
- **Empty input exits 0 with no output.** `printf '' | vrk pct --encode` is valid; it produces nothing and exits 0.

---

## sip — Stream Sampler

Samples lines from stdin using one of four strategies. Memory-efficient for arbitrarily large streams — reservoir sampling uses O(N) memory where N is the sample size, not the stream size.
Input: stdin only (no positional argument — sip is a pure stream filter).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--first N` | — | Take first N lines (deterministic) |
| `--count N` | `-n` | Reservoir sample of exactly N lines (random, O(N) memory) |
| `--every N` | — | Emit every Nth line (deterministic) |
| `--sample N` | — | Include each line with N% probability (approximate, random) |
| `--seed N` | — | Fix random seed for deterministic output; 0 is a valid seed value |
| `--json` | `-j` | Append `{"_vrk":"sip","strategy":"...","requested":N,"returned":N,"total_seen":N}` after output |
| `--quiet` | `-q` | Suppress stderr; exit codes unchanged |

Exactly one of `--first`, `--count`, `--every`, `--sample` must be specified.

### --json metadata shape

```json
{"_vrk":"sip","strategy":"reservoir","requested":100,"returned":87,"total_seen":1000}
```

| Field | Meaning |
|-------|---------|
| `strategy` | `"first"`, `"every"`, `"reservoir"`, or `"sample"` |
| `requested` | N from flag (percentage value for `--sample`) |
| `returned` | Actual lines emitted |
| `total_seen` | Non-empty lines read from stdin |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including sample larger than population — emit all, no error) |
| 1 | I/O error reading stdin |
| 2 | Usage error — no strategy flag, multiple strategy flags, invalid flag value, interactive terminal |

### Examples

```bash
# Inspect the first 10 lines of any stream
cat data.jsonl | vrk sip --first 10

# Every 100th line — uniform stride across a large file
seq 1000000 | vrk sip --every 100

# Reproducible reservoir sample of 1000 lines — same seed gives same output every run
cat events.log | vrk sip --count 1000 --seed 42

# Approximate 5% random sample (count varies per run)
cat events.log | vrk sip --sample 5

# Metadata: how many lines were in the stream and how many were returned
seq 1000 | vrk sip --count 100 --json | tail -1
```

### Gotchas

- **`--sample` is probabilistic** — `--sample 10` on 1000 lines produces approximately 100, not exactly 100. Use `--count` for exact counts.
- **`--seed 0` is valid** — seed value 0 is not a "not set" sentinel. Use it freely.
- **Reservoir output preserves input order** — `--n 100` on a 1000-line stream emits 100 lines in the same relative order they appeared in the input, not in random order.
- **Empty lines are skipped and not counted** — a line that is `""` is invisible to all strategies and does not appear in `total_seen`.
- **`--first` and `--every` still read the full stream** — `total_seen` reflects all non-empty lines, even for `--first N` where only the first N are emitted.
- **No positional argument** — `sip` is a pure filter; input must come from a pipe.

---

## assert — Pipeline Condition Check

Evaluates conditions on stdin and either passes data through (exit 0) or kills
the pipeline (exit 1). The test gate for agent output verification — put it
between any two tools to enforce invariants.
Input: stdin only (pipe).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `<condition>` | — | jq-compatible condition (positional, repeatable; all must pass) |
| `--message <msg>` | `-m` | Custom failure message shown in stderr and `--json` output |
| `--contains <str>` | — | Plain text: assert stdin contains substring (case-sensitive) |
| `--matches <regex>` | — | Plain text: assert stdin matches Go regex |
| `--json` | `-j` | Emit `{"passed":bool,...}` to stdout; exit codes unchanged |
| `--quiet` | `-q` | Suppress stderr on failure; exit codes unchanged |

`--contains` and `--matches` can be combined (both must pass). Both are
mutually exclusive with positional jq conditions.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Assertion passed — stdin passed through byte-for-byte |
| 1 | Assertion failed, JSON parse error, or I/O error |
| 2 | Usage error — no condition, no stdin, mode conflict, invalid regex |

### Examples

```bash
# JSON condition — pass through if status is ok
echo '{"status":"ok"}' | vrk assert '.status == "ok"'

# Multiple conditions — all must pass
echo '{"count":5,"errors":[]}' | vrk assert '.count > 0' '.errors == []'

# Numeric comparison
echo '{"score":0.91}' | vrk assert '.score > 0.8'

# Missing fields are null (jq compat)
echo '{}' | vrk assert '.missing == null'

# Array length check
echo '{"items":[1,2,3]}' | vrk assert '.items | length > 0'

# Plain text — substring match
echo 'All tests passed' | vrk assert --contains 'passed'

# Plain text — regex match
echo 'OK: all systems nominal' | vrk assert --matches '^OK:'

# Combine --contains and --matches (both must pass)
echo 'OK: all tests passed' | vrk assert --contains 'passed' --matches '^OK:'

# Custom failure message
echo '{"status":"fail"}' | vrk assert '.status == "ok"' --message 'Bad API response'

# JSON output mode
echo '{"status":"ok"}' | vrk assert '.status == "ok"' --json
# → {"passed":true,"condition":".status == \"ok\"","input":{"status":"ok"}}
```

### Compose patterns

```bash
# Gate pipeline: only store result if confidence is high enough
cat data.jsonl | vrk prompt --schema schema.json | vrk assert '.confidence > 0.8' | vrk kv set result

# Validate API response before processing
vrk grab https://api.example.com | vrk assert '.status == "ok"' | vrk emit --parse-level

# Assert + kv: only store if condition passes
echo '{"score":0.9}' | vrk assert '.score > 0.8' | vrk kv set last_result
```

### --json output shapes

| Scenario | Shape |
|----------|-------|
| Pass (JSON mode) | `{"passed":true,"condition":"<expr>","input":<parsed>}` |
| Fail (JSON mode) | `{"passed":false,"condition":"<expr>","input":<parsed>}` |
| Fail with --message | adds `"message":"<msg>"` to the fail object |
| Pass (plain text) | `{"passed":true,"condition":"--contains: <str>"}` |
| Fail (plain text) | `{"passed":false,"condition":"--contains: <str>","message":"..."}` |
| Error | `{"error":"assert: ...","code":2}` |

### Gotchas

- **Byte-for-byte transparency** — on success, stdin passes to stdout unchanged. No reformatting, no newline normalisation.
- **JSONL streaming** — for multi-line JSON input, assert evaluates each line independently. On the first failing line, exit 1. Lines already written to stdout before the failure are not recalled.
- **Positional conditions require valid JSON** — if input is not valid JSON, exit 1. Use `--contains`/`--matches` for plain text.
- **`--contains`/`--matches` read full stdin as one blob** — they do not operate line-by-line. They are plain text modes.
- **jq truthiness** — non-boolean gojq results use jq semantics: `null` and `false` are falsy, everything else (including `0`, empty string, empty array) is truthy.
- **`.field > 0` on a missing field** — compares `null > 0` which is false. Check for presence first if the field may be absent.
- **`--quiet` suppresses stderr only** — it does not suppress `--json` output (which goes to stdout).
- **`--json` fail: `message` field omitted when `--message` not set** — only included when explicitly provided via `--message`.

---

# Utilities

These are not pipeline tools — they support discovery and integration.

---

## mcp — MCP Server (Discovery Only)

Starts a discovery-only MCP server over stdio (JSON-RPC 2.0). Exposes all vrksh
pipeline tools for MCP client discovery via `tools/list`. Does not execute tools —
`tools/call` is not implemented.

Add `vrk mcp` to your Claude Code MCP config to see all tools in discovery.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--list` | — | Print all exposed tools and exit (human-readable) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Clean shutdown (SIGINT/EOF) or `--list` completed |
| 1 | Startup error |
| 2 | Usage error — unknown flag |

### Examples

```bash
# Start MCP server (stdio mode, for Claude Code)
vrk mcp

# List all exposed tools at the terminal
vrk mcp --list

# Send an initialize request
printf '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}\n' | vrk mcp

# Send a tools/list request
printf '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}\n' | vrk mcp
```

### Gotchas

- **Discovery only** — `tools/call` returns method-not-found (-32601). Use shell invocation (`vrk <tool> [flags]`) to call tools.
- **Stdout purity** — nothing goes to stdout except JSON-RPC responses. All logging goes to stderr.
- **Tool names are prefixed** — MCP tool names use `vrk_` prefix (e.g. `vrk_tok`, `vrk_jwt`) to avoid collisions in MCP clients.
- **`input` field in schemas** — tools that require stdin have `"input"` in their inputSchema's `required` array. Tools that don't need stdin (uuid, epoch, moniker) do not.
