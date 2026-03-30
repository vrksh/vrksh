# slug - URL/filename slug generator - --separator, --max, --json

When to use: generate URL-safe slugs from text. Normalizes Unicode, lowercases, collapses hyphens. Use --max to truncate at word boundaries.
Composes with: grab, links, kv

| Flag          | Short | Type   | Description                                                   |
|---------------|-------|--------|---------------------------------------------------------------|
| `--separator` |       | string | Word separator (default: `-`)                                 |
| `--max`       |       | int    | Max output length; truncates at word boundary (0 = unlimited) |
| `--json`      | `-j`  | bool   | Emit `{"input":"...","output":"..."}` per line                |
| `--quiet`     | `-q`  | bool   | Suppress stderr                                               |

Exit 0: success (including empty output)
Exit 1: I/O error reading stdin
Exit 2: interactive terminal with no stdin, unknown flag

Example:

    echo 'Hello, World! (2026)' | vrk slug
    # hello-world-2026

Anti-pattern:
- Don't use tr for slugification. It doesn't handle Unicode normalization, double-hyphen collapsing, or word-boundary truncation.
