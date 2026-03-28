# links - Hyperlink extractor - markdown, HTML, bare URLs to JSONL

When to use: extract all hyperlinks from a document for crawling, auditing, or indexing.
Composes with: grab, urlinfo, slug, kv

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--bare` | `-b` | bool | Output URLs only, one per line |
| `--json` | `-j` | bool | Append `{"_vrk":"links","count":N}` after all records |

Exit 0: success (including no links found)
Exit 1: I/O error reading stdin
Exit 2: interactive terminal with no stdin, unknown flag

Example:

    cat README.md | vrk links --bare | sort -u

Anti-pattern:
- Don't assume links are absolute -- relative URLs are emitted as-is. Resolve them yourself if needed.
