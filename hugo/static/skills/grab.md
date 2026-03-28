# grab - URL fetcher - clean markdown, plain text, or raw HTML

When to use: fetch a web page and extract readable content for downstream processing.
Composes with: tok, chunk, links, plain, prompt, kv

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--text` | `-t` | bool | Plain prose output, no markdown syntax |
| `--raw` | | bool | Raw HTML, no processing |
| `--json` | `-j` | bool | JSON envelope with metadata and token estimate |

Exit 0: content fetched and extracted
Exit 1: HTTP error, DNS failure, timeout, too many redirects
Exit 2: no URL provided, invalid URL, mutually exclusive flags, unknown flag

Example:

    vrk grab https://example.com | vrk tok --budget 8000

Anti-pattern:
- Don't pipe grab directly to prompt without a tok budget check -- the page may exceed the model's context window.
