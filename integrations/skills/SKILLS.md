# vrksh Skills

This file is embedded in the `vrk` binary and served via `vrk --skills`.
It is the agent-facing reference for using vrksh tools in AI pipelines.

Running bare `vrk` with no arguments prints the tool list to stdout and exits 0; `vrk help`, `vrk -h` and `--help` also print to stdout. `vrk --version` (or `vrk version`) prints the version. Line-based tools (mask, emit, sip, throttle, slug, recase, sse, jsonl, validate, assert) accept lines up to 64 MiB; a longer line fails with `line exceeds 64 MiB limit`.

---

## jwt - JWT Inspector

Decodes a JWT and prints the payload as JSON. It does not verify signatures.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--claim <name>` | `-c` | Print a single claim value as plain text |
| `--expired` | `-e` | Exit 1 if the token is expired |
| `--valid` | - | Exit 1 if expired, nbf in future, or iat in future |
| `--json` | `-j` | Emit structured JSON output (shape depends on other flags) |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### --json output shapes

| Flags | Shape |
|-------|-------|
| `--json` alone | `{"header":{…},"payload":{…},"signature":"…","expired":bool,"valid":bool,"signature_verified":false}` |
| `--expired --json` | `{"expired":bool}` - exit 1 if expired, 0 if not |
| `--claim <name> --json` | `{"claim":"name","value":"…"}` |
| Any error + `--json` | `{"error":"msg","code":N}` on stdout; stderr empty |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success - token decoded, condition met |
| 1 | Runtime error - invalid token, expired (with `--expired`), claim not found |
| 2 | Usage error - no input provided, unknown flag |

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

- `--expired` exits 1 only if the `exp` claim is present **and** the current time is on or after it (RFC 7519).
  If the token has no `exp` claim, `vrk jwt` treats it as never expiring and exits 0.
- **`valid` means the time claims are OK. It does not mean the token is genuine.** `vrk jwt` never checks the signature,
  so `alg: none` with an empty signature can give `"valid":true`. `signature_verified` is always `false`.
  Verify signatures with your auth library before you trust any claim.
- **Time claims fail closed.** `exp`, `nbf` and `iat` can be fractional or use an exponent (`1e9`). `vrk jwt` compares them as real
  dates. If a time claim is present but is not a number (a string, `null`), `--expired` and `--valid` exit 1 and
  `valid` is false.
- **`--claim` prints strings raw.** The claim is token data that an attacker can control, and terminal escape sequences in it
  reach your terminal unchanged. Before you show a claim to a person, use `--claim <name> --json` or pipe it through `cat -v`.
- `--json` alone never exits 1 for an expired token. Check the `expired` field or use `--expired`.
- `--expired --json` exits 1 when the token is expired, and stdout still has `{"expired":true}`.
  Without `--json`, `--expired` leaves stdout empty on exit 1.
- When `--json` is active, all errors go to stdout as `{"error":"msg","code":N}` and stderr is empty.
- Default output (no flags) prints the payload only. Use `--json` to also get the header and signature.
- Stdout is always empty on error unless `--json` is active.

---

## epoch - Timestamp Converter

Converts between Unix timestamps and ISO 8601 dates/times.
Default output is always a Unix integer. `--iso` switches to ISO 8601.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--iso` | - | Output as ISO 8601 string instead of Unix integer |
| `--json` | `-j` | Emit structured JSON: `{input?, unix, iso, ref?, tz?}` |
| `--tz <zone>` | - | Timezone for `--iso` or `--json` output; IANA name or `+HH:MM` offset |
| `--now` | - | Print current Unix timestamp and exit |
| `--at <ts>` | - | Override reference time for relative input (unix integer) |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### --json output shape

```json
{"input":"+3d","unix":1740268800,"iso":"2025-02-23T00:00:00Z","ref":"1740009600","tz":"+05:30"}
```

- `input`: the original input string - omitted for `--now`
- `unix`: computed Unix timestamp (integer)
- `iso`: always present - ISO 8601 in UTC or the specified `--tz`
- `ref`: only when `--at` was used
- `tz`: only when `--tz` was used

Errors with `--json` active: `{"error":"msg","code":N}` to stdout; stderr empty.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Bad input value - unsupported format, invalid date, relative offset without a sign |
| 2 | Usage error - unknown flag, no input, bad `--tz` or `--at`, ambiguous timezone, `--tz` without `--iso` or `--json` |

### Relative time format

Must include a sign prefix: `+3d` (3 days from now), `-3d` (3 days ago).
Bare `3d` exits 1 with "sign required". Units: `s` `m` `h` `d` `w` (no months or years).
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

- Relative times **must** be signed: `+3d` or `-3d`. Bare `3d` exits 1.
- Integers from 1e11 up to (not including) 1e14 are read as milliseconds. `vrk epoch --iso 1792042275000` gives `2026-10-15T05:31:15Z`; `--json` adds `"unit":"ms"`.
- Timezone abbreviations (IST, EST, PST) exit 2 because they are ambiguous.
  Use full IANA names (`America/New_York`) or numeric offsets (`+05:30`).
- `--tz` requires `--iso` or `--json`. With plain integer output, it exits 2.
- A Unix integer input passes through unchanged. The timezone changes only `--iso` and `--json` output.
- Use `--at <ts>` to make pipelines involving relative times deterministic.
- `--now` is a boolean flag (prints current timestamp and exits). Use `--at` to set a reference.
- Negative integers (`-1000`) are valid pre-epoch Unix timestamps. Pass them on stdin so they are not read as flags.
- When `--json` is active, errors go to stdout as `{"error":"msg","code":N}` and stderr is empty.
- Stdout is always empty on error unless `--json` is active.
- Relative offsets that overflow int64 seconds (`-9223372036854775807d`) exit 1 with `relative time ... is out of range`. They do not wrap.

---

## uuid - UUID Generator

Generates UUIDs. v4 (random) by default, v7 (time-ordered) with `--v7`.
Reads no stdin - input is never required or consumed.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--v7` | - | Generate a v7 (time-ordered) UUID instead of v4 |
| `--count <n>` | `-n` | Number of UUIDs to generate (default 1, must be >= 1) |
| `--json` | `-j` | Emit each UUID as a JSON object: `{uuid, version, generated_at}` |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error - `--count` less than 1, unknown flag |

### Output format

Plain text (default): one UUID per line, lowercase, with hyphens. Format: `8-4-4-4-12` hex characters.

JSON (`--json`): one object per line (JSONL), always with these three fields:
```json
{"uuid":"a3f2c1d4-8e7b-4f2a-9c1d-3e5f7a8b9c0d","version":4,"generated_at":1740000000}
```
`generated_at` is a Unix timestamp (seconds) computed once per invocation - all UUIDs in a batch share the same value.

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

- `uuid` reads **no stdin**. It ignores anything you pipe into it.
- v7 UUIDs in one batch are **lexicographically ordered**. A monotonic counter makes each UUID greater than the last, even within the same millisecond.
- `--count 0` exits 2. It is a usage error, not a no-op.
- `uuid` computes `generated_at` **once, before it generates the batch**. All UUIDs in a `--count N` batch share one timestamp. It shows when you requested the batch.
- Stdout is always empty on error - errors go to stderr only.

---

## tok - Token Counter

Counts tokens in stdin with o200k_base, the tokenizer of GPT-4o, GPT-4.1, GPT-5
and the o-series. `--model gpt-4` or `gpt-3.5-turbo` counts with cl100k_base.
The count is exact for OpenAI models and an estimate for Claude.
If you pass `--check N`, `tok` is a pipeline gate: it passes the input
through when the count is within N, and exits 1 when the count is over N.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--check <N>` | - | Pass input through if within N tokens; exit 1 with empty stdout if over. Reads all stdin before deciding. |
| `--model <name>` | `-m` | OpenAI model or encoding: `gpt-5`, `gpt-4o`, `o3` and other current models use `o200k_base` (default); `gpt-4`, `gpt-3.5-turbo` use `cl100k_base`. Unknown names exit 2. |
| `--json` | `-j` | Emit output as `{"tokens": N, "model": "o200k_base"}` (measurement mode) or JSON error (check mode). `model` is the encoding used. |
| `--quiet` | `-q` | Suppress all stderr output (exit codes unaffected) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success - count printed (or JSON emitted); `--check` within limit |
| 1 | Over limit - `--check N` was set and token count exceeds N |
| 2 | Usage error - unknown flag; unknown `--model`; running interactively with no piped input |

### Examples

```bash
# Count tokens in a file
cat prompt.txt | vrk tok

# Count a string directly
vrk tok "hello world"
# -> 2

# JSON output
echo "hello world" | vrk tok --json
# -> {"tokens":2,"model":"o200k_base"}

# Gate: pass through if within 8000 tokens
echo "hello world" | vrk tok --check 8000
# -> hello world  (exits 0, input passes through unchanged)

# Gate: exit 1 if over limit
cat big_context.txt | vrk tok --check 100
# -> (nothing on stdout, exits 1)

# Count for GPT-4 (cl100k_base)
echo "hello world" | vrk tok --model gpt-4 --json
# -> {"tokens":2,"model":"cl100k_base"}
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

- **`--check` buffers all stdin** before it decides whether to pass the input through. Memory use grows with input size. For input over 100MB, sample it first with `vrk sip`.
- **`--json` with `--check`, within the limit,** passes the raw input through, not a JSON envelope. `--json` changes only the error output.
- **`--check 0`** exits 1 for any non-empty input. Valid but rarely useful.
- **Runs over 4 KB without a break are merged in 4 KB pieces.** This keeps memory flat (20 MB of spaces counts in under a second). Counts match tiktoken for normal text; a single word or space run longer than 4 KB can come out a token or two off, and `chunk` output still decodes back to the exact input.
- **Positional args pass through** joined with a space and no trailing newline. Stdin passes through byte for byte, including trailing newlines.
- **`--model` picks the tokenizer.** Matching follows tiktoken: `gpt-4`, `gpt-4-*`, `gpt-3.5*` and `text-embedding-*` count with cl100k_base; every other `gpt-*`, `chatgpt-*` and `o1`/`o3`/`o4` model counts with o200k_base. Non-OpenAI names such as `claude-*` exit 2; leave `--model` off to estimate for them.
- **Counts differ between encodings.** English prose and code come out within about 1% of each other; Hindi, CJK and Greek are often half as many tokens in o200k_base. Pass the model you send to.
- **OpenAI tokenizers give an estimate for Claude.** The Claude tokenizer is not public. Set `--check` at 90% of the context limit of the model to allow for the error.
- **An empty pipe is 0 tokens, not an error.** `cat /dev/null | vrk tok` exits 0 and prints `0`. `vrk tok` exits 2 only when you run it in a terminal with no pipe.

---

## sse - SSE Stream Parser

Reads a raw `text/event-stream` from stdin and writes one JSON object per event to stdout (JSONL).
Stdin is the only input source. Pipe the HTTP stream into it.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--event <name>` | `-e` | Only emit events of this type; skip all others |
| `--field <path>` | `-F` | Extract a dot-path field from the record and print as plain text. Array indexes work: `data.choices[0].delta.content` or `data.choices.0.delta.content` |
| `--raw` | - | With `--field`, write each value with no newline after it, flushed per event, so streamed text reassembles exactly |

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

