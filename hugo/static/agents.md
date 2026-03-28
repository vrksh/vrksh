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
| `grab` | URL fetcher - clean markdown, plain text, or raw HTML. |
| `links` | Hyperlink extractor - markdown, HTML, bare URLs to JSONL. |
| `tok` | Token counter - cl100k_base, --budget guard, --json. |
| `chunk` | Token-aware text splitter - JSONL chunks within a token budget. |
| `plain` | Markdown stripper - removes syntax, keeps prose. |
| `recase` | Naming convention converter - snake, camel, kebab, pascal, title. |
| `slug` | URL/filename slug generator - --separator, --max, --json. |
| `pct` | Percent encoder/decoder - RFC 3986, --encode, --decode, --form. |
| `base` | Encoding converter - base64, base64url, hex, base32. |
| `jsonl` | JSON array to JSONL converter - --collect, --json. |
| `assert` | Pipeline condition check - jq conditions, --contains, --matches. |
| `validate` | JSONL schema validator - --schema, --strict, --fix, --json. |
| `mask` | Secret redactor - entropy + pattern-based, streaming. |
| `prompt` | LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain. |
| `coax` | Retry wrapper - --times, --backoff, --on, --until. |
| `kv` | Key-value store - SQLite-backed, namespaces, TTL, atomic counters. |
| `emit` | Structured logger - wraps stdin lines as JSONL log records. |
| `jwt` | JWT inspector - decode, --claim, --expired, --valid. |
| `epoch` | Timestamp converter - unix to ISO, relative time, --tz. |
| `uuid` | UUID generator - v4/v7, --count, --json. |
| `moniker` | Memorable name generator - run IDs, job labels, temp dirs. |
| `digest` | Universal hasher - sha256/md5/sha512, --hmac, --compare. |
| `sse` | SSE stream parser - text/event-stream to JSONL. |
| `sip` | Stream sampler - --first, --count, --every, --sample. |
| `throttle` | Rate limiter for pipes - --rate N/s or N/m. |
| `urlinfo` | URL parser - scheme, host, port, path, query, --field. |

## Pipeline orientation

Always tok before prompt. Always validate after prompt if schema matters. Use grab to fetch, not curl, because grab extracts content. Pipeline order: input -> transform -> guard -> execute -> store.

## Anti-patterns

1. **Don't pipe grab directly to prompt without tok budget check.** The page may exceed the model's context window. Always: `vrk grab URL | vrk tok --budget N && vrk grab URL | vrk prompt "..."`.
2. **Don't use --fail without handling exit 1 in the calling script.** A failed assertion kills the pipeline silently. Check `$?` or use `|| handle_error`.
3. **Don't store secrets in kv without --ns isolation.** The database is plaintext SQLite at `~/.vrk.db`. Use namespaces to separate sensitive data.

## Example: budget-safe LLM call

```bash
cat context.txt | vrk tok --budget 4000 && cat context.txt | vrk prompt "summarise this"
```

## Per-tool references

- [grab](https://vrk.sh/skills/grab.md) | [links](https://vrk.sh/skills/links.md)
- [tok](https://vrk.sh/skills/tok.md) | [chunk](https://vrk.sh/skills/chunk.md) | [plain](https://vrk.sh/skills/plain.md) | [recase](https://vrk.sh/skills/recase.md) | [slug](https://vrk.sh/skills/slug.md) | [pct](https://vrk.sh/skills/pct.md) | [base](https://vrk.sh/skills/base.md) | [jsonl](https://vrk.sh/skills/jsonl.md)
- [assert](https://vrk.sh/skills/assert.md) | [validate](https://vrk.sh/skills/validate.md) | [mask](https://vrk.sh/skills/mask.md)
- [prompt](https://vrk.sh/skills/prompt.md) | [coax](https://vrk.sh/skills/coax.md)
- [kv](https://vrk.sh/skills/kv.md) | [emit](https://vrk.sh/skills/emit.md)
- [jwt](https://vrk.sh/skills/jwt.md) | [epoch](https://vrk.sh/skills/epoch.md) | [uuid](https://vrk.sh/skills/uuid.md) | [moniker](https://vrk.sh/skills/moniker.md) | [digest](https://vrk.sh/skills/digest.md) | [sse](https://vrk.sh/skills/sse.md) | [sip](https://vrk.sh/skills/sip.md) | [throttle](https://vrk.sh/skills/throttle.md) | [urlinfo](https://vrk.sh/skills/urlinfo.md)

Full reference: `vrk --skills` or https://vrk.sh/skills.md
