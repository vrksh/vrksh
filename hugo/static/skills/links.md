# links - Hyperlink extractor - markdown, HTML, bare URLs to JSONL

When to use: extract all hyperlinks from markdown, HTML, or plain text. Outputs JSONL with text, URL, and line number. Use --bare for URLs only.
Composes with: grab, urlinfo, slug, kv

| Flag     | Short | Type | Description                                           |
|----------|-------|------|-------------------------------------------------------|
| `--bare` | `-b`  | bool | Output URLs only, one per line                        |
| `--json` | `-j`  | bool | Append `{"_vrk":"links","count":N}` after all records |

Exit 0: success (including no links found)
Exit 1: I/O error reading stdin
Exit 2: interactive terminal with no stdin, unknown flag

Example:

    cat README.md | vrk links --bare | sort -u

Anti-pattern:
- Don't use regex to extract URLs from markdown. Regex misses reference-style links ([text][ref] with [ref]: url elsewhere). vrk links handles all markdown link formats.
