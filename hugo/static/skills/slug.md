# slug - URL/filename slug generator - --separator, --max, --json

When to use: convert text to URL-safe or filename-safe slugs.
Composes with: grab, links, kv

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--separator` | | string | Word separator (default: `-`) |
| `--max` | | int | Max output length; truncates at word boundary (0 = unlimited) |
| `--json` | `-j` | bool | Emit `{"input":"...","output":"..."}` per line |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success (including empty output)
Exit 1: I/O error reading stdin
Exit 2: interactive terminal with no stdin, unknown flag

Example:

    echo 'Hello, World! (2026)' | vrk slug
    # hello-world-2026

Anti-pattern:
- Don't set --max smaller than your longest expected word -- if the first word exceeds --max, output is empty.
