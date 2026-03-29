# vrksh skills reference

Machine-readable tool reference. One section per tool.

## assert - pipeline condition check - jq conditions, --contains, --matches

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--contains` |   | Assert stdin contains this literal substring |
| `--matches` |   | Assert stdin matches this regular expression |
| `--message` | -m | Custom message on failure |
| `--json` | -j | Emit errors as JSON to stdout |
| `--quiet` | -q | Suppress stderr output on failure |

Exit 0: All conditions passed; input passed through to stdout
Exit 1: Assertion failed, or runtime error
Exit 2: No condition specified, mixed modes, invalid regex, interactive TTY

```bash
vrk grab https://api.example.com/health | vrk assert --contains '"status":"ok"'
```

## bare - symlink creator - use vrksh tools without the vrk prefix

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--force` |   | Overwrite existing files at symlink paths |
| `--remove` |   | Remove bare symlinks (only those pointing to vrk) |
| `--list` |   | List currently active bare symlinks |
| `--dry-run` |   | Show what would happen, make no changes |

Exit 0: Success
Exit 1: Filesystem error creating or removing symlinks
Exit 2: Usage error

```bash
vrk --bare --dry-run
```

## base - encoding converter - base64, base64url, hex, base32

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--to` |   | Target encoding: base64, base64url, hex, base32 (encode subcommand) |
| `--from` |   | Source encoding: base64, base64url, hex, base32 (decode subcommand) |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Invalid encoded input (bad characters, wrong padding)
Exit 2: Missing subcommand, --to/--from missing or unsupported

```bash
echo 'hello' | vrk base encode --to base64
```

## chunk - token-aware text splitter - JSONL chunks within a token budget

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--size` |   | Max tokens per chunk (required) |
| `--overlap` |   | Token overlap between adjacent chunks |
| `--by` |   | Chunking strategy: paragraph |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success, including empty input
Exit 1: I/O error
Exit 2: No input, --size missing or < 1, --overlap >= --size, unknown flag

```bash
cat long-doc.md | vrk chunk --size 4000
```

## coax - retry wrapper - --times, --backoff, --on, --until

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--times` |   | Number of retries (total attempts = N+1) |
| `--backoff` |   | Delay between retries: 500ms for fixed, exp:100ms for exponential |
| `--backoff-max` |   | Cap for exponential backoff; 0 means uncapped |
| `--on` |   | Retry only on these exit codes; repeatable |
| `--until` |   | Shell condition; retry while this exits non-zero |
| `--quiet` | -q | Suppress retry progress lines |
| `--json` | -j | Emit errors as JSON |

Exit 0: Command succeeded (first attempt or a retry)
Exit 1: All retries exhausted (last exit code from wrapped command)
Exit 2: Missing command after --, invalid --backoff spec

```bash
vrk coax --times 5 --backoff exp:200ms -- curl -sf https://api.example.com
```

## completions - shell completion script generator - bash, zsh, fish

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | -j | Emit errors as JSON |

Exit 0: Script emitted to stdout
Exit 1: Unknown shell argument
Exit 2: No shell argument provided

```bash
vrk completions bash > ~/.bash_completion.d/vrk
```

## digest - universal hasher - sha256/md5/sha512, --hmac, --compare

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--algo` | -a | Hash algorithm: sha256, md5, sha512 |
| `--bare` | -b | Output hex hash only, without algo: prefix |
| `--file` |   | Path to file to hash (repeatable) |
| `--compare` |   | Compare hashes of all --file inputs |
| `--hmac` |   | Compute HMAC instead of plain hash |
| `--key` | -k | HMAC secret key (required with --hmac) |
| `--verify` |   | Known HMAC hex to verify against |
| `--json` | -j | Emit JSON object instead of algo:hash line |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success, hash written, or --verify matched
Exit 1: File not found, read error, or --verify mismatch
Exit 2: Unknown algorithm, --hmac without --key, --verify without --hmac

```bash
vrk digest 'hello'
```

## emit - structured logger - wraps stdin lines as JSONL log records

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--level` | -l | Log level: debug, info, warn, error |
| `--tag` |   | Value for the tag field on every record |
| `--msg` |   | Fixed message override; stdin lines parsed as JSON and merged |
| `--parse-level` |   | Auto-detect level from ERROR/WARN/INFO/DEBUG line prefixes |

Exit 0: All non-empty lines emitted as JSONL records
Exit 1: Stdin scanner error or write failure
Exit 2: Interactive TTY with no positional arg, or unknown --level value

```bash
some-script.sh | vrk emit --level info --tag my-script
```

## epoch - timestamp converter - unix/ISO, relative time

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--iso` |   | Output as ISO 8601 string instead of Unix integer |
| `--json` | -j | Emit JSON with all representations |
| `--tz` |   | Timezone for --iso or --json output (IANA name or offset) |
| `--now` |   | Print current Unix timestamp without reading stdin |
| `--at` |   | Reference timestamp for relative input (makes scripts deterministic) |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Runtime error (I/O failure)
Exit 2: Unsupported format, ambiguous timezone, --tz without --iso/--json

