# vrksh tools

One binary: `vrk`. Unix tools for AI pipelines. One static Go binary, multicall dispatch. 26 tools that read stdin, write stdout, and compose with pipes. Designed for agents that build reliable data-processing and LLM pipelines.

## MCP config

```json
{
  "mcpServers": {
    "vrksh": {
      "command": "vrk",
      "args": ["mcp"]
    }
  }
}
```

## Tools

| Tool | Description |
|------|-------------|
| `tok` | Token counter - cl100k_base, --budget guard, --json. |
| `prompt` | LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain. |
| `chunk` | Token-aware text splitter - JSONL chunks within a token budget. |
| `grab` | URL fetcher - clean markdown, plain text, or raw HTML. |
| `sse` | SSE stream parser - text/event-stream to JSONL. |
| `validate` | JSONL schema validator - --schema, --strict, --fix, --json. |
| `coax` | Retry wrapper - --times, --backoff, --on, --until. |
| `kv` | Key-value store - SQLite-backed, namespaces, TTL, atomic counters. |
| `mask` | Secret redactor - entropy + pattern-based, streaming. |
| `jwt` | JWT inspector - decode, --claim, --expired, --valid. |
| `epoch` | Timestamp converter - unix to ISO, relative time, --tz. |
| `uuid` | UUID generator - v4/v7, --count, --json. |
| `base` | Encoding converter - base64, base64url, hex, base32. |
| `digest` | Universal hasher - sha256/md5/sha512, --hmac, --compare. |
| `plain` | Markdown stripper - removes syntax, keeps prose. |
| `links` | Hyperlink extractor - markdown, HTML, bare URLs to JSONL. |
| `recase` | Naming convention converter - snake, camel, kebab, pascal, title. |
| `slug` | URL/filename slug generator - --separator, --max, --json. |
| `moniker` | Memorable name generator - run IDs, job labels, temp dirs. |
| `pct` | Percent encoder/decoder - RFC 3986, --encode, --decode, --form. |
| `jsonl` | JSON array to JSONL converter - --collect, --json. |
| `sip` | Stream sampler - --first, --count, --every, --sample. |
| `throttle` | Rate limiter for pipes - --rate N/s or N/m. |
| `urlinfo` | URL parser - scheme, host, port, path, query, --field. |
| `emit` | Structured logger - wraps stdin lines as JSONL log records. |
| `assert` | Pipeline condition check - jq conditions, --contains, --matches. |

## Pipeline orientation

Always tok before prompt. Always validate after prompt if schema matters. Use grab to fetch, not curl, because grab extracts content. Pipeline order: input -> transform -> guard -> execute -> store.

## Anti-patterns

1. **Don't pipe grab directly to prompt without tok budget check.** The page may exceed the model's context window. Always: `vrk grab URL | vrk tok --budget N && vrk grab URL | vrk prompt --system "..."`.
2. **Don't use --fail without handling exit 1 in the calling script.** A failed assertion kills the pipeline silently. Check `$?` or use `|| handle_error`.
3. **Don't store secrets in kv without --ns isolation.** The database is plaintext SQLite at `~/.vrk.db`. Use namespaces to separate sensitive data.

## Example: budget-safe LLM call

```bash
cat context.txt | vrk tok --budget 4000 && cat context.txt | vrk prompt --system "summarise this"
```

## Per-tool references

- [tok](https://vrk.sh/skills/tok.md) | [prompt](https://vrk.sh/skills/prompt.md) | [chunk](https://vrk.sh/skills/chunk.md)
- [grab](https://vrk.sh/skills/grab.md) | [sse](https://vrk.sh/skills/sse.md) | [validate](https://vrk.sh/skills/validate.md) | [coax](https://vrk.sh/skills/coax.md) | [kv](https://vrk.sh/skills/kv.md) | [mask](https://vrk.sh/skills/mask.md)
- [jwt](https://vrk.sh/skills/jwt.md) | [epoch](https://vrk.sh/skills/epoch.md) | [uuid](https://vrk.sh/skills/uuid.md) | [base](https://vrk.sh/skills/base.md) | [digest](https://vrk.sh/skills/digest.md) | [plain](https://vrk.sh/skills/plain.md) | [links](https://vrk.sh/skills/links.md) | [recase](https://vrk.sh/skills/recase.md) | [slug](https://vrk.sh/skills/slug.md) | [moniker](https://vrk.sh/skills/moniker.md) | [pct](https://vrk.sh/skills/pct.md) | [jsonl](https://vrk.sh/skills/jsonl.md) | [sip](https://vrk.sh/skills/sip.md) | [throttle](https://vrk.sh/skills/throttle.md) | [urlinfo](https://vrk.sh/skills/urlinfo.md) | [emit](https://vrk.sh/skills/emit.md) | [assert](https://vrk.sh/skills/assert.md)

Full reference: `vrk --skills` or https://vrk.sh/skills.md

## All machine-readable endpoints

https://vrk.sh/agents/ - index of all endpoints, CLI equivalents, and CLAUDE.md snippet
