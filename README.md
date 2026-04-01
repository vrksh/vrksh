# vrksh

LLMs are probabilistic. The tools around them shouldn't be.

vrksh (वृक्ष) is the Sanskrit word for tree. The project is **vrksh**. The command is `vrk`.

[![CI](https://github.com/vrksh/vrksh/actions/workflows/ci.yml/badge.svg)](https://github.com/vrksh/vrksh/actions/workflows/ci.yml)
![Go](https://img.shields.io/badge/go-1.25-blue)
![Release](https://img.shields.io/github/v/release/vrksh/vrksh)

One binary. Small composable tools. Zero silent failures.

## Why

Shell pipelines that call LLMs break in quiet ways. A prompt exceeds the context window and the model silently truncates. A retry loop swallows errors. An API key leaks into logs. `jq` and `awk` were not designed for this failure mode.

vrksh is 26 Unix-style tools in a single static binary. Each tool reads stdin, writes stdout, and uses exit codes that pipelines can trust: 0 for success, 1 for runtime errors, 2 for usage errors. JSONL is the native interchange format. Large inputs stream through `bufio.Scanner`, not `io.ReadAll`, so a 10GB log file won't OOM your agent.

## Install

**Homebrew**

```bash
brew tap vrksh/vrksh
brew install vrk
```

**Binary install**

```bash
curl -fsSL https://vrk.sh/install.sh | sh
```

**From source**

```bash
go install github.com/vrksh/vrksh@latest
```

## Tools

| Tool | What it does | Key flags |
|------|-------------|-----------|
| `tok` | Count tokens, gate pipelines by token budget | `--check N`, `--json` |
| `prompt` | Send a prompt to an LLM (Anthropic/OpenAI) | `--model`, `--system`, `--schema`, `--json` |
| `chunk` | Split text into token-bounded chunks | `--size N`, `--overlap N`, `--by` |
| `jwt` | Decode and inspect JWTs | `--claim`, `--expired`, `--valid`, `--json` |
| `epoch` | Convert between Unix timestamps and ISO 8601 | `--iso`, `--tz`, `--now`, `--at` |
| `uuid` | Generate UUIDs (v4/v7) | `--v7`, `--count N`, `--json` |
| `sse` | Parse Server-Sent Events stream to JSONL | `--event`, `--field` |
| `coax` | Retry a command until it succeeds | `--times N`, `--backoff`, `--on`, `--until` |
| `kv` | Persistent key-value store (SQLite) | `set`, `get`, `del`, `incr`, `list` |
| `grab` | Fetch a URL as clean markdown or plain text | `--text`, `--raw`, `--json` |
| `links` | Extract hyperlinks from text as JSONL | `--bare`, `--json` |
| `plain` | Strip markdown syntax, keep prose | `--json` |
| `jsonl` | Convert JSON arrays to JSONL or collect back | `--collect`, `--json` |
| `validate` | Validate JSONL against a schema, optionally repair via LLM | `--schema`, `--strict`, `--fix` |
| `mask` | Redact secrets by entropy and pattern matching | `--pattern`, `--entropy`, `--json` |
| `emit` | Wrap lines as structured JSONL log records | `--level`, `--tag`, `--parse-level` |
| `assert` | Check conditions mid-pipeline, halt on failure | `<expr>`, `--contains`, `--matches` |
| `sip` | Sample lines from stdin | `--first`, `--count`, `--every`, `--sample` |
| `throttle` | Rate-limit lines from stdin | `--rate N/s`, `--burst N` |
| `digest` | Hash stdin (sha256/md5/sha512), HMAC, compare | `--algo`, `--hmac`, `--key`, `--compare` |
| `base` | Encode/decode base64, base64url, hex, base32 | `encode --to`, `decode --from` |
| `recase` | Convert naming conventions (snake, camel, kebab) | `--to`, `--json` |
| `slug` | Convert text to URL-safe slugs | `--separator`, `--max`, `--json` |
| `moniker` | Generate memorable adjective-noun names | `--count`, `--seed`, `--json` |
| `pct` | Percent-encode/decode per RFC 3986 | `--encode`, `--decode`, `--form` |
| `urlinfo` | Parse a URL into components, no network calls | `--field`, `--json` |

Every tool accepts input as a positional argument or via stdin:

```bash
vrk epoch '+3d'              # positional
echo '+3d' | vrk epoch      # stdin - same result
```

## Pipelines

**Gate a prompt by token budget, then send to an LLM:**

```bash
cat document.txt | vrk tok --check 8000 | vrk prompt --system 'Summarize this'
```

If the input exceeds 8000 tokens, `tok` exits 1 and the pipeline stops before the API call.

**Redact secrets, validate structure, log the result:**

```bash
cat debug.log \
  | vrk mask \
  | vrk assert --contains 'ERROR' \
  | vrk emit --level error --tag incidents
```

**Fetch a page, extract links, sample 10, hash the result:**

```bash
vrk grab 'https://example.com' \
  | vrk links --bare \
  | vrk sip --first 10 \
  | vrk digest
```

**Generate a run ID, store it, use it across pipeline stages:**

```bash
vrk kv set run_id "$(vrk moniker --seed 42)"
vrk kv get run_id
```

## Error contract

All tools follow the same contract:

- **Data** goes to stdout. **Errors** go to stderr.
- **Exit 0** = success. **Exit 1** = runtime error. **Exit 2** = usage error.
- When `--json` is active, errors go to stdout as `{"error":"...","code":N}` and stderr stays empty.

## Discovery

```bash
vrk --manifest          # JSON list of all tools
vrk --skills            # full reference: flags, exit codes, gotchas
vrk --skills tok        # reference for a single tool
vrk mcp                 # MCP server for tool discovery (stdio JSON-RPC)
vrk <tool> --help       # per-tool usage
```

## Symlinks

Create direct symlinks so you can run `tok` instead of `vrk tok`:

```bash
vrk --bare tok jwt epoch     # creates symlinks in /usr/local/bin
vrk --bare --list            # show existing symlinks
vrk --bare --remove tok      # remove a symlink
```

## Shell completions

**Bash:**

```bash
vrk completions bash > ~/.bash_completion.d/vrk
source ~/.bash_completion.d/vrk
```

**Zsh:**

```bash
vrk completions zsh > "${fpath[1]}/_vrk"
```

**Fish:**

```bash
vrk completions fish > ~/.config/fish/completions/vrk.fish
```

## Not for you if

- You need signature verification on JWTs. `vrk jwt` is an inspector, not a validator.
- You want a chat interface. `vrk prompt` is a pipeline tool with temperature 0 by default.
- You need exact token counts for Claude. `tok` uses cl100k_base, which is exact for GPT-4 and ~95% accurate for Claude.
- You want a framework. vrksh is filters and pipes, not an SDK.

## Docs

Full tool reference, examples, and compose patterns: **[vrk.sh](https://vrk.sh)**