```bash
vrk epoch 1740009600 --iso
```

## grab - URL fetcher - clean markdown, plain text, or raw HTML.

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--text` | -t | Plain prose output, no markdown syntax |
| `--raw` |   | Raw HTML, no processing |
| `--json` | -j | Emit JSON envelope with metadata |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: HTTP error, fetch timeout, or I/O error
Exit 2: Usage error - invalid URL, no input, mutually exclusive flags

```bash
vrk grab --text https://example.com/article
```

## jsonl - JSON array to JSONL converter - --collect, --json

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--collect` | -c | Collect JSONL lines into a JSON array |
| `--json` | -j | Append metadata trailer after all records (split mode only) |

Exit 0: Success, including empty input
Exit 1: Invalid JSON, I/O error
Exit 2: Interactive TTY with no input, unknown flag

```bash
cat data.json | vrk jsonl
```

## jwt - JWT inspector - decode, --claim, --expired, --valid.

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | -j | Emit decoded token as JSON |
| `--claim` | -c | Print value of a single claim |
| `--expired` | -e | Exit 1 if the token is expired |
| `--valid` |   | Exit 1 if expired, not-yet-valid, or issued in the future |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success or token is valid
Exit 1: Token expired/invalid or runtime error
Exit 2: Usage error - bad format, too many args

```bash
vrk jwt --claim sub eyJhbGciOiJIUzI1NiJ9...
```

## kv - key-value store - SQLite-backed, namespaces, TTL, atomic counters.

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--ns` |   | Namespace (default "default") |
| `--quiet` | -q | Suppress stderr output |
| `--ttl` |   | Expiry duration (set only); 0 = no expiry |
| `--dry-run` |   | Print intent without writing (set only) |
| `--json` | -j | Emit errors as JSON (get, incr, decr) |
| `--by` |   | Delta for incr/decr (must be >= 1) |

Exit 0: Success
Exit 1: Key not found, not a number, or database error
Exit 2: Usage error - unknown subcommand, missing args

```bash
vrk kv set --ns cache mykey "myvalue" --ttl 1h
```

## links - hyperlink extractor - markdown, HTML, bare URLs to JSONL

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--bare` | -b | Output URLs only, one per line |
| `--json` | -j | Append metadata trailer after all records |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success, including empty input and documents with no links
Exit 1: I/O error reading stdin
Exit 2: Interactive TTY with no piped input, unknown flag

```bash
cat README.md | vrk links
```

## mask - secret redactor - entropy + pattern-based, streaming

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--pattern` |   | Additional Go regex to match and redact (repeatable) |
| `--entropy` |   | Shannon entropy threshold; lower catches more |
| `--json` | -j | Append metadata trailer after output |
| `--quiet` | -q | Suppress stderr output |

Exit 0: All lines processed
Exit 1: Stdin scanner error or write failure
Exit 2: Interactive TTY with no piped input, invalid regex

```bash
cat output.txt | vrk mask
```

## moniker - memorable name generator - run IDs, job labels, temp dirs

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--count` | -n | Number of names to generate |
| `--separator` |   | Word separator |
| `--words` |   | Words per name (minimum 2) |
| `--seed` |   | Random seed for deterministic output |
| `--json` | -j | Emit JSON per name: {name, words} |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Word pool exhausted for requested count
Exit 2: --count less than 1, --words less than 2

```bash
vrk moniker --seed 42
```

## pct - percent encoder/decoder - RFC 3986, --encode, --decode, --form

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--encode` |   | Percent-encode input (RFC 3986 unless --form) |
| `--decode` |   | Percent-decode input |
| `--form` |   | Use application/x-www-form-urlencoded rules (spaces / +) |
| `--json` | -j | Emit JSON per line: {input, output, op, mode} |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Invalid percent sequence during decode, I/O error
Exit 2: Neither --encode nor --decode specified, both specified, interactive TTY

```bash
echo 'hello world' | vrk pct --encode
```

## plain - markdown stripper - removes syntax, keeps prose

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--json` | -j | Emit JSON with text, input_bytes, output_bytes |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Could not read stdin or write stdout
Exit 2: Interactive TTY with no piped input and no positional arg

```bash
cat README.md | vrk plain | vrk tok --budget 4000
```

