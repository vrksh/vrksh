# vrksh skills reference

Machine-readable tool reference. One section per tool.

## tok - Token counter - cl100k_base, --budget guard, --json.

Group: core

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | -j | Emit JSON with token count and metadata |
| `--budget` |   | Exit 1 if token count exceeds N |
| `--model` | -m | Tokenizer model (currently cl100k_base only) |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success, within budget
Exit 1: Over budget or I/O error
Exit 2: Usage error - no input, unknown flag

```bash
cat prompt.txt | vrk tok --budget 4000
```

## prompt - LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain.

Group: core

| Flag | Short | Description |
|------|-------|-------------|
| `--model` | -m | LLM model (default from VRK_DEFAULT_MODEL or claude-sonnet-4-6) |
| `--system` |   | System prompt text, or `@file.txt` to read from file |
| `--budget` |   | Exit 1 if prompt exceeds N tokens |
| `--fail` | -f | Fail on non-2xx API response or schema mismatch |
| `--json` | -j | Emit response as JSON envelope with metadata |
| `--schema` | -s | JSON schema for response validation |
| `--explain` |   | Print equivalent curl command, no API call |
| `--retry` |   | Retry N times on schema mismatch (escalates temperature) |
| `--endpoint` |   | OpenAI-compatible API base URL |

Exit 0: Success
Exit 1: API failure, budget exceeded, or schema mismatch
Exit 2: Usage error - no input, missing flags

```bash
echo "Summarize this" | vrk prompt --model claude-sonnet-4-6 --json
```

## chunk - Token-aware text splitter - JSONL chunks within a token budget.

Group: core

| Flag | Short | Description |
|------|-------|-------------|
| `--size` |   | Max tokens per chunk (required) |
| `--overlap` |   | Token overlap between adjacent chunks (default 0) |
| `--by` |   | Chunking strategy: `paragraph` or token-level (default) |

Exit 0: Success (including empty input)
Exit 1: I/O error, tokenizer failure
Exit 2: --size missing or < 1, --overlap >= --size, unknown --by mode

```bash
cat doc.txt | vrk chunk --size 1000 --overlap 100
```

## grab - URL fetcher - clean markdown, plain text, or raw HTML.

Group: pipeline

| Flag | Short | Description |
|------|-------|-------------|
| `--text` | -t | Plain prose output, no markdown syntax |
| `--raw` |   | Raw HTML, no processing |
| `--json` | -j | Emit JSON envelope with metadata |

Exit 0: Success
Exit 1: HTTP error, fetch timeout, or I/O error
Exit 2: Usage error - invalid URL, no input, mutually exclusive flags

```bash
vrk grab --text https://example.com/article
```

## sse - SSE stream parser - text/event-stream to JSONL.

Group: pipeline

| Flag | Short | Description |
|------|-------|-------------|
| `--event` | -e | Only emit events of this type |
| `--field` | -F | Extract a dot-path field as plain text |

Exit 0: Success (stream parsed, [DONE] or EOF)
Exit 1: I/O error reading stdin
Exit 2: Interactive terminal with no stdin, unknown flag

```bash
curl -sN $API | vrk sse --event content_block_delta --field data.delta.text
```

## validate - JSONL schema validator - --schema, --strict, --fix, --json.

Group: pipeline

| Flag | Short | Description |
|------|-------|-------------|
| `--schema` | -s | Inline JSON schema or file path (required) |
| `--strict` |   | Exit 1 on first invalid line |
| `--fix` |   | Attempt LLM repair of invalid lines |
| `--json` | -j | Append metadata record at end |

Exit 0: All valid, or invalid found but --strict not set
Exit 1: --strict and invalid line found; I/O error
Exit 2: --schema missing or invalid, unknown schema type, unknown flag

```bash
cat records.jsonl | vrk validate --schema '{"name":"string","age":"number"}' --strict
```

## coax - Retry wrapper - --times, --backoff, --on, --until.

Group: pipeline

| Flag | Short | Description |
|------|-------|-------------|
| `--times` |   | Number of retries (default 3) |
| `--backoff` |   | Delay: `100ms` (fixed) or `exp:100ms` (exponential) |
| `--backoff-max` |   | Cap for exponential backoff |
| `--on` |   | Retry only on this exit code (repeatable) |
| `--until` |   | Shell command; retry until it exits 0 |
| `--quiet` | -q | Suppress coax progress lines |

Exit 0: Command succeeded
Exit (last): All retries exhausted
Exit 2: --times < 1, no command, unknown flag

```bash
vrk coax --times 3 --backoff exp:1s --on 1 -- vrk prompt --system "summarise" < doc.txt
```

## kv - Key-value store - SQLite-backed, namespaces, TTL, atomic counters.

Group: pipeline