The dot-path starts at the top-level record (which has `event` and `data` keys):

```bash
vrk sse --field data.delta.text   # extracts the nested text token
vrk sse --field event             # extracts the event name itself
```

- String values are printed as-is (no quotes).
- Number and boolean values are printed as their JSON representation (`42`, `true`).
- If the path is not found, the record is skipped silently. If the stream had events but `--field` matched none of them, `sse` exits 1 with `sse: --field <path> matched no events`. A stream with no events at all still exits 0.
- If `data` is not a JSON object, the record is skipped silently.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success - stream parsed, `[DONE]` encountered, or stdin closed cleanly |
| 1 | Runtime error - I/O error reading stdin, `--field` matched no events, or one event's `data:` lines passed 64 MiB |
| 2 | Usage error - no stdin when running interactively, unknown flag |

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

# Same, without tr: --raw writes values with no newline (OpenAI-style array path)
curl -N ... | vrk sse --field 'data.choices[0].delta.content' --raw

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

- **A blank line ends each event.** `sse` emits an event only when it reads a blank line. If stdin closes mid-event (no trailing `\n\n`), `sse` drops that event without a message. Real streaming servers always send the blank line.
- **`[DONE]` stops the stream, even with `--event`.** The `[DONE]` sentinel (used by Anthropic and OpenAI) is a protocol signal, not a data event. `sse` stops parsing at once, whatever the `--event` filter.
- **No stdin in a terminal exits 2.** If you run `vrk sse` interactively with no pipe, it exits 2. Pipe a stream or redirect a file.
- **Space stripping follows the SSE spec.** `sse` strips exactly one leading space from a field value after the colon: `data: hello` → `hello`, `data:  hello` → ` hello` (one stripped, one remains).
- **Multi-line `data:` fields join with `\n`.** Two `data:` lines in one event become `val1\nval2`. If the result is valid JSON, `sse` parses it. Otherwise `sse` keeps it as a raw string.
- **A `data:` line with no value adds an empty string** to the joined data. This follows the SSE spec.
- **Non-JSON `data:` values are emitted as strings**, not dropped. For example, `data: ping` appears as `{"event":"message","data":"ping"}`. `[DONE]` is the exception: `sse` treats it as the end of the stream and does not emit it.
- **`--field` skips records with a missing path or non-JSON data.** It prints no error and no output for them. Real streams mix JSON and non-JSON events. `sse` exits 1 only if no event in the whole stream matched.
- **One event is limited to 64 MiB of data.** If a server streams `data:` lines without the closing blank line, `sse` reaches the limit and exits 1 with `sse: event exceeds 64 MiB limit`. `sse` does not buffer events that `--event` will skip.
- **`--field` strips terminal control characters only when stdout is a terminal.** Piped output stays byte-exact, including with `--raw`.
- **Stdout is always empty on error.** Errors go to stderr only.

---

## kv - Persistent Key-Value Store

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
| 0 | `set`, `del`, `list`, `incr`, `decr` - success |
| 0 | `get` - key found and not expired |
| 1 | `get` - key not found or expired |
| 1 | `incr`/`decr` - stored value is not a parseable integer |
| 1 | any - database open or write failure |
| 2 | any - missing subcommand, unknown subcommand, unknown flag, `--by` < 1 |

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

- **Empty string is a valid value.** `vrk kv set mykey ""` stores an empty string. `get` then exits 0 and prints an empty line. A missing key is different: it exits 1.
- **Stdin value for `set`.** If you omit the value argument, `kv set` reads the value from stdin. It strips exactly one trailing newline. So `echo 'val' | vrk kv set key` and `vrk kv set key val` store the same value.
- **Namespaces are isolated.** `--ns a` and `--ns b` never share keys. The default namespace is `"default"`. If you set a key with `--ns`, pass the same `--ns` to `get`. Otherwise `get` reports "key not found".
- **`incr`/`decr` on a missing key start at 0.** The first `vrk kv incr counter` stores and prints `1`. The first `vrk kv decr counter` stores `-1`.
- **`incr`/`decr` on a non-numeric value exit 1.** The stored value must parse as a 64-bit integer. Float strings (`"1.5"`) and alphabetic strings fail with "value is not a number".
- **TTL precision is whole seconds.** `--ttl 1500ms` rounds up to 2 seconds, so a short TTL never expires instantly. A negative `--ttl` exits 2. The database file must be yours and must not be a symlink. `kv` sets its mode to 0600.
- **`list` output is sorted alphabetically.** The order is deterministic, so you can diff it in scripts. The output has no namespace prefix.
- **`del` on a missing key exits 0.** This matches filesystem `rm -f` semantics.
- **Concurrent writers are safe.** `kv` uses SQLite WAL mode with `PRAGMA busy_timeout=5000`. `incr` and `decr` use `BEGIN IMMEDIATE` to take the write lock before they read, so no update is lost. 10 parallel `vrk kv incr counter` processes always end at 10.
- **Do not store secrets in `kv`.** The database is plaintext SQLite at `~/.vrk.db`. Keep credentials in env vars or the system keychain.
- **Stdout is always empty on error.** Errors go to stderr only.

---

## coax - Retry Wrapper

Retries a command when it fails. `coax` supports fixed and exponential backoff,
retry on chosen exit codes (`--on`), and retry until a condition passes (`--until`).
`coax` buffers stdin and pipes it to the command again on every attempt.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--times N` | - | Number of retries (default 3); total attempts = N+1. Must be >= 1. |
| `--backoff <spec>` | - | Delay between retries: `100ms` for fixed, `exp:100ms` for exponential |
| `--backoff-max <d>` | - | Cap for exponential backoff; 0 = uncapped |
| `--on <code>` | - | Retry only when exit code matches; repeatable: `--on 1 --on 2`. Default: any non-zero exit. |
| `--until <cmd>` | - | Shell command; retry until it exits 0 (takes precedence over `--on`) |
| `--quiet` | `-q` | Suppress coax's own retry progress lines to stderr; subprocess stderr always passes through |

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Command exits 0 on some attempt, or `--until` condition exits 0 |
| last cmd code | All retries exhausted - coax passes through the last command's actual exit code |
| 2 | Usage error - `--times < 1`, no command, unknown flag, bad or negative `--backoff` or `--backoff-max` |

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
vrk coax -- sh -c 'exit 1'

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

- **`--` is required.** It separates coax flags from the retried command: `vrk coax --times 2 -- cmd args`. If there is no command, coax exits 2 with "missing command".
- **Commands run directly, not through a shell.** The args after `--` are the program and its arguments, exactly as given. If you need pipes, redirects or builtins (`exit`, `cd`), run a shell: `vrk coax -- sh -c 'curl -sf url | jq .data'`. A single quoted string like `vrk coax -- 'cmd1 && cmd2'` fails with "command not found". Only `--until` is a shell string.
- **Exit code passthrough.** When all retries fail, coax exits with the last command's exit code, not 1. `vrk coax --times 2 -- sh -c 'exit 42'` exits 42. A test for `$? -ne 0` still works. A test for one specific code must expect the command's code.
- **`--on` with no match exits immediately.** If you set `--on 42` and the command exits 1, coax stops after the first attempt. The filter is strict. It does not fall back to "retry on any non-zero".
- **`--until` takes precedence over `--on`.** If you set both, coax checks only the `--until` condition to decide whether to retry.
- **`--until` output is discarded.** Coax suppresses the stdout and stderr of the `--until` command. It is a check, not data.
- **Stdin is replayed on every retry.** Coax keeps up to 64 MiB for replay. If stdin is larger, a retry fails with `coax: stdin larger than 64 MiB cannot be replayed` (exit 1). A command that ignores stdin starts at once, even if the pipe stays open.
- **Missing command and signals.** If the command does not exist, coax exits 1 with `coax: command not found: <name>` and does not retry. Coax forwards SIGINT and SIGTERM to the command and stops further retries. A command killed by a signal gives exit 128 + signal number. When all retries fail, the last stderr line is `coax: all <N> attempts failed (last exit <code>)`.
- **`--quiet` suppresses only the `coax: attempt N failed` progress lines.** It never suppresses the stdout and stderr of the command.
- **Stdout is empty on error.** Coax writes its own error messages to stderr.

---

## prompt - LLM Call for Pipelines

Sends a prompt to an LLM and prints the response. Defaults to Anthropic
(`claude-sonnet-4-6`). Reads from stdin or a positional argument.
Input: positional argument or stdin.

### Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--model` | `-m` | string | `claude-sonnet-4-6` | Model name; overridden by `VRK_DEFAULT_MODEL` env var |
| `--endpoint <url>` | - | string | `""` | OpenAI-compatible API base URL; overrides provider detection |
| `--field <path>` | - | string | `""` | Dot-path field in each JSONL line to use as prompt text |
| `--budget <N>` | - | int | 0 | Exit 1 before calling the API if prompt exceeds N tokens (0 = disabled) |
| `--fail` | `-f` | bool | false | With `--schema`: exit 1 on mismatch. Without `--fail` or `--retry`, a mismatch only warns on stderr and exits 0 |
| `--json` | `-j` | bool | false | Emit response as a JSON envelope with metadata |
| `--schema <val>` | `-s` | string | "" | Inline JSON schema or file path; validates response keys and types |
| `--explain` | - | bool | false | Print equivalent curl command and exit 0; no API call |
| `--retry <N>` | - | int | 0 | Retry up to N times on schema mismatch with temperature escalation |
| `--system <val>` | - | string | `""` | System prompt text, or `@path` to read from file |

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success - response on stdout. Also a `--schema` mismatch without `--fail` or `--retry`: response is printed and `prompt: warning: response does not match schema` goes to stderr |
| 1 | Runtime error: no API key, HTTP error, budget exceeded, empty or whitespace-only prompt, schema mismatch with `--fail` or after all retries, invalid JSONL, field not found |
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
| - | No key and no endpoint (not `--explain`) | Exit 1: "no API key found" |

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

**Gotcha:** `VRK_LLM_KEY` is bound to `VRK_LLM_URL`. `prompt` sends it only to that origin (scheme, host, port). If `VRK_LLM_KEY` is set and `--endpoint` points to another origin, `prompt` exits 2. For a keyless local server, unset the key: `VRK_LLM_KEY= vrk prompt --endpoint ...`. Set both variables together: `VRK_LLM_URL=https://llm.example.com/v1 VRK_LLM_KEY=...`.

**Gotcha:** if a key would go over plain `http` to a host other than `localhost`, `127.0.0.1` or `::1`, `prompt` exits 2. Use https. `prompt` accepts only `http` and `https` endpoints. It never follows redirects (a 3xx exits 1).

**Gotcha:** `prompt` cuts API error bodies to 2 KB and strips control characters from them. With `--json`, `prompt` omits `system_prompt` when the prompt came from `--system @file`.

