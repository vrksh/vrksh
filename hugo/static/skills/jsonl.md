# jsonl - JSON array to JSONL converter - --collect, --json

When to use: convert between JSON arrays and JSONL streams. Default splits arrays into lines. Use --collect to gather lines back into an array.
Composes with: validate, mask, emit, sip, throttle

| Flag        | Short | Type | Description                                                         |
|-------------|-------|------|---------------------------------------------------------------------|
| `--collect` | `-c`  | bool | JSONL to JSON array mode (reverse)                                  |
| `--json`    | `-j`  | bool | Append `{"_vrk":"jsonl","count":N}` after records (split mode only) |

Exit 0: success (including empty input or empty array)
Exit 1: invalid JSON input, non-array in split mode, invalid line in collect mode
Exit 2: interactive TTY with no stdin, unknown flag

Example:

    echo '[{"a":1},{"a":2}]' | vrk jsonl | vrk mask | vrk jsonl --collect

Anti-pattern:
- Don't use jq '.[]' on large JSON arrays - it loads the entire array into memory. vrk jsonl uses a streaming decoder that handles files larger than available RAM.