| Flag | Short | Description |
|------|-------|-------------|
| `--ns` |   | Namespace (default "default") |
| `--ttl` |   | Expiry duration (set only); 0 = no expiry |
| `--dry-run` |   | Print intent without writing (set only) |
| `--by` |   | Delta for incr/decr (must be >= 1) |
| `--json` | -j | Emit errors as JSON |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Key not found, not a number, or database error
Exit 2: Usage error - unknown subcommand, missing args

```bash
vrk kv set --ns cache mykey "myvalue" --ttl 1h
```

## mask - Secret redactor - entropy + pattern-based, streaming.

Group: pipeline

| Flag | Short | Description |
|------|-------|-------------|
| `--pattern` |   | Additional regex (repeatable) |
| `--entropy` |   | Shannon entropy threshold (default 4.0) |
| `--json` | -j | Append metadata record after output |

Exit 0: Success (redacted or not)
Exit 1: I/O error
Exit 2: Interactive terminal, unknown flag, invalid --pattern regex

```bash
vrk prompt --system "summarise" < doc.txt | vrk mask | vrk kv set summary
```

## jwt - JWT inspector - decode, --claim, --expired, --valid.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--claim` | -c | Print value of a single claim |
| `--expired` | -e | Exit 1 if the token is expired |
| `--valid` |   | Exit 1 if expired, nbf in future, or iat in future |
| `--json` | -j | Emit structured JSON output |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success or token is valid
Exit 1: Token expired/invalid or runtime error
Exit 2: Usage error - bad format, too many args

```bash
vrk jwt --claim sub eyJhbGciOiJIUzI1NiJ9...
```

## epoch - Timestamp converter - unix to ISO, relative time, --tz.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--iso` |   | Output as ISO 8601 instead of Unix integer |
| `--json` | -j | Emit structured JSON |
| `--tz` |   | Timezone: IANA name or +HH:MM (requires --iso or --json) |
| `--now` |   | Print current Unix timestamp |
| `--at` |   | Override reference time for relative input |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success
Exit 2: Unsupported format, missing sign, ambiguous timezone

```bash
echo '+3d' | vrk epoch --at 1740009600 --iso
```

## uuid - UUID generator - v4/v7, --count, --json.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--v7` |   | Generate v7 (time-ordered) UUID |
| `--count` | -n | Number of UUIDs (default 1) |
| `--json` | -j | Emit `{uuid, version, generated_at}` per UUID |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success
Exit 2: --count < 1, unknown flag

```bash
vrk uuid --v7
```

## base - Encoding converter - base64, base64url, hex, base32.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--to` |   | Target encoding for encode subcommand |
| `--from` |   | Source encoding for decode subcommand |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success (including empty input)
Exit 1: Invalid input data for the chosen decoding
Exit 2: No subcommand, missing --to/--from, unsupported encoding, unknown flag

```bash
echo 'hello' | vrk base encode --to base64
```

## digest - Universal hasher - sha256/md5/sha512, --hmac, --compare.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--algo` | -a | Algorithm: sha256 (default), md5, sha512 |
| `--bare` | -b | Hash only, no algo: prefix |
| `--file` |   | File to hash (repeatable) |
| `--compare` |   | Compare hashes of --file inputs |
| `--hmac` |   | HMAC mode (requires --key) |
| `--key` | -k | HMAC secret key |
| `--verify` |   | Compare computed HMAC against hex; exit 1 on mismatch |
| `--json` | -j | JSON output with metadata |

Exit 0: Success, --compare result, --verify match
Exit 1: --verify mismatch, file not found, I/O error
Exit 2: Unknown flag/algo, --hmac without --key, --bare + --json

```bash
echo 'hello' | vrk digest --bare
```

## plain - Markdown stripper - removes syntax, keeps prose.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | -j | JSON envelope: `{"text":"...","input_bytes":N,"output_bytes":M}` |

Exit 0: Success (including empty input)
Exit 1: I/O error reading stdin
Exit 2: Interactive terminal with no input, unknown flag

```bash
vrk grab https://example.com | vrk plain
```

## links - Hyperlink extractor - markdown, HTML, bare URLs to JSONL.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--bare` | -b | Output URLs only, one per line |
| `--json` | -j | Append `{"_vrk":"links","count":N}` after all records |

Exit 0: Success (including no links found)
Exit 1: I/O error reading stdin
Exit 2: Usage error - interactive terminal with no stdin, unknown flag

```bash
cat README.md | vrk links --bare
```

## recase - Naming convention converter - snake, camel, kebab, pascal, title.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--to` |   | Target convention (required): camel, pascal, snake, kebab, screaming, title, lower, upper |
| `--json` | -j | Emit JSON object per line |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success (including empty stdin)
Exit 1: I/O error reading stdin
Exit 2: --to missing or unknown, unknown flag, interactive terminal

```bash
echo 'hello_world' | vrk recase --to camel
```

