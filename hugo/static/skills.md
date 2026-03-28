# vrksh skills reference

Machine-readable tool reference. One section per tool.

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

## kv - Key-value store - SQLite-backed, namespaces, TTL, atomic counters.

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

## prompt - LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain.

Group: v1

| Flag | Short | Description |
|------|-------|-------------|
| `--model` | -m | LLM model (default from VRK_DEFAULT_MODEL or claude-sonnet-4-6) |
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
echo "Summarize this" | vrk prompt --model claude-sonnet-4-6 --json
```

## tok - Token counter - cl100k_base, --budget guard, --json.

Group: v1

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