**Gotcha:** if the server returns a non-standard response shape, `prompt` exits 1 and puts the raw response body in the error. Use `--explain` to see the exact request.

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

- **`--json` means metadata envelope, not "respond in JSON".** To get JSON from the LLM, use `--schema`. `--json` wraps the response in `{response, model, prompt_tokens, response_tokens, elapsed_ms}`.
- **`--field` and `--explain` are mutually exclusive.** `--explain` shows one call, and `--field` makes many. If you set both, `prompt` exits 2.
- **`--field` stops at the first error.** If line 5 has invalid JSON, lines 1-4 are already on stdout. If you need all-or-nothing output, buffer the output and check the exit code.
- **`--field` overwrites `response` in input records.** If an input record has a field named `response`, `prompt` overwrites it and writes a warning to stderr.
- **`--budget` is a hard gate.** It runs before the API call, even if no API key is set. There is no warning mode. To gate the pipeline upstream, use `vrk tok --check N`.
- **Temperature default is 0.** Responses are deterministic by default. `--retry` raises the temperature by 0.1 per attempt, up to 1 (0.0 → 0.1 → 0.2 → ...). `prompt` has no `--temperature` flag.
- **API keys never appear in output.** `prompt` removes the key value from all error messages and from `--explain` output. `--explain` shows `$ANTHROPIC_API_KEY` or `$OPENAI_API_KEY` as a shell variable.
- **No conversation history.** Each call is stateless. For a multi-turn conversation, put the earlier turns in the prompt text.
- **`prompt` reads all input before the API call.** It waits for EOF. For very large input, check that the token count fits the context window of the model.
- **`--schema` checks the top level only.** It validates top-level keys and types (`string`, `number`, `boolean`, `array`, `object`). It does not validate nested structures.
- **A `--schema` mismatch is only a warning by default.** Without `--fail` or `--retry`, `prompt` still prints the response, writes a warning to stderr, and exits 0. This holds in `--field` mode and on Anthropic. Add `--fail` to exit 1. Add `--retry N` to retry first. `prompt` checks replies against the schema on Anthropic, OpenAI and custom endpoints.
- **`--schema` is sent differently per provider.** OpenAI gets a JSON Schema in `response_format` (every key required, `additionalProperties: false`; `strict` only when all types are scalar). Anthropic gets the schema as a system prompt. `--endpoint` and `VRK_LLM_URL` get it as a system message with `response_format: json_object`. In all cases `prompt` also validates the reply after the call.
- **An empty prompt is an error.** If the prompt is empty or only whitespace, `prompt` exits 1 with `prompt: empty prompt, nothing sent` and sends no request. With `--field`, `prompt` skips empty records with `line N: empty prompt, skipped`.
- **Stdout is empty on error.** Error messages go to stderr. Stdout is empty on exit 1 and exit 2.
- **`--system @file` paths are relative to the working directory** where you run `vrk`, not to the script. In CI, use absolute paths or paths relative to the repo root.

---

## chunk - Token-Aware Text Splitter

Splits text from stdin into chunks, each within a token budget. Emits JSONL.
Counts with o200k_base by default; `--model gpt-4` switches to cl100k_base (see tok).
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--size N` | - | Max tokens per chunk (required, must be >= 1) |
| `--overlap N` | - | Token overlap between adjacent chunks (default 0; must be < --size) |
| `--by <mode>` | - | Chunking strategy; only `paragraph` is currently supported |
| `--model <name>` | `-m` | Tokenizer, same values as `tok --model` (default `o200k_base`) |

### Output format

One JSONL record per chunk:

```json
{"index":0,"text":"...","tokens":998}
{"index":1,"text":"...","tokens":1000}
{"index":2,"text":"...","tokens":312}
```

- `index`: 0-based, sequential across all emitted records
- `text`: decoded token window - may split mid-word at token boundaries, but never splits a UTF-8 character (see Gotchas)
- `tokens`: exact token count for the emitted text in the chosen encoding; stays within `--size` except when a single character is wider than the window (then up to 3 tokens over)

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty input - no records emitted) |
| 1 | Runtime error - I/O error reading stdin, tokenizer failure |
| 2 | Usage error - `--size` absent or < 1, `--overlap` >= `--size`, unknown `--by` mode, unknown `--model`, unknown flag |

### Splitting modes

**Default (no `--by`):** Sliding window over token IDs. Step = `size - overlap`. Each window is decoded back to text. Window edges move to a UTF-8 character boundary, so `tokens <= size` holds except for a single character wider than `--size` (at most 3 tokens over).

**`--by paragraph`:** Splits text at double-newlines (`\n\n`), then greedily packs paragraphs into chunks, joined with a blank line (`\n\n`) so the separators are kept in `text`. A paragraph that exceeds `--size` tokens falls back to token-level splitting. Overlap is applied by prepending the last `--overlap` token IDs from the previous chunk at the start of the next.

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

- **`--size` is required.** It has no default. If you omit it, `chunk` exits 2 with `chunk: --size is required`.
- **Chunks may split mid-word.** The default mode works on token IDs. Token boundaries do not always match word boundaries, so a chunk can end inside a word. `chunk` emits the decoded text as-is. It never moves a boundary, because that would change the token count and could break the `--size` limit.
- **`--by paragraph` splits on a double newline and keeps it.** `chunk` joins packed paragraphs with `\n\n` in `text`. A single newline is not a paragraph break. If your text uses single newlines between paragraphs, convert them with `sed` or `awk` before you pipe to `chunk`.
- **`--by paragraph` splits oversized paragraphs by token.** If a paragraph is over `--size` tokens, `chunk` splits it at token boundaries, not at sentence or word boundaries. It can split mid-word.
- **`--overlap` counts tokens, like `--size`.** It is not a percentage.
- **A large `--overlap` multiplies the output.** Each chunk advances `size - overlap` tokens, so the output is about `size / (size - overlap)` times the input. `--size 1000 --overlap 999` on 200 KB writes about 195 MB. Records stream out one at a time, so memory stays flat. Keep overlap well under half of `--size` unless you need more.
- **Empty input exits 0 with no output.** `printf '' | vrk chunk --size 1000` emits nothing and exits 0. Empty input is not an error.
- **The `tokens` field is exact.** It is the real token count of the emitted `text`. Use it for budget checks downstream.
- **OpenAI tokenizers are an estimate for Claude.** Set `--size` at 90% of the model's actual context limit to absorb the error margin.
- **Stdout is empty on error.** Errors go to stderr.

---

## grab - URL Fetcher

Fetches a URL and returns clean markdown (default), plain text, or raw HTML.
Applies Readability-style content extraction - strips navigation, ads, boilerplate.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--text` | `-t` | Plain prose output, no markdown syntax |
| `--raw` | - | Raw HTML, no processing or extraction |
| `--json` | `-j` | Emit a JSON envelope with metadata |
| `--quiet` | `-q` | Suppress stderr output |
| `--max-size` | - | Max response body size in bytes (default 10MB) |
| `--allow-internal` | - | Allow requests to private/loopback/link-local addresses (needs `VRK_ALLOW_INTERNAL=1`) |

`--text`, `--raw`, and `--json` are mutually exclusive. Combining any two exits 2.
Requests to internal network addresses (loopback, private, CGNAT 100.64.0.0/10, link-local, reserved, and NAT64/6to4 addresses that wrap one of those) are blocked by default. Pass `--allow-internal` with `VRK_ALLOW_INTERNAL=1` in the environment to allow them.

### --json output shape

```json
{"url":"https://example.com","title":"Example Domain","content":"# Example Domain\n\n...","fetched_at":1740000000,"status":200,"token_estimate":180}
```

- `url`: final URL after any redirects
- `title`: text content of `<title>` element (empty string if absent)
- `content`: extracted content in the active mode (markdown by default)
- `fetched_at`: Unix timestamp (integer seconds) of when the response was received
- `status`: HTTP status code (always 200 on exit 0)
- `token_estimate`: o200k_base token count of `content` (an estimate for Claude)

### Exit codes

| Code | Condition |
|------|-----------|
| 0 | Success - output on stdout |
| 1 | Fetch failed: DNS error, connection refused, timeout |
| 1 | HTTP status >= 400 |
| 1 | More than 5 redirect hops |
| 1 | Response body exceeded `--max-size` limit |
| 1 | Request to internal address blocked (without `--allow-internal`) |
| 2 | No URL provided (interactive terminal or empty stdin) |
| 2 | `--allow-internal` without `VRK_ALLOW_INTERNAL=1` in the environment |
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

- **Extraction is best-effort, not full Readability.** `grab` picks content with a simple scoring heuristic. Pages with semantic HTML (`<main>`, `<article>`, `<p>`) extract cleanly. Complex layouts (SPAs, dashboards, heavily nested tables) can add noise or lose content.
- **JavaScript is not executed.** `grab` makes a static HTTP request. If a page renders its content in the browser with JavaScript, `grab` returns the HTML skeleton, not the final content.
- **An invalid URL is a usage error (exit 2), not a runtime error (exit 1).** A URL with no scheme (`example.com`) or a non-http/https scheme (`ftp://`) exits 2. A valid URL that fails to fetch (DNS, timeout, 404) exits 1.
- **Non-HTML responses pass through unchanged.** If the server returns `Content-Type: application/json` or any other non-`text/html` type, `grab` writes the raw body to stdout without extraction. `--text` has no effect on non-HTML. `--json` still wraps the raw body in the envelope.
- **`grab` follows at most 5 redirects.** A longer chain exits 1 with "too many redirects (> 5)". `grab` never stores cookies between redirects or runs.
- **`token_estimate` uses o200k_base.** It is exact for GPT-4o and later, and an estimate for Claude. It counts the extracted `content`, not the raw HTML. Set downstream budgets at 90% of the model limit to allow for the error.
- **`--text` normalises whitespace.** `grab --text` runs the HTML extractor, then the markdown stripper of `vrk plain`. It collapses consecutive blank lines to one and multiple spaces to one. The prose is unchanged, but the whitespace can differ from the page.
- **Stdout is empty on error.** Error messages go to stderr. Stdout is empty on exit 1 and exit 2.
- **Internal addresses are blocked by default.** Requests to 127.0.0.0/8, 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16, 100.64.0.0/10, 169.254.0.0/16, other reserved ranges, ::1, ULA and link-local IPv6, and NAT64 (64:ff9b::/96) or 6to4 (2002::/16) addresses that embed one of those, exit 1 unless you pass `--allow-internal`. This includes addresses reached by redirect. A public URL that redirects to localhost is also blocked.
- **`--allow-internal` needs `VRK_ALLOW_INTERNAL=1`.** The flag alone exits 2. A model can add a flag to a command line, so the person who runs vrk must opt in through the environment.
- **Credentials in the URL are not echoed.** A password in `https://user:pass@host/` shows as `***` in errors and in the `--json` `url` field. `grab` does not copy it into resolved links.
- **Terminal control characters are stripped.** `grab` removes escape sequences from page text and titles. A non-HTML body passes through byte for byte in a pipe. `grab` strips it only when stdout is a terminal.
- **Redirects drop the `Referer` header.** A token in the query string of the first URL does not reach the redirect target.
- **The response size limit is 10MB by default.** Use `--max-size` to change it. If a response is over the limit, `grab` exits 1.

