# throttle - Rate limiter for pipes - --rate N/s or N/m

When to use: prevent hitting API rate limits when processing lines in a pipeline loop.
Composes with: prompt, grab, sip, emit

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--rate` | `-r` | string | Rate limit (required): N/s or N/m |
| `--burst` | | int | Emit first N lines without delay |
| `--tokens-field` | | string | Rate by token count of a JSONL field (dot-path) |
| `--json` | `-j` | bool | Append `{"_vrk":"throttle","rate":"...","lines":N,"elapsed_ms":N}` |

Exit 0: success (including empty input)
Exit 1: I/O error, token count failure, invalid JSON with --tokens-field
Exit 2: missing --rate, rate <= 0, bad format, unknown flag, TTY stdin

Example:

    cat prompts.jsonl | vrk throttle --rate 10/m | vrk prompt --model gpt-4o

Anti-pattern:
- Don't use fractional rates like 0.5/s -- N must be a positive integer. Use 1/m for sub-1/s rates.
