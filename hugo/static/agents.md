# vrksh tools

One binary: `vrk`. Unix tools for AI pipelines.

| Tool          | Description                                                        |
|---------------|--------------------------------------------------------------------|
| `assert`      | pipeline condition check - jq conditions, --contains, --matches    |
| `bare`        | symlink creator - use vrksh tools without the vrk prefix           |
| `base`        | encoding converter - base64, base64url, hex, base32                |
| `chunk`       | token-aware text splitter - JSONL chunks within a token budget     |
| `coax`        | retry wrapper - --times, --backoff, --on, --until                  |
| `completions` | shell completion script generator - bash, zsh, fish                |
| `digest`      | universal hasher - sha256/md5/sha512, --hmac, --compare            |
| `emit`        | structured logger - wraps stdin lines as JSONL log records         |
| `epoch`       | timestamp converter - unix/ISO, relative time                      |
| `grab`        | URL fetcher - clean markdown, plain text, or raw HTML.             |
| `jsonl`       | JSON array to JSONL converter - --collect, --json                  |
| `jwt`         | JWT inspector - decode, --claim, --expired, --valid.               |
| `kv`          | key-value store - SQLite-backed, namespaces, TTL, atomic counters. |
| `links`       | hyperlink extractor - markdown, HTML, bare URLs to JSONL           |
| `mask`        | secret redactor - entropy + pattern-based, streaming               |
| `moniker`     | memorable name generator - run IDs, job labels, temp dirs          |
| `pct`         | percent encoder/decoder - RFC 3986, --encode, --decode, --form     |
| `plain`       | markdown stripper - removes syntax, keeps prose                    |
| `prompt`      | LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain.       |
| `recase`      | naming convention converter - snake, camel, kebab, pascal, title   |
| `sip`         | stream sampler - --first, --count, --every, --sample               |
| `slug`        | URL/filename slug generator - --separator, --max, --json           |
| `sse`         | SSE stream parser - text/event-stream to JSONL                     |
| `throttle`    | rate limiter for pipes - --rate N/s or N/m                         |
| `tok`         | Count tokens. Gate pipelines before they fail.                     |
| `urlinfo`     | URL parser - scheme, host, port, path, query, --field              |
| `uuid`        | UUID generator - v4/v7, --count, --json                            |
| `validate`    | JSONL schema validator - --schema, --strict, --fix, --json         |

Full reference: `vrk --skills` or https://vrk.sh/skills.md