---

## plain - Markdown Stripper

Strips markdown formatting from stdin or a positional argument. Preserves all content - only syntax is removed.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | `-j` | JSON envelope: `{"text":"…","input_bytes":N,"output_bytes":M}` |
| `--quiet` | `-q` | Suppress stderr output |

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

- **`plain` uses goldmark's AST** - it handles nested emphasis, reference-style links, and fenced code blocks correctly. Character-level regex strippers do not.
- **Raw HTML embedded in markdown is dropped silently.** Block-level `<div>` and inline `<span>` tags and their content are not extracted. For HTML-heavy input, use `grab --text` instead (which runs the HTML extractor first, then `StripMarkdown`).
- **Output whitespace may differ from input.** Consecutive blank lines are collapsed to one. Leading and trailing whitespace is trimmed.
- **Deeply nested input comes back as plain text.** More than 1000 back-to-back nesting openers (`[a](`, `- `, `> `) is not real Markdown and makes the parser quadratic, so the text is returned with only whitespace cleanup. Any other input that takes longer than 10 seconds to parse exits 1 with `plain: markdown too complex to parse within 10s`.

---

## links - Hyperlink Extractor

Extracts all hyperlinks from markdown, HTML, or plain text as JSONL.
One record per link: `{"text":"...","url":"...","line":N}`.
Input: positional argument or stdin. Empty input exits 0 with no output.

### Record shape

```
{"text":"Homebrew","url":"https://brew.sh","line":1}
```

`text` is the anchor text (Markdown or HTML) or the URL itself for bare URLs.
`line` is the 1-based line number in the input.
Terminal control characters (ESC, BEL, C1, bidi overrides) are dropped from `text` and `url`, so a hostile page can't smuggle OSC 52 or OSC 8 sequences through `--bare`.

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
| `--quiet` | `-q` | Suppress stderr output |

### --json output shapes

| Flags | Shape |
|-------|-------|
| `--json` alone | link records + `{"_vrk":"links","count":N}` |
| `--bare --json` | bare URL lines + `{"_vrk":"links","count":N}` |
| Any error + `--json` | `{"error":"msg","code":N}` on stdout; stderr empty |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success - links found or not, including empty input |
| 1 | I/O error reading stdin |
| 2 | Usage error - interactive terminal with no stdin, unknown flag |

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

- **Relative URLs are emitted as-is** - `href="/about"` produces `{"url":"/about",...}`. The caller knows the base URL; `links` does not resolve relative hrefs.
- **Markdown ref definitions are not emitted as links** - `[label]: url` lines are collected internally but only appear in output when a `[text][label]` usage references them. The emitted `line` is the line of the usage, not the definition.
- **Multi-line `<a>` tags are not supported** - the tool processes line by line. An `<a href="...">` that spans multiple lines will not be matched.
- **No deduplication** - the same URL appearing twice produces two records. Pipe through `sort -u` or `jq -r '.url'` if you need unique URLs.
- **Empty stdin exits 0 with empty output** - this is not an error. Only an interactive terminal (no pipe) exits 2.

---

## validate - JSONL Schema Validator

Validates JSONL records against a simplified type schema. Valid lines pass through
to stdout unchanged. Invalid lines are warned to stderr and skipped (or cause exit 1
with `--strict`). Input: stdin only (no positional argument - schema comes from `--schema`).

### Schema format

`{"key":"type"}` map. Supported types: `string | number | boolean | array | object`.
Schema keys are **required fields** - a record missing a schema key is invalid.
Extra keys in the record that are not in the schema are **ignored** (subset check).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--schema <spec>` | `-s` | Inline JSON schema or file path (required) |
| `--strict` | - | Exit 1 on first invalid line (no shorthand - `-s` is taken) |
| `--fix` | - | Attempt to repair invalid lines via `vrk prompt` before re-validating |
| `--json` | `-j` | Append metadata record `{"_vrk":"validate","total":N,"passed":N,"failed":N}` at end |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | All lines valid, OR invalid lines found but `--strict` not set |
| 1 | `--strict` and at least one invalid line; or I/O error (with `--json`: error record to stdout) |
| 2 | `--schema` missing or invalid, unknown schema type, a JSON Schema instead of a flat `{"key":"type"}` map, unknown flag |

### Examples

```bash
# Validate a single record
echo '{"name":"alice","age":30}' | vrk validate --schema '{"name":"string","age":"number"}'
# → {"name":"alice","age":30}  (stdout, exit 0)

# Invalid record — warned to stderr, nothing on stdout
echo '{"name":"alice","age":"wrong"}' | vrk validate --schema '{"name":"string","age":"number"}'
# → (empty stdout, stderr: "warning: validate: line 1: age expected number, got string", exit 0)

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

- **`--strict` has no shorthand.** `-s` is reserved for `--schema`.
- **`--fix` requires an API key.** It runs `vrk prompt` to repair invalid lines. If no API key is configured, the line stays invalid and `validate` writes a warning to stderr. A missing key never causes exit 2.
- **`--json` on empty input still emits the metadata record:** `{"_vrk":"validate","total":0,"passed":0,"failed":0}`.
- **With `--json` and `--fix`, repaired lines count as `passed`.** There is no `repaired` counter.
- **The schema is a subset check.** Extra keys in a record are not an error. `validate` checks only the keys in the schema.
- **Large integers are safe.** `validate` decodes records with `UseNumber()`, so integers above 2^53 keep full precision.
- **Empty lines are skipped.** They do not count toward `total`.
- **Each line must hold one JSON object.** A line with a second value (`{"a":1} {"a":2}`) is invalid. A line with valid JSON that is not an object (`null`, `[]`, `"x"`) is also invalid.
- **Schema errors do not echo values.** An unknown type names only the key, so a `--schema` file that holds secrets by mistake does not print them. A `--schema` file can be at most 1 MB.
- **`--strict` on zero records exits 0.** Empty input is not a failure. If you need at least one record, gate with `vrk assert`.

---

## mask - Secret Redactor

Redacts secrets from stdin with entropy-based detection and built-in patterns.
It writes the text with each secret replaced by `[REDACTED]`. It streams line by line, so large input is safe.
Redaction is best-effort and is not a security boundary.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--pattern <regex>` | - | Additional pattern; repeatable. Entire match → `[REDACTED]` |
| `--entropy <float>` | - | Shannon entropy threshold (default: 4.0; lower = more aggressive) |
| `--json` | `-j` | Append metadata record after text output |

### Built-in patterns

Applied before entropy scanning. For the keyword patterns the key prefix is preserved and the value is replaced.

| Name | Matches |
|------|---------|
| `bearer` | `Bearer <token>` (case-insensitive) |
| `password` | keys ending in `password`, `passwd` or `passphrase`, or a `pass` / `pwd` segment (`DB_PASS`, `db-pwd`) |
| `secret` | keys ending in `secret(s)` or `credential(s)`, or an `auth` segment (`basic_auth`) |
| `api_key` | keys ending in `api_key`, `apikey`, `access_key`, `private_key`, `secret_key`, `signing_key` (`X-API-Key`, `aws_secret_access_key`) |
| `token` | keys ending in `token` (`access_token`, `HF_TOKEN`, `_authToken`) |
| `auth_header` | `Authorization:` / `Proxy-Authorization:` with any scheme (`Basic`, `token`, `Digest`, ...) or none |
| `url_userinfo` | the password in `scheme://user:pass@host` (the user is kept) |
| `curl_user` | the password in `curl -u user:pass` / `--user user:pass` |
| `aws_key` | AWS `AKIA` / `ASIA` access key IDs |
| `openai_anthropic` | `sk-...` keys (20+ key characters) |
| `stripe` | `sk_live_`, `sk_test_`, `rk_live_`, `rk_test_` keys |
| `github` | `ghp_` / `gho_` / `ghu_` / `ghs_` / `ghr_` tokens and `github_pat_` |
| `slack` | `xoxb-`, `xoxp-`, `xoxa-`, ... tokens |
| `google` | `AIza...` API keys |
| `huggingface` | `hf_...` tokens |
| `npm` | `npm_...` tokens |
| `sendgrid` | `SG.<id>.<secret>` keys |
| `jwt` | `eyJ...` three-part JWTs |

Each keyword family matches these shapes, case-insensitive:

- `key=v`, `key: v`, `key = v`, and quoted keys and values: `"password": "v"`, `'token': 'v'`. A quoted value is redacted up to its closing quote (escaped quotes included) and the quotes stay, so JSON stays valid and a quoted passphrase with spaces goes completely.
- A YAML/INI/env line that starts with an unquoted key (`db_password: v`, `password = v`, `export PGPASSWORD=v`, `- token: v`) loses the rest of the line, so an unquoted multi-word passphrase goes too.
- `<password>v</password>` (a namespace prefix is allowed).
- `--password v` and `-token v` on a command line.

Only keys that end in the secret word count, so `"tokens": 42` (from `vrk tok --json`), `max_tokens` and `bypass` are left alone.

The key-prefix patterns (`aws_key` onward) replace only the key itself, apply anywhere in a line, and also catch keys inside URLs and paths, which entropy scanning skips.

### Entropy scanning

Tokens shorter than 8 characters are never entropy-checked. Tokens that already
contain `[REDACTED]` (from pattern substitution) are skipped. Shannon entropy is
calculated per whitespace-delimited token: `H = -sum(p * log2(p))`.

Random hex can't go above 4.0 bits per character, so runs of 32 or more hex characters are judged against 3.0 instead (or `--entropy`, if lower). That catches hex API keys and webhook secrets. A 40-character hex run on a line that mentions git (`commit`, `merge`, `sha`, `rev`, `ref`, `tree`, `parent`, `cherry-pick`, `revert`) is taken as a commit SHA and kept.

URLs without credentials, file paths, UUIDs, prefixed digests (`sha256:...`) and container image refs are not redacted by entropy, so logs keep their context. Each path segment and each secret-named query value (`api_key`, `token`, `sig`, ...) is still checked on its own. Punctuation around a redacted secret is kept: `key sk-..., please` gives `key [REDACTED], please`.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success - filter ran, text (redacted or not) written to stdout |
| 1 | I/O error with `--json` active - `{"error":"...","code":1}` written to stdout |
| 2 | Usage error - interactive terminal, unknown flag, invalid `--pattern` regex |

### `--json` output shape

Text lines are written first (streaming); the metadata record is appended after EOF:

```json
{"_vrk":"mask","lines":N,"redacted":N,"patterns_matched":["bearer","entropy"]}
```

- `lines`: total lines processed
- `redacted`: lines where ≥ 1 substitution was made
- `patterns_matched`: deduplicated list - built-in names (`"bearer"`, `"entropy"`, …) or literal regex strings for `--pattern` values

