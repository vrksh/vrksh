# jsonl - JSON array to JSONL converter - --collect, --json

When to use: bridge between JSON array APIs and line-oriented pipeline tools.
Composes with: validate, mask, emit, sip, throttle

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--collect` | `-c` | bool | JSONL to JSON array mode (reverse) |
| `--json` | `-j` | bool | Append `{"_vrk":"jsonl","count":N}` after records (split mode only) |

Exit 0: success (including empty input or empty array)
Exit 1: invalid JSON input, non-array in split mode, invalid line in collect mode
Exit 2: interactive TTY with no stdin, unknown flag

Example:

    echo '[{"a":1},{"a":2}]' | vrk jsonl | vrk mask | vrk jsonl --collect

Anti-pattern:
- Don't use --json in collect mode -- it's a no-op because appending a trailer after `]` would produce invalid JSON.
