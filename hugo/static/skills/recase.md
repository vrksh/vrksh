# recase - Naming convention converter - snake, camel, kebab, pascal, title

When to use: convert identifiers between naming conventions in batch.
Composes with: slug, emit

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--to` | | string | Target convention (required): camel, pascal, snake, kebab, screaming, title, lower, upper |
| `--json` | `-j` | bool | Emit `{"input":"...","output":"...","from":"...","to":"..."}` per line |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success (including empty stdin)
Exit 1: I/O error reading stdin
Exit 2: --to missing or unknown, unknown flag, interactive terminal

Example:

    echo 'hello_world' | vrk recase --to camel
    # helloWorld

Anti-pattern:
- Don't use `lower` or `upper` for identifier output -- they produce space-separated words, not formatted identifiers.