### `--json` error shape

```json
{"error":"<message>","code":1}
```

Written to stdout; stderr is empty. `Run()` returns 1. The envelope `code` and
`$?` always match - an agent can safely check either.

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

- **Best-effort only, not a security boundary.** Do not use `mask` as a compliance or audit control. Use it to reduce accidental exposure in logs and pipelines.
- **UUIDs, `sha256:...` digests, file paths and plain URLs are left alone** by entropy scanning. Other high-entropy tokens still count as secrets.
- **Hex digests get redacted.** `md5sum` / `sha256sum` output and other bare 32+ character hex runs look like hex keys. Keep a digest by writing it as `sha256:<hex>`, the form `vrk digest` prints. A 40-hex git SHA survives only on a line that mentions git (see Entropy scanning).
- **Strings under 16 characters cannot reach the 4.0 entropy threshold.** A short random password or key with no keyword or known prefix passes through. To catch those, lower `--entropy` or add a `--pattern`.
- **Line-form keys take the rest of the line.** `password = correct horse battery staple` becomes `password = [REDACTED]`. Anything after the value on that line is also removed.
- **Keyword patterns do not see JSON nested inside a JSON string.** In `"{\"password\": \"x\"}"` the escaped quotes hide the key. If your logs carry nested JSON, decode it first with `jq -r`.
- **URLs with a password skip the per-segment check.** In `https://u:pass@host/<secret>`, `mask` redacts the password but does not entropy-check the path segments.
- **Short passwords are below the 8-character token floor, so expect misses.** `mask` never entropy-checks `hunter2` (7 characters). The pattern `password=hunter2` still catches it. A bare short password with no keyword is not redacted.
- **`--pattern` replaces the entire match.** Built-in patterns keep the key prefix. `--pattern` replaces the whole regex match with `[REDACTED]`. To keep a prefix, write the pattern to match only the value, or use a lookahead.
- **`patterns_matched` lists `--pattern` values as literal regex strings.** The entry `"sk-ant-[A-Za-z0-9]+"` means that exact regex matched. It is not a named category.
- **Ordering in `patterns_matched`:** built-ins in declaration order (bearer → password → secret → api_key → token → auth_header → url_userinfo → curl_user → the key-prefix patterns), then `"entropy"`, then custom `--pattern` values in argument order.

---

## emit - Structured Logger

Wraps stdin lines as JSONL log records with timestamps and levels.
Input: positional argument or stdin (line-by-line streaming).
`emit` writes JSONL: every output line is a JSON object. It has no `--json` flag.

### Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--level <level>` | `-l` | `info` | Log level for all lines: debug, info, warn, error |
| `--tag <tag>` | - | - | Add `"tag"` field to every record |
| `--msg <msg>` | - | - | Override message; stdin treated as JSON to merge extra fields |
| `--parse-level` | - | - | Auto-detect level from line prefix; strip prefix from msg |

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

If you set `--msg`, `emit` tries to parse each stdin line as a JSON object. If the line is valid, `emit` merges its fields into the record after `msg`, sorted alphabetically.
`emit` ignores non-JSON lines and adds no extra fields. The record still has the `--msg` value.
Core fields (`ts`, `level`, `tag`, `msg`) in the stdin JSON are always dropped. The values from flags win.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | I/O error (scanner error, stdout write error) |
| 2 | Usage error - interactive stdin, unknown flag, invalid `--level` value |

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

- **No `--json` flag.** Every output line is already a JSON record. I/O errors go to stderr as plain text (exit 1).
- **Empty lines are skipped.** `emit` writes no record and no error. `echo '' | vrk emit` exits 0 with no output.
- **`--tag ""` omits the field.** It has the same effect as no `--tag`.
- **`--parse-level` with `--msg`:** `emit` still detects the level from the raw line prefix, and `--msg` sets the message. The two flags are independent.
- **Extra fields from the `--msg` JSON merge are sorted alphabetically,** so the order is predictable.
- **Large integers are preserved.** `emit` keeps the exact bytes (`json.RawMessage`), so a merge loses no float64 precision.
- **`WARNING` and `WARN` both map to level `"warn"`.** The output level is always the short form.

## throttle - Rate Limiter

Delays lines from stdin to match a rate limit. Use it to stay under API rate
limits when you call LLMs or other services in a loop. Input: stdin only.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--rate <N/s\|N/m>` | `-r` | Rate limit - required. N must be a positive integer |
| `--burst N` | - | Emit first N lines without delay, then rate-limit the rest |
| `--tokens-field <field>` | - | Rate by token count of a JSONL field value (dot-path) |
| `--json` | `-j` | Append `{"_vrk":"throttle","rate":"...","lines":N,"elapsed_ms":N}` after all lines |
| `--quiet` | `-q` | Suppress stderr output |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty input) |
| 1 | I/O error, token count failure, invalid JSON with `--tokens-field` |
| 2 | Usage error - missing `--rate`, rate ≤ 0, bad format, unknown flag, TTY stdin |

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

- **`--rate` is required.** If you omit it, `throttle` exits 2. There is no default rate.
- **N must be a positive integer.** `--rate 0.5/s` exits 2 with a "positive integer" error. For a rate under 1 per second, use `--rate 1/m`.
- **Empty lines are skipped.** An empty line (`""`) produces no output. A whitespace-only line (`"   "`) is content and passes through unchanged.
- **`--burst` counts lines, not tokens.** With `--tokens-field`, burst still counts lines.
- **`--tokens-field` on non-JSON input exits 1.** Use it only with JSONL. Bad JSON or a missing field is a fatal error, not a skip.
- **`elapsed_ms` is the wall time from the first line to the last line.** It includes all sleep time.

## jsonl - JSON Array ↔ JSONL Converter

Converts a JSON array to JSONL (one record per line) or collects JSONL lines into a JSON array.
Use it to connect tools that read JSON arrays with tools that read lines.
Input: positional argument or stdin.

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
| 2 | Usage error - interactive TTY with no stdin, unknown flag |

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

- **Empty stdin in split mode exits 0 with no output.** `printf '' | vrk jsonl` is not an error.
- **Empty stdin in collect mode outputs `[]`.** `printf '' | vrk jsonl --collect` prints `[]` and exits 0.
- **`--json` has no effect in collect mode.** The output is one JSON array, and a trailer after `]` would be invalid JSON.
- **Invalid JSON in collect mode exits 1 at once,** with the line number in the error message. `jsonl` writes no partial output.
- **Split mode streams.** It uses `json.Decoder`, so an array larger than memory is safe.
- **A round trip gives the same structure, not the same bytes.** `json.Marshal` sorts object keys alphabetically. `{"b":2,"a":1}` comes back as `{"a":1,"b":2}`.
- **`--collect` skips blank lines** between JSONL records.

## digest - Universal Hasher

Hashes stdin or files. SHA-256 by default. Outputs `algo:hash`. Supports HMAC for message authentication and file comparison.
Input: positional argument or stdin, or one or more `--file` paths.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--algo <name>` | `-a` | Hash algorithm: `sha256` (default), `md5`, `sha512` |
| `--bare` | `-b` | Output hash only - no `algo:` prefix; mutually exclusive with `--json` |
| `--file <path>` | none | File to hash; repeatable for multiple files |
| `--compare` | none | Compare hashes of all `--file` inputs; outputs `match: true` or `match: false`; exits 0 either way |
| `--hmac` | none | Compute HMAC instead of plain hash; requires one of `--key`, `--key-env`, `--key-file` |
| `--key <secret>` | `-k` | HMAC secret key. Visible in `ps` and shell history; prefer `--key-env` or `--key-file` |
| `--key-env <NAME>` | none | Read the HMAC key from environment variable `NAME`; unset or empty exits 2 |
| `--key-file <path>` | none | Read the HMAC key from a file; one trailing newline is dropped |
| `--verify <hex>` | none | Compare computed HMAC against this hex string or `sha256:<hex>` (the form digest prints); exits 0 on match, 1 on mismatch (constant-time). An empty value (`''` or `sha256:`) is a mismatch. A prefix naming a different algorithm than `--algo` exits 2. Not allowed with `--file` (exit 2) |
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
| 0 | Success - hash produced, `--compare` result (match or mismatch), `--hmac --verify` match |
| 1 | Runtime error - `--hmac --verify` mismatch (including an empty `--verify`), file not found, I/O error |
| 2 | Usage error - interactive terminal with no `--file`; unknown flag; unknown `--algo`; `--hmac` without a key; more than one of `--key`/`--key-env`/`--key-file`; `--verify` without `--hmac`; `--verify` with `--file`; `--compare` with fewer than 2 `--file` values; `--bare` + `--json` together |

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
# --key-env keeps the secret out of argv (ps, shell history, agent transcripts)
echo 'payload' | vrk digest --hmac --key-env WEBHOOK_SECRET
HMAC=$(echo 'payload' | vrk digest --hmac --key-env WEBHOOK_SECRET --bare)
echo 'payload' | vrk digest --hmac --key-env WEBHOOK_SECRET --verify "$HMAC"
# → exit 0 (match); exit 1 on mismatch