## slug - URL/filename slug generator - --separator, --max, --json.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--separator` |   | Word separator (default: `-`) |
| `--max` |   | Max output length; truncates at word boundary |
| `--json` | -j | Emit `{"input":"...","output":"..."}` per line |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success (including empty output)
Exit 1: I/O error reading stdin
Exit 2: Interactive terminal with no stdin, unknown flag

```bash
echo 'Hello, World!' | vrk slug
```

## moniker - Memorable name generator - run IDs, job labels, temp dirs.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--count` | -n | Number of names (default 1) |
| `--separator` |   | Word separator (default `-`) |
| `--words` |   | Words per name, minimum 2 (default 2) |
| `--seed` |   | Fix random seed for deterministic output |
| `--json` | -j | Emit `{"name":"...","words":[...]}` per name |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success
Exit 1: Count exceeds available unique combinations
Exit 2: --count 0, --words < 2, unknown flag

```bash
vrk moniker --seed 42
```

## pct - Percent encoder/decoder - RFC 3986, --encode, --decode, --form.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--encode` |   | Percent-encode input |
| `--decode` |   | Percent-decode input |
| `--form` |   | Form mode: spaces as `+` instead of `%20` |
| `--json` | -j | Emit JSON object per line |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success (including empty input)
Exit 1: Invalid percent-encoded sequence during decode
Exit 2: Neither or both mode flags, unknown flag, interactive terminal

```bash
echo 'hello world' | vrk pct --encode
```

## jsonl - JSON array to JSONL converter - --collect, --json.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--collect` | -c | JSONL to JSON array mode |
| `--json` | -j | Append `{"_vrk":"jsonl","count":N}` (split mode only) |

Exit 0: Success (including empty input)
Exit 1: Invalid JSON input, non-array in split mode
Exit 2: Interactive TTY with no stdin, unknown flag

```bash
echo '[{"a":1},{"a":2}]' | vrk jsonl
```

## sip - Stream sampler - --first, --count, --every, --sample.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--first` |   | Take first N lines |
| `--count` | -n | Reservoir sample of exactly N lines |
| `--every` |   | Emit every Nth line |
| `--sample` |   | Include each line with N% probability |
| `--seed` |   | Fix random seed |
| `--json` | -j | Append metadata trailer |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success
Exit 1: I/O error reading stdin
Exit 2: No strategy flag, multiple strategies, invalid value, interactive terminal

```bash
cat events.log | vrk sip --count 1000 --seed 42
```

## throttle - Rate limiter for pipes - --rate N/s or N/m.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--rate` | -r | Rate limit (required): N/s or N/m |
| `--burst` |   | Emit first N lines without delay |
| `--tokens-field` |   | Rate by token count of a JSONL field |
| `--json` | -j | Append metadata trailer |

Exit 0: Success (including empty input)
Exit 1: I/O error, token count failure
Exit 2: Missing --rate, rate <= 0, bad format, unknown flag, TTY stdin

```bash
cat prompts.jsonl | vrk throttle --rate 10/m
```

## urlinfo - URL parser - scheme, host, port, path, query, --field.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--field` | -F | Extract a single field (dot-path for query params) |
| `--json` | -j | Append `{"_vrk":"urlinfo","count":N}` after records |
| `--quiet` | -q | Suppress stderr |

Exit 0: Success
Exit 1: Invalid URL (both scheme and host empty)
Exit 2: Interactive terminal with no input, unknown flag

```bash
vrk urlinfo --field host 'https://api.example.com/path'
```

## emit - Structured logger - wraps stdin lines as JSONL log records.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `--level` | -l | Log level: debug, info (default), warn, error |
| `--tag` |   | Add "tag" field to every record |
| `--msg` |   | Override message; stdin as JSON to merge extra fields |
| `--parse-level` |   | Auto-detect level from line prefix |

Exit 0: Success
Exit 1: I/O error
Exit 2: Interactive stdin, unknown flag, invalid --level

```bash
./deploy.sh 2>&1 | vrk emit --tag deploy --parse-level
```

## assert - Pipeline condition check - jq conditions, --contains, --matches.

Group: utility

| Flag | Short | Description |
|------|-------|-------------|
| `<condition>` |   | jq-compatible condition (positional, repeatable) |
| `--contains` |   | Assert stdin contains substring |
| `--matches` |   | Assert stdin matches Go regex |
| `--message` | -m | Custom failure message |
| `--json` | -j | Emit `{"passed":bool,...}` to stdout |
| `--quiet` | -q | Suppress stderr on failure |

Exit 0: Assertion passed, stdin passed through
Exit 1: Assertion failed, JSON parse error, I/O error
Exit 2: No condition, no stdin, mode conflict, invalid regex

```bash
echo '{"score":0.9}' | vrk assert '.score > 0.8' | vrk kv set result
```
