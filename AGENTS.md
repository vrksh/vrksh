# AGENTS.md — vrksh quick reference

Unix-style CLI tools for AI pipelines. One static Go binary, multicall dispatch.
For full flag reference, gotchas, and compose patterns: `vrk --skills`

---

## Tools

| Tool | What it does | Key flags |
|------|-------------|-----------|
| `jwt` | Decode and inspect JWTs (no signature verification) | `--claim <key>`, `--expired`, `--valid`, `--json` |
| `epoch` | Convert between Unix timestamps and ISO 8601 | `--iso`, `--tz <zone>`, `--now`, `--at <ts>` |
| `uuid` | Generate UUIDs | `--v7`, `--count <n>`, `--json` |
| `tok` | Count tokens, enforce budget | `--budget <n>`, `--model <name>` |
| `sse` | Parse Server-Sent Events stream to JSONL | `--event <name>`, `--field <path>` |
| `coax` | Retry a command until it succeeds | `--times <n>`, `--backoff <spec>`, `--on <code>`, `--until <cmd>` |
| `prompt` | Send a prompt to an LLM, emit response | `--model`, `--system`, `--json`, `--schema` |
| `kv` | Persistent key-value store (SQLite-backed) | subcommands: `set get del incr list` |
| `chunk` | Split text into token-bounded chunks | `--size <n>`, `--overlap <n>`, `--by <mode>` |
| `grab` | Fetch a URL as clean markdown or plain text | `--text`, `--raw`, `--json` |
| `plain` | Strip markdown syntax, keep prose | `--json` |
| `jsonl` | Convert JSON arrays to JSONL or collect JSONL into an array | `--collect`, `--json` |
| `validate` | Validate JSONL records against a type schema; optionally repair via LLM | `--schema <spec>`, `--strict`, `--fix`, `--json` |
| `mask` | Redact secrets by pattern matching and Shannon entropy analysis | `--pattern <regex>`, `--entropy <n>`, `--json` |
| `emit` | Wrap text lines as structured JSONL log records with timestamps | `--level <l>`, `--tag <t>`, `--msg <m>`, `--parse-level` |
| `links` | Extract hyperlinks from markdown, HTML, or plain text as JSONL | `--bare`, `--json` |
| `throttle` | Rate-limit lines from stdin | `--rate <N/s\|N/m>`, `--burst N`, `--tokens-field <f>`, `--json` |
| `digest` | Hash stdin or files; HMAC with --hmac --key; file comparison with --compare | `--algo <sha256\|md5\|sha512>`, `--bare`, `--file`, `--hmac`, `--key`, `--verify`, `--json` |
| `base` | Encode and decode between base64, base64url, hex, base32 | subcommands: `encode --to`, `decode --from`; `--quiet` |
| `recase` | Convert naming conventions — auto-detects input, converts to target | `--to <convention>`, `--json`, `--quiet` |
| `slug` | Convert text to URL/filename-safe slugs, unicode normalised to ASCII | `--separator <s>`, `--max <n>`, `--json` |

---

## The contract — every tool follows this

- Input: positional argument **or** stdin (both always work)
- Output: data → stdout only
- Errors: → stderr only; stdout is empty on error
- Exit 0: success
- Exit 1: runtime error (invalid input, API failure, budget exceeded)
- Exit 2: usage error (missing input, unknown flag, bad argument)
- `--help`: always works, exits 0, prints to stdout

```bash
vrk jwt 'eyJ...'           # positional arg
echo 'eyJ...' | vrk jwt    # stdin — identical result
```

---

## Five key patterns

**1. Pipeline guard — fail fast on expired token**
```bash
echo "$JWT" | vrk jwt --expired | vrk prompt --system "..."
# If JWT is expired, vrk jwt exits 1 and the pipeline stops
```

**2. Deterministic timestamps in pipelines**
```bash
echo '+3d' | vrk epoch --at 1740009600    # always 1740268800, regardless of system time
```

**3. Token budget enforcement**
```bash
cat context.txt | vrk tok --budget 4000   # exits 1 if over budget
```

**4. Extract a single JWT claim**
```bash
echo "$JWT" | vrk jwt --claim sub         # prints raw value, no JSON wrapping
```

**5. Persistent state across pipeline runs**
```bash
vrk kv set run_id "$(vrk uuid)"
vrk kv get run_id
```

---

## Flag conventions — consistent across all tools

| Flag | Short | Meaning |
|------|-------|---------|
| `--json` | `-j` | Emit output as JSON object or JSONL |
| `--text` | `-t` | Plain text output, no formatting |
| `--quiet` | `-q` | Suppress stderr |
| `--fail` | `-f` | Exit 1 if condition not met |
| `--schema` | `-s` | Output must match JSON schema |
| `--model` | `-m` | Override model |
| `--count` | `-n` | Numeric count |
| `--explain` | — | Print what would happen, don't do it |
| `--dry-run` | — | Preview mutations without executing |

---

## Gotchas

- **`epoch` relative times must be signed.** `+3d` or `-3d`. Bare `3d` exits 2.
- **`epoch` timezone abbreviations are ambiguous.** `IST`, `EST`, `PST` exit 2.
  Use IANA names (`America/New_York`) or numeric offsets (`+05:30`).
- **`epoch --tz` requires `--iso`.** Using `--tz` without `--iso` exits 2.
- **`epoch --at` requires input.** `vrk epoch --at 1740009600` with no other
  input exits 2. Use `vrk epoch --now` to print the current timestamp.
- **`jwt` does not verify signatures.** It is an inspector, not a validator.
- **`jwt --claim` returns raw values.** Strings are unquoted, booleans are
  `true`/`false`, numbers are plain integers. No JSON wrapping.
- **`prompt --json` wraps the response in metadata.** It does NOT instruct the
  LLM to respond in JSON. Use `--schema` for structured LLM output.
- **`emit --parse-level` matches plain prefixes only.** `ERROR`, `WARN`, `WARNING`, `INFO`, `DEBUG` at the start of a line (optionally followed by `:`, space, or tab). `[INFO]`-style bracket formats are not detected.
- **`mask` is best-effort.** Entropy scanning skips tokens shorter than 8 characters and can false-positive on high-entropy non-secrets. Pattern matching is case-insensitive and replaces the full match, not just the value.
- **`validate --fix` requires an API key.** It shells out to `vrk prompt`. Set `VRK_ANTHROPIC_KEY` or `VRK_LLM_KEY` before use.

---

## Discovery

```bash
vrk --manifest    # JSON list of all tools and descriptions
vrk --skills      # full reference: flags, exit codes, gotchas, compose patterns
vrk <tool> --help # per-tool usage
```