# Pipeline: hash → store
echo 'hello' | vrk digest | vrk kv set last_hash
```

### Gotchas

- **`echo` appends a newline.** `echo 'hello' | vrk digest` hashes `"hello\n"` (6 bytes), not `"hello"` (5 bytes). To hash a string without a newline, use one of these:
  ```bash
  vrk digest 'hello'          # positional arg - no newline added
  printf 'hello' | vrk digest # printf does not append \n
  ```
- **`--file` and `printf` stdin give identical hashes.** Both hash the bytes unchanged. `echo` stdin and `--file` differ if the file does not end with `\n`.
- **`--compare` exits 0 for both a match and a mismatch.** Stdout says `match: true` or `match: false`. To detect a mismatch, read stdout, not the exit code.
- **`--verify` compares in constant time** with `hmac.Equal`, which prevents timing attacks. Do not compare HMACs with `==`.
- **`--verify` fails closed.** If `SIG` is unset, `--verify "sha256:$SIG"` is a mismatch (exit 1, nothing on stdout), so a request with a dropped signature header is rejected. `--verify` checks stdin or a positional argument only. With `--file` it exits 2. Pipe the file in instead: `vrk digest --hmac -k "$K" --verify "$SIG" < body`.
- **`--bare` and `--json` are mutually exclusive.** If you combine them, `digest` exits 2.
- **`input_bytes` counts the bytes that `digest` hashed.** For `echo 'hello'`, `input_bytes` is 6 (the newline counts).
- **MD5 is available but is not secure.** Use it only to match existing MD5 checksums.

---

## base - Encoding Converter

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
| `--to <encoding>` | - | Target encoding for `encode`: `base64`, `base64url`, `hex`, `base32` |
| `--from <encoding>` | - | Source encoding for `decode`: same set |
| `--quiet` | `-q` | Suppress stderr; exit codes unaffected |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty input) |
| 1 | Runtime error - invalid input data for the chosen decoding |
| 2 | Usage error - no subcommand, missing `--to`/`--from`, unsupported encoding name, unknown flag, interactive TTY with no input |

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

- **`base` strips one trailing newline from stdin before encoding.** `echo 'hello' | vrk base encode --to hex` encodes `hello` (5 bytes), not `hello\n` (6 bytes). To encode the newline, use `printf 'hello\n'`. `vrk digest` differs on purpose: it does not strip the newline.
- **Line-wrapped base64 is fine.** Output from `base64 -w76` decodes as is; newlines inside the input are ignored.
- **base64url has no padding and uses a URL-safe alphabet.** The output uses `-` and `_` instead of `+` and `/`, and has no trailing `=`. JWT headers and URL query parameters use this format. Padding is strict both ways: `--from base64` needs the `=`, `--from base64url` rejects it.
- **hex output is lowercase.** `echo 'hello' | vrk base encode --to hex` produces `68656c6c6f`, not `68656C6C6F`. `--from hex` accepts upper or lower case.
- **base32 output is uppercase.** Decoding is case-sensitive; lowercase base32 input exits 1.
- **Decoded output has no trailing newline.** `echo aGVsbG8= | vrk base decode --from base64` prints `hello` with nothing after it.
- **`--quiet` does not suppress the no-subcommand error.** That error happens before flag parsing. `--quiet` works only inside a subcommand.

---

## recase - Naming Convention Converter

Converts text between naming conventions. It detects the input convention from
separators and casing. It reads stdin line by line: one line in, one line out.
Input: stdin (streaming, line by line).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--to <convention>` | - | Target naming convention (required): `camel`, `pascal`, `snake`, `kebab`, `screaming`, `title`, `lower`, `upper` |
| `--json` | `-j` | Emit a JSON object per line: `{"input":"…","output":"…","from":"…","to":"…"}` |
| `--quiet` | `-q` | Suppress stderr; exit codes unchanged |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty stdin) |
| 1 | I/O error reading stdin |
| 2 | Usage error - `--to` missing or unknown, unknown flag, interactive TTY with no input |

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
cat vars.txt | vrk recase --to camel
```

### Gotchas

- **`lower` and `upper` are prose transforms, not identifier formats.** The output is space-separated: `hello_world → lower` gives `"hello world"`, not `"hello_world"`. For identifiers, use `snake` or `kebab`.
- **A single word with no separators and no case changes counts as one word.** `helloworld → camel` stays `helloworld` because there is no word boundary. To get `helloWorld`, write `hello_world`.
- **Digits are not word boundaries.** `oauth2` is one token; `base64url` is one token. `oauth2 → pascal` = `Oauth2`.
- **Two consecutive acronyms with no separator cannot be split.** `getHTTPSURL → snake` produces `get_httpsurl`, not `get_https_url`. `recase` needs a lowercase letter to see where one acronym ends and the next begins. Put `-` or `_` between every word, for example `get-https-url`. A mixed form like `getHTTPS_URL` still fuses into `gethttps_url`.
- **Empty lines are preserved.** A blank input line gives a blank output line, so line counts stay the same.
- **Detection priority:** separators (`_`, `-`, ` `) win over case. `recase` always detects `hello_world` as `snake`.

---

## slug - URL/Filename Safe Slug Generator

Converts text to URL/filename-safe slugs. Lowercase, hyphen-separated. Latin text folds to ASCII; other scripts are kept. Exactly one output line per input line; an empty slug (all-punctuation line) prints an empty line and warns `line N: empty slug` on stderr.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--separator <s>` | - | Word separator (default: `-`). Any string, including multi-character or empty. |
| `--max <n>` | - | Max output length in characters (0 = unlimited). Truncates at last word boundary at or before `n`; never splits a character. |
| `--ascii` | - | Strict ASCII output; other scripts are dropped |
| `--json` | `-j` | Emit `{"input":"...","output":"..."}` per input line (JSONL). Empty-slug lines are emitted with `"output":""`. |
| `--quiet` | `-q` | Suppress stderr; exit codes unchanged. |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success (including empty output) |
| 1 | I/O error reading stdin |
| 2 | Usage error - interactive terminal with no stdin, unknown flag |

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

- **`--max` cuts at a word boundary. If the first word is longer than `--max`, the output is empty.** For example, `echo 'helloworld' | vrk slug --max 3` prints an empty line (and the `empty slug` warning), not `hel`. Use a value larger than your longest word.
- **Every input line produces exactly one output line.** A line with no letters or digits prints an empty line and warns `line N: empty slug` on stderr. Filter with `grep .` if you want blanks gone.
- **Non-Latin scripts are kept.** `日本語 abc` gives `日本語-abc`. Latin text folds to ASCII (`é → e`, `Straße → strasse`, `æ → ae`, `ø → o`). Use `--ascii` for strict ASCII output; other scripts are then dropped.
- **`--separator` accepts any string, including an empty one.** `--separator ''` joins words with no separator (`hello-world → helloworld`). Multi-character separators such as `--separator --` also work.

---

## Breaking changes

### digest - stdin no longer strips trailing newline (streaming fix)

Previously, `echo 'foo' | vrk digest` hashed `"foo"` (3 bytes).
After this change it hashes `"foo\n"` (4 bytes, the literal pipe content).

This aligns stdin with `--file` behaviour (which never stripped bytes) and
fixes an OOM crash on binary pipes. `printf 'foo' | vrk digest` and
`vrk digest foo` are unaffected - neither ever produced a trailing newline.

To keep the old hash, replace `echo 'value' |` with `printf 'value' |`.

---

## moniker - Memorable Name Generator

Generates adjective-noun names for run IDs, job labels and temporary
directory names, like Docker container names.
Input: none (generates names from embedded wordlists; stdin is ignored).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--count` | `-n` | Number of names to generate (default: 1) |
| `--separator` | - | Word separator (default: `-`) |
| `--words` | - | Number of words per name, minimum 2 (default: 2) |
| `--seed` | - | Fix random seed for deterministic output |
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
| 2 | Usage error - `--count 0` or over 1000000, `--words < 2`, unknown flag |

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

- **`moniker` ignores stdin.** It builds names from embedded wordlists. Input you pipe in has no effect, so you can place it anywhere in a pipeline.
- **`--seed 0` is a valid seed.** It is not the same as no seed. Without `--seed`, each run uses a random seed. With `--seed 0`, the output is deterministic.
- **`--count 1000` gives 1000 unique names.** The 2-word pool has 87,000+ entries. If you ask for more than the pool size, `moniker` exits 1 with a message.
- **`--json` always emits a `words` array.** The shape `{"name":"...","words":[...]}` is the same for every `--words` value. You can read `words[0]`, `words[1]` and so on without checking the word count.
- **The separator does not affect uniqueness.** Uniqueness holds at the word-index level. A different `--separator` gives different strings from the same combinations.

---

## pct - Percent Encoder/Decoder

Encodes and decodes percent-encoding per RFC 3986. Each input line gives one output line.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--encode` | - | Percent-encode input (RFC 3986 strict) |
| `--decode` | - | Percent-decode input |
| `--form` | - | Form encoding mode: spaces↔`+` instead of `%20` |
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
| 2 | Usage error - neither or both mode flags set, unknown flag, interactive terminal |

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

- **Double encoding is correct.** `echo 'hello%20world' | vrk pct --encode` gives `hello%2520world`, because `%` becomes `%25` (RFC 3986). To avoid double encoding, decode first.
- **`+` is a literal character in decode without `--form`.** `echo 'hello+world' | vrk pct --decode` gives `hello+world`. To read `+` as a space, use `--form`.
- **`%2B` decodes to `+` in both modes.** `echo 'hello%2Bworld' | vrk pct --decode` gives `hello+world`.
- **`~` is never encoded.** It is an RFC 3986 unreserved character, so `a~b` stays `a~b` with or without `--form`.
- **`--encode` and `--decode` are mutually exclusive.** If you pass both, `pct` exits 2.

---

## urlinfo - URL Parser

Parses a URL string into its components as JSON. It makes no network calls. To fetch a URL, use `grab`. To inspect one, use `urlinfo`.
Input: positional argument or stdin (single line or multiline batch).

### Record shape

```json
{"scheme":"https","host":"api.example.com","port":0,"path":"/v1/users","query":{"limit":"10","page":"2"},"fragment":"","user":""}
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
| 0 | Success - URL parsed (including scheme-relative `//host/path`) |
| 1 | Invalid URL - both scheme and host are empty after parsing |
| 2 | Usage error - interactive terminal with no input, unknown flag |

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

- **`urlinfo` never prints the password,** not even in error messages. An invalid URL is echoed with `***@` in place of its userinfo. To validate credentials, inspect the raw string yourself.
- **`--field port` returns an empty string when the URL has no explicit port.** `https://example.com` has no port in the string, even though HTTPS implies 443.
- **`urlinfo` accepts any string that Go's `url.Parse` accepts.** The parser is lenient. A string with no scheme parses without error and gives an empty scheme. `urlinfo` exits 1 only when both scheme and host are empty.
- **`--field query.<key>` returns an empty string if the key is absent.** It is not an error.
- **Query keys are sorted and the first value wins.** `?a=1&a=2` gives `"query":{"a":"1"}`.
- **`--field` suppresses the `--json` metadata trailer.** `urlinfo` prints only the field values.
- **Empty input exits 0 with no output.** `printf '' | vrk urlinfo` prints nothing and exits 0.

---

## sip - Stream Sampler