## prompt - LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain.

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--model` | -m | LLM model (default from VRK_DEFAULT_MODEL or claude-sonnet-4-6) |
| `--system` |   | System prompt text, or @file.txt to read from file |
| `--budget` |   | Exit 1 if prompt exceeds N tokens |
| `--fail` | -f | Fail on non-2xx API response or schema mismatch |
| `--json` | -j | Emit response as JSON envelope with metadata |
| `--quiet` | -q | Suppress stderr output |
| `--schema` | -s | JSON schema for response validation |
| `--explain` |   | Print equivalent curl command, no API call |
| `--retry` |   | Retry N times on schema mismatch (escalates temperature) |
| `--endpoint` |   | OpenAI-compatible API base URL |

Exit 0: Success
Exit 1: API failure, budget exceeded, or schema mismatch
Exit 2: Usage error - no input, missing flags

```bash
cat article.md | vrk prompt --system 'Summarize this' --model claude-sonnet-4-6 --json
```

## recase - naming convention converter - snake, camel, kebab, pascal, title

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--to` |   | Target convention: camel, pascal, snake, kebab, screaming, title, lower, upper |
| `--json` | -j | Emit JSON per line: {input, output, from, to} |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Runtime error (I/O failure)
Exit 2: --to missing or invalid value, interactive TTY with no stdin

```bash
echo 'getUserName' | vrk recase --to snake
```

## sip - stream sampler - --first, --count, --every, --sample

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--first` |   | Take first N lines |
| `--count` | -n | Reservoir sample of exactly N lines |
| `--every` |   | Emit every Nth line |
| `--sample` |   | Include each line with N% probability (1-100) |
| `--seed` |   | Random seed for reproducibility |
| `--json` | -j | Append metadata record after all output |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: I/O failure reading stdin
Exit 2: No strategy specified, multiple strategies, --sample outside 1-100, interactive TTY

```bash
cat huge.jsonl | vrk sip --count 100 --seed 42
```

## slug - URL/filename slug generator - --separator, --max, --json

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--separator` |   | Word separator character or string |
| `--max` |   | Max output length; truncated at last separator (0 = no limit) |
| `--json` | -j | Emit JSON per line: {input, output} |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Runtime error (I/O failure)
Exit 2: Interactive TTY with no stdin

```bash
echo 'My Article Title' | vrk slug
```

## sse - SSE stream parser - text/event-stream to JSONL

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--event` | -e | Only emit events of this type |
| `--field` | -F | Extract dot-path field from record as plain text |

Exit 0: Success, including clean [DONE] termination
Exit 1: I/O error reading stdin
Exit 2: Usage error - interactive terminal with no piped input, unknown flag

```bash
curl -sN https://api.example.com/stream | vrk sse
```

## throttle - rate limiter for pipes - --rate N/s or N/m

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--rate` | -r | Rate limit in N/s or N/m format (required) |
| `--burst` |   | Emit first N lines immediately before applying rate limit |
| `--tokens-field` |   | Dot-path to JSONL field for token-based rate limiting |
| `--json` | -j | Append metadata record after all output |
| `--quiet` | -q | Suppress stderr output |

Exit 0: All lines emitted at specified rate
Exit 1: Stdin read error, write error, or --tokens-field not found
Exit 2: --rate missing or invalid, interactive TTY

```bash
cat urls.txt | vrk throttle --rate 10/s | xargs -I{} vrk grab {}
```

## tok - Count tokens. Gate pipelines before they fail.

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--check` |   | Pass input through if ≤N tokens; exit 1 with empty stdout if over |
| `--model` |   | Tokenizer — cl100k_base (default) |
| `--json` | -j | Emit JSON (measurement) or JSON error (gate). Does not wrap passthrough. |
| `--quiet` | -q | Suppress stderr on failure |

Exit 0: Measurement success; or --check within limit
Exit 1: --check over limit; I/O error; tokenizer error
Exit 2: Usage error — unknown flag, no stdin, --check without value

```bash
cat prompt.txt | vrk tok --check 8000 | vrk prompt
```

## urlinfo - URL parser - scheme, host, port, path, query, --field

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--field` | -F | Extract a single field as plain text (supports dot-path for query params) |
| `--json` | -j | Append metadata trailer |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Invalid URL that cannot be parsed, I/O error
Exit 2: Interactive TTY with no stdin or positional arg

```bash
echo 'https://api.example.com:8443/v2/users?page=2' | vrk urlinfo
```

## uuid - UUID generator - v4/v7, --count, --json

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--v7` |   | Generate a v7 (time-ordered) UUID instead of v4 |
| `--count` | -n | Number of UUIDs to generate |
| `--json` | -j | Emit JSON with uuid, version, generated_at |
| `--quiet` | -q | Suppress stderr output |

Exit 0: Success
Exit 1: Runtime error (generation failure)
Exit 2: --count less than 1, unknown flag

```bash
vrk uuid --v7
```

## validate - JSONL schema validator - --schema, --strict, --fix, --json

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--schema` | -s | JSON schema inline or path to .json file (required) |
| `--strict` |   | Exit 1 on first invalid record |
| `--fix` |   | Send invalid records to vrk prompt for repair |
| `--json` | -j | Append metadata trailer after all output |
| `--quiet` | -q | Suppress stderr output |

Exit 0: All records passed
Exit 1: Record failed in --strict mode, or scanner error
Exit 2: --schema missing, schema JSON invalid, unknown flag

```bash
cat records.jsonl | vrk validate --schema '{"name":"string","age":"number"}' --strict
```

