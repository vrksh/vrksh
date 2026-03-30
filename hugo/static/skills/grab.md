# grab - URL fetcher - clean markdown, plain text, or raw HTML

When to use: fetch a web page and get clean content instead of raw HTML. Default output is markdown; --text strips formatting for token-efficient LLM input.
Composes with: tok, chunk, links, plain, prompt, kv

| Flag     | Short | Type | Description                                    |
|----------|-------|------|------------------------------------------------|
| `--text` | `-t`  | bool | Plain prose output, no markdown syntax         |
| `--raw`  |       | bool | Raw HTML, no processing                        |
| `--json` | `-j`  | bool | JSON envelope with metadata and token estimate |

Exit 0: content fetched and extracted
Exit 1: HTTP error, DNS failure, timeout, too many redirects
Exit 2: no URL provided, invalid URL, mutually exclusive flags, unknown flag

Example:

    vrk grab https://example.com | vrk tok --check 8000 | vrk prompt --system "summarise"

Anti-pattern:
- Don't assume grab executes JavaScript. It fetches static HTML only. SPAs that render client-side will return empty or minimal content.
- Don't skip vrk mask before piping grab output to an LLM - web pages can contain API keys, tokens, or credentials in code samples.