Samples lines from stdin with one of four strategies. It handles streams of any size. Reservoir sampling uses memory in proportion to the sample size N, not the stream size.
Input: stdin only. `sip` ignores a positional argument and does not report an error.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--first N` | - | Take first N lines (deterministic) |
| `--count N` | `-n` | Reservoir sample of exactly N lines (random, O(N) memory) |
| `--every N` | - | Emit every Nth line (deterministic) |
| `--sample N` | - | Include each line with N% probability (approximate, random) |
| `--seed N` | - | Fix random seed for deterministic output; 0 is a valid seed value |
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
| 0 | Success (including sample larger than population - emit all, no error) |
| 1 | I/O error reading stdin |
| 2 | Usage error - no strategy flag, multiple strategy flags, invalid flag value, interactive terminal |

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

- **`--sample` is probabilistic.** `--sample 10` on 1000 lines gives about 100 lines, not exactly 100. For an exact count, use `--count`.
- **`--seed 0` is valid.** Seed 0 does not mean "not set".
- **Reservoir output keeps the input order.** `--count 100` on a 1000-line stream emits 100 lines in the order they appeared in the input.
- **Empty lines are skipped and not counted.** An empty line (`""`) is ignored by every strategy and is not in `total_seen`.
- **`--first` and `--every` still read the whole stream.** `total_seen` counts all non-empty lines, even for `--first N`.
- **No positional argument.** `sip` is a stream filter. Input must come from a pipe.

---

## assert - Pipeline Condition Check

Evaluates conditions on stdin. If all pass, it passes the data through (exit 0). If one fails, it stops the pipeline (exit 1).
Use it between two tools to check agent output.
Input: stdin only (pipe).

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `<condition>` | - | jq-compatible condition (positional, repeatable; all must pass) |
| `--message <msg>` | `-m` | Custom failure message shown in stderr and `--json` output |
| `--contains <str>` | - | Plain text: assert stdin contains substring (case-sensitive) |
| `--matches <regex>` | - | Plain text: assert stdin matches Go regex |
| `--json` | `-j` | Emit `{"passed":bool,...}` to stdout; exit codes unchanged |
| `--quiet` | `-q` | Suppress stderr on failure; exit codes unchanged |

`--contains` and `--matches` can be combined (both must pass). Both are
mutually exclusive with positional jq conditions.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Assertion passed - stdin passed through byte-for-byte |
| 1 | Assertion failed, JSON parse error, empty input in JSON mode, condition timed out, or I/O error |
| 2 | Usage error - no condition, no stdin, mode conflict, invalid regex |

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
# → {"condition":".status == \"ok\"","input":{"status":"ok"},"passed":true}
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
| Fail with a reason (empty input, timeout, eval error) | adds `"error":"<reason>"` to the fail object |
| Pass (plain text) | `{"passed":true,"condition":"--contains: <str>"}` |
| Fail (plain text) | `{"passed":false,"condition":"--contains: <str>","message":"..."}` |
| Error | `{"error":"assert: ...","code":2}` |

### Gotchas

- **Byte-for-byte output.** On success, stdin reaches stdout unchanged. `assert` does not reformat it or change newlines.
- **JSONL streaming.** For multi-line JSON input, `assert` evaluates each line on its own. At the first failing line, it exits 1. Lines already on stdout stay there.
- **Positional conditions need valid JSON.** If the input is not valid JSON, `assert` exits 1. For plain text, use `--contains` or `--matches`.
- **`--contains` and `--matches` read all of stdin as one string.** They do not work line by line.
- **jq truthiness.** For non-boolean gojq results, `null` and `false` are falsy. Everything else is truthy, including `0`, an empty string and an empty array.
- **`.field > 0` on a missing field** compares `null > 0`, which is false. If the field may be absent, check that it exists first.
- **`--quiet` suppresses stderr only.** It does not suppress `--json` output, which goes to stdout.
- **With `--json`, a failure has a `message` field only if you set `--message`.**
- **One JSON value per line.** `{"ok":true}{"ok":false}` on one line is a parse error (exit 1). It does not pass on the first value.
- **Empty input fails.** With a jq condition and no JSON records on stdin, `assert` exits 1. A gate with nothing to check does not pass.
- **Conditions time out after 5s.** A runaway expression like `last(range(1e18))` exits 1 with `condition timed out after 5s`. Memory is not limited, so do not pass untrusted conditions.

---

## pdf - PDF Text Extractor

Extracts the text layer of a PDF as Markdown by default: bigger or bold lines become `#` headings, bullet glyphs become `- ` items, aligned rows become tables, paragraphs are split by blank lines. Running headers, footers and page numbers are dropped, and words hyphenated across lines are joined (`--as-printed` turns both off). `--text` gives plain text instead. Pages are separated by form feeds (`\f`).
With `--json`, one record per page: `{"page":N,"text":"..."}`, then `{"_vrk":"pdf","count":N}`.
Input: a file path as the positional argument, or PDF bytes on stdin. Empty stdin exits 0 with no output.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--text` | `-t` | Plain text instead of Markdown, `\f` straight after each page |
| `--pages` | `-p` | Pages: `N`, `N-M`, `N-` (to the end), or a comma list like `1,3,5-7`. 1-indexed |
| `--json` | `-j` | One `{"page":N,"text":"..."}` per page, then `{"_vrk":"pdf","count":N}` |
| `--as-printed` | | Keep running headers, footers, page numbers and end-of-line hyphens. Use it to cite printed page numbers, or when the cleanup drops a line it shouldn't. Works with `--text` and `--json` |
| `--quiet` | `-q` | Suppress stderr output |

### --json output shapes

| Flags | Shape |
|-------|-------|
| `--json` | page records + `{"_vrk":"pdf","count":N}` |
| `--json --pages 2-3` | records for pages 2 and 3 (real page numbers) + trailer with `count:2` |
| Any error + `--json` | `{"error":"msg","code":N}` on stdout; stderr empty |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, including empty stdin |
| 1 | File not found, not a PDF, encrypted, corrupt, page out of range, no extractable text |
| 2 | Usage error - invalid `--pages`, more than one file, unknown flag, interactive terminal with no input |

### Examples

```bash
# Extract all text from a file, as Markdown
vrk pdf report.pdf

# Plain text, no headings or list markers
vrk pdf --text report.pdf

# Same thing from stdin
cat report.pdf | vrk pdf

# Only page 3, or pages 2 through 5, or page 4 to the end
vrk pdf --pages 3 report.pdf
vrk pdf --pages 2-5 report.pdf
vrk pdf -p 4- report.pdf

# A list: pages come out in the order given, repeats dropped
vrk pdf --pages 1,3,5-7 report.pdf

# One JSONL record per page
vrk pdf --json report.pdf

# Split plain output into pages
vrk pdf --text report.pdf | awk 'BEGIN{RS="\f"} {print NR": "length($0)" chars"}'
```

### Compose patterns

```bash
# Count tokens in the text of a PDF
vrk pdf report.pdf | vrk tok

# Per-page token counts (skip the trailer with select(.page))
vrk pdf --json report.pdf | jq -c 'select(.page)' | while read -r rec; do
  printf '%s %s\n' "$(jq -r .page <<<"$rec")" "$(jq -r .text <<<"$rec" | vrk tok)"
done

# Chunk a PDF for RAG
vrk pdf report.pdf | vrk chunk --size 1000

# Summarise one page with an LLM
vrk pdf --pages 1 report.pdf | vrk prompt --system 'Summarise this page in two sentences.'

# Scanned PDFs exit 1, so the pipeline stops before reaching the model
vrk pdf scan.pdf | vrk prompt --system 'Summarise.' || echo "no text layer, needs OCR"
```

### Gotchas

- **The argument is a path, not content.** Unlike most vrk tools, `vrk pdf 'text'` opens a file called `text`. Pipe the bytes on stdin or pass a path.
- **Text layer only, no OCR.** A scanned PDF (images of pages) exits 1 with `pdf: no extractable text found; document may be scanned (OCR required)`. If only the pages you pick with `--pages` are blank, the message is `pdf: no extractable text on the selected pages`.
- **Spacing is rebuilt from glyph positions.** Word gaps written as kerning (LaTeX, arXiv) become spaces. Baseline jumps become newlines. Text comes out in content-stream order, which is the reading order for LaTeX and most two-column papers. If a page is written row by row across two columns, `pdf` puts the left column first, but only when no text crosses the gutter. A full-width title above row-ordered columns can still interleave. Figures and tables appear where the stream puts them. Math can lose its spacing.
- **Headings are guessed from font size and weight.** A run of lines clearly bigger than the body text, followed by smaller text, becomes a heading. The largest size is `#`, the next `##`, and the rest `###`. A short bold line at body size (like `3.2.1 Scaled Attention`) followed by a full line of plain text is one level below the smallest sized heading. Big equations can become headings. If that matters, use `--text`.
- **Tables are guessed from wide gaps.** Two or more rows become a Markdown table if gaps wider than two ems split their cells and the same number of cells start at the same x. The first row is the header. A row with a merged or empty cell splits the table. Cells spaced like words stay one cell. Aligned code comments can come out as a two-column table. `--text` keeps them as plain lines.
- **Running headers and footers are dropped.** A line in the top two or bottom two lines of a page is removed if it repeats (digits ignored) on three or more of the extracted pages. The first copy stays. Bare page numbers are removed everywhere. Only extracted pages count, so `--pages 5` keeps the header of that page.
- **Hyphenated line breaks are joined.** `trans-` at a line end and `former` on the next line become `transformer`. The hyphen stays if the document uses the compound elsewhere (`self-attention`) or if either half already has a hyphen. Soft hyphens always join. `--as-printed` leaves every line break as it is.
- **Separators differ by mode.** Markdown puts a blank line before each `\f` (`page\n\n\f\n`). `--text` puts `\f` right after the page. `--json` records hold Markdown unless you add `--text`.
- **A broken page does not stop the other pages.** `pdf` warns on stderr (`pdf: page N: ...`) and keeps the text it read before the error. With `--json`, the record has an `error` field. `pdf` exits 1 only if no page has text. If a page draws more than 1,000,000 glyphs or runs more than 10,000 Form XObjects, it stops there with the error `exceeds the per-page glyph or form budget`. After the document passes 20,000,000 glyphs, every later page stops with `exceeds the document glyph budget`. These limits stop a small hostile file from hanging the pipeline.
- **Text outside the page box is dropped.** Content past the CropBox (or MediaBox) does not come out. `pdf` follows Form XObjects up to 8 levels deep.
- **Invisible text is dropped.** Glyphs drawn with render mode 3 or 7 (`3 Tr`) never show in a viewer, and `pdf` drops them too. This is a common way to hide a prompt injection in a document that looks harmless. Tiny or white text still comes out.
- **Control characters are stripped.** `pdf` drops C0 controls, DEL, C1 and bidi overrides from page text and warnings. A newline or tab inside a string becomes a space. The only `\f` in the output is the page separator, so splitting on it is safe.
- **PDF 2.0 is rejected.** The reader handles PDF 1.0 to 1.7. A `%PDF-2.0` header exits 1 with `pdf: PDF 2.0 is not supported (PDF 1.0 to 1.7 only)`.
- **Encrypted PDFs exit 1.** There is no password flag. A PDF with an empty user password (owner-only restrictions) still opens.
- **Ligatures are expanded.** `ﬁ` comes out as `fi`. A font with no ToUnicode map for a glyph can still give `�`.
- **The `--json` trailer is a record too.** To skip `{"_vrk":"pdf","count":N}`, use `jq 'select(.page)'`.
- **An empty page is still a page.** The output keeps its `\f`, so the number of form feeds is always the page count minus one.
- **The page count comes from the page tree, not `/Count`.** `pdf` walks the tree once, so a wrong or huge `/Count` does not matter. If an entry in the tree cannot be read, `pdf` skips it and writes `pdf: page tree: skipped N unreadable entries` to stderr (not with `--json`). Later pages move up one number.
- **`--pages` past the end exits 1** with `pdf: page 9 does not exist (PDF has 3 pages)`. A malformed range like `0`, `5-2` or `abc` exits 2.
- **`pdf` holds the whole file in memory, up to 512 MiB.** The reader needs random access, so a 500 MB PDF uses about 500 MB. Bigger input exits 1 with `input exceeds 512 MiB limit`. A path must be a regular file. `/dev/zero`, a FIFO or a device exits 1 with `not a regular file`. Pipe those on stdin instead.

---

## sql - SQL over CSV

Runs a SQL query over CSV files in an in-memory SQLite database. Each file is a table named after its basename (`users.csv` -> `users`), or `name=file.csv` names it. Piped CSV is the table `stdin`; `-` reads it explicitly and `name=-` names it.
Output is CSV with a header row by default. With `--json`, one object per row, then `{"_vrk":"sql","count":N}`.
Input: the query is the first positional argument (or `--query`), the rest are CSV file paths, `name=path` pairs or `-`. Without `-`, stdin is read only when the query mentions `stdin`.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--query` | | SQL query. Every positional argument is then a CSV file |
| `--no-header` | | Leave the header row out of CSV output |
| `--delimiter` | `-d` | Input field separator: one character, or `\t` / `tab`. Default `,`, or tab for `.tsv` files |
| `--json` | `-j` | One JSON object per row (keys in column order), then `{"_vrk":"sql","count":N}` |
| `--quiet` | `-q` | Suppress stderr output |
| `--timeout` | | Stop and exit 1 (`sql: query timed out after 30s`) if loading and the query take longer. Go duration: `500ms`, `30s`, `2m`. Default `60s`, `0` for no limit |

### Type inference

| Column values (empty cells ignored) | Type |
|------|------|
| All integers that fit in int64, no leading zeros | `INTEGER` |
| Decimals that all have N places, like `19.90` and `5.00` | `REAL(N)` - prints with N decimals |
| Other mixes of integers and decimals, like `1.5` and `2` | `REAL` |
| Anything else, including `007`, `1e3`, `" 30"` or an all-empty column | `TEXT` |
| A decimal a float64 can't hold exactly, like `12345678901234567890.5` | `TEXT` - digits kept as written |

Empty cells are NULL in every column. In INTEGER and REAL columns, `N/A`, `NA`, `null`, `none`, `nan` and `-` (any case) are NULL too, and don't make the column TEXT. Other text in a mostly-numeric column, including an integer too big for 64 bits, makes it TEXT and prints a warning naming the column and row. Under `--json` stderr stays empty and the warnings go in the trailer: `{"_vrk":"sql","count":N,"warnings":["..."]}`.

### --json output shapes

| Flags | Shape |
|-------|-------|
| `--json` | `{"col":value,...}` per row + `{"_vrk":"sql","count":N}` |
| Zero matching rows | trailer only, `count:0` |
| Duplicate result column names | later ones get `_1`, `_2` (`id`, `id_1`), same as the CSV header |
| Any error + `--json` | `{"error":"msg","code":N}` on stdout; stderr empty |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, including empty stdin when the query reads `stdin` |
| 1 | SQL error (syntax, no such table or column, a string in double quotes, runtime), file not found, CSV parse error, empty file, gzip file, `PRAGMA`, timeout, a size limit hit |
| 2 | Usage error - no query, a table but no files given, duplicate or invalid table name, `-` or `stdin` with nothing piped, a file where the query should be (`vrk sql users.csv`), bad `--delimiter`, unknown flag |

### Examples

```bash
# Filter one file
vrk sql 'SELECT name, city FROM users WHERE age > 25' users.csv

# Join two files
vrk sql 'SELECT u.name, sum(o.total) AS spent FROM users u JOIN orders o ON o.user_id = u.id GROUP BY u.name' users.csv orders.csv

# Query piped CSV as the table stdin
cat users.csv | vrk sql 'SELECT city, count(*) AS n FROM stdin GROUP BY city'

# Name a table, or read stdin explicitly with -
vrk sql 'SELECT * FROM sales WHERE total > 100' sales=2024-sales.csv
cat orders.csv | vrk sql 'SELECT u.name FROM o JOIN users u ON u.id = o.user_id' o=- users.csv

# TSV files work as-is; other separators need -d
vrk sql 'SELECT * FROM data' data.tsv
vrk sql -d ';' 'SELECT * FROM export' export.csv

# JSONL out
vrk sql --json 'SELECT * FROM users' users.csv

# Values only, no header
vrk sql --no-header 'SELECT count(*) FROM users' users.csv
```

### Compose patterns

```bash
# Rows as JSONL into an LLM, one call per row
vrk sql --json 'SELECT id, feedback FROM survey WHERE rating < 3' survey.csv \
  | jq -c 'select(.id)' | vrk prompt --field feedback --system 'Classify this complaint.'

# Gate a pipeline on a count
n="$(vrk sql --no-header 'SELECT count(*) FROM orders WHERE total < 0' orders.csv)"
[ "$n" -eq 0 ] || echo "negative totals found"

# Count tokens in a column before sending it
vrk sql --no-header 'SELECT body FROM tickets' tickets.csv | vrk tok
```

### Gotchas

- **Table names come from file names.** Any character other than a letter, digit or `_` becomes `_` (accented letters stay), so `sales-2024.csv` is `sales_2024`. A name that starts with a digit (`2024-sales.csv`, or `/dev/fd/63` from `<(...)`) needs quotes in SQL. Use `name=path` instead. Two tables with the same name (case-insensitive) exit 2. If you use a wrong table name, the error lists the loaded tables.
- **Header names are cleaned the same way.** `first name` becomes `first_name`, an empty header becomes `col_N` (1-indexed), and repeats get `_1`, `_2`. Quote reserved words: `SELECT "order" FROM t`. If you quote the original header (`"First Name"`, any case), `sql` exits 1 and names the cleaned header.
- **Double quotes are for names, single quotes for strings.** The SQLite fallback that reads an unknown `"x"` as the string `x` is off. So `WHERE city = "Pune"` or a typo like `"nme"` exits 1. It does not match nothing or print `nme` on every row. If you use a wrong column name, the error lists the columns of each table.
- **`stdin` is found by word match.** If the query contains `stdin`, even inside a string literal, `vrk sql` reads stdin when it is piped. Pass `-` to be explicit. If stdin is empty or only whitespace (like `echo ''`), `sql` drops the table. If the query needs that table, the result is no output and exit 0, even for `count(*)`. A file called `stdin.csv` clashes with it.
- **`007` stays text.** Leading zeros keep a column TEXT, so ZIP codes and IDs are not changed. To get numbers, cast: `CAST(zip AS INTEGER)`.
- **No such table, with no files, is exit 2.** `vrk sql 'SELECT * FROM t'` means you forgot the file. If you gave files, it is exit 1 with the SQLite message.
- **ATTACH, VACUUM INTO and PRAGMA are blocked.** ATTACH and VACUUM INTO exit 1 with `too many attached databases - max 0`. Any statement with the word `PRAGMA` exits 1 with `PRAGMA is not allowed`. So a query cannot read or write other files. Temp tables stay in memory. The table-valued forms still work: `SELECT name FROM pragma_table_info('users')`.
- **Multiple statements run, and `sql` prints the last result.** `CREATE TABLE x AS SELECT ...; SELECT * FROM x` works. Statements like `DELETE` print nothing.
- **JSONL input.** If the first line is a JSON object (or the input is a JSON array), keys become columns in first-seen order. A missing key or `null` is NULL, `true`/`false` are 1/0, nested objects and arrays are JSON text (`json_extract(tags, '$[0]')`). Records with a `_vrk` key (vrk trailers) are skipped, so `vrk pdf --json | vrk sql ...` works. A bad line exits 1 and names the line. Input with no records (only a trailer, from an upstream run that matched nothing) counts as empty stdin: no output, exit 0.
- **Input must be UTF-8.** A latin1 file exits 1 with the line number. Convert it first with `iconv -f latin1 -t utf-8`. Old Mac `\r`-only line endings work.
- **Numbers may be padded.** ` 42` and `42 ` load as INTEGER 42. Floats print without exponents unless they are tiny or huge (`1e-09`, `1e+300`).
- **Missing cells in short rows are NULL** - a row with more non-empty fields than the header exits 1 and names the line. Trailing empty fields (a trailing comma) are dropped. Stray quotes inside unquoted fields (`5" screen`) are kept as-is. A stray quote inside a quoted field (`"x"y`), or a quote that is never closed, exits 1 with its line number instead of swallowing the lines after it; double quotes inside a quoted field (`""`). Quoted fields that span lines on purpose still load.
- **N/A becomes NULL in number columns.** `SELECT *` prints it as an empty cell. In TEXT columns it stays `N/A`.
- **Mixed decimal places lose trailing zeros.** A column with `1.50` and `2.125` is plain REAL, so `1.50` prints as `1.5`.
- **Gzip files exit 1.** Decompress first: `gunzip -c f.csv.gz | vrk sql 'SELECT ... FROM stdin' -`.
- **Memory use is about 15x the file size.** `sql` holds the file in memory and loads every row into an in-memory SQLite table. A 20 MB CSV with 10M short rows peaks near 275 MB.
- **Size limits exit 1.** The limits are: 1 GiB per file or stdin, 50M cells (rows times header columns), 64 MiB for a single value (`string or blob too big`), and 1 GiB for a result (`add a LIMIT or select fewer columns`). Filter big input before it reaches `vrk sql`.
- **Queries stop after 60s by default.** `--timeout 0` turns the limit off. `--timeout 5m` raises it.
- **Output is held back up to 1 MiB, then streamed.** Below that, stdout is empty on error. A bigger result that fails partway leaves the rows already written; under `--json` the missing `_vrk` trailer marks it incomplete. A runaway query (a recursive CTE with no stop) streams until the timeout, so pipe it into `head` or set `--timeout`.
- **Output is raw.** `sql` does not neutralise cells that start with `=`, `+`, `-` or `@` for spreadsheets, and it does not strip control characters. Do not open untrusted results in Excel as they are. If the result goes to a terminal or an LLM, use `--json`.

---

# Utilities

These are not pipeline tools - they support discovery and integration.

---

## mcp - MCP Server (Discovery Only)

Starts a discovery-only MCP server over stdio (JSON-RPC 2.0). It lists the vrksh
pipeline tools for MCP clients through `tools/list`. It does not run tools:
`tools/call` is not implemented.

Add `vrk mcp` to your Claude Code MCP config to list the tools.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--list` | - | Print all exposed tools and exit (human-readable) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Clean shutdown (SIGINT/EOF) or `--list` completed |
| 1 | Startup error |
| 2 | Usage error - unknown flag |

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

- **Discovery only.** `tools/call` returns method-not-found (-32601). To run a tool, call it from the shell: `vrk <tool> [flags]`.
- **Some tools and flags are hidden.** The server does not list `coax`, `kv` or `prompt`. It also hides flags that take a file path, take a shell command, or turn off a safety guard (`--file`, `--schema`, `--system`, `--until`, `--allow-internal`).
- **Request limits.** A request line over 1 MiB ends the server with a -32700 error and exit 1. An `id` that is not a string or number gets -32600.
- **Clean stdout.** Only JSON-RPC responses go to stdout. All logging goes to stderr.
- **Tool names have a prefix.** MCP tool names start with `vrk_` (for example `vrk_tok`, `vrk_jwt`), so they do not collide in MCP clients.
- **The `input` field in schemas.** Tools that need stdin list `"input"` in the `required` array of their inputSchema. Tools that do not need stdin (uuid, epoch, moniker) do not.
