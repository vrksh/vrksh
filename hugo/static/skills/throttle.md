# throttle - Rate limiter for pipes - --rate N/s or N/m

When to use: pace pipeline flow to respect API rate limits. Set --rate 10/s or 100/m. Use --burst for APIs that allow initial burst traffic.
Composes with: prompt, grab, sip, emit

| Flag             | Short | Type   | Description                                                        |
|------------------|-------|--------|--------------------------------------------------------------------|
| `--rate`         | `-r`  | string | Rate limit (required): N/s or N/m                                  |
| `--burst`        |       | int    | Emit first N lines without delay                                   |
| `--tokens-field` |       | string | Rate by token count of a JSONL field (dot-path)                    |
| `--json`         | `-j`  | bool   | Append `{"_vrk":"throttle","rate":"...","lines":N,"elapsed_ms":N}` |

Exit 0: success (including empty input)
Exit 1: I/O error, token count failure, invalid JSON with --tokens-field
Exit 2: missing --rate, rate <= 0, bad format, unknown flag, TTY stdin

Example:

    cat prompts.jsonl | vrk throttle --rate 10/m | vrk prompt --model gpt-4o

Anti-pattern:
- Don't use sleep in a while loop instead of throttle. sleep doesn't account for processing time, so your effective rate is always slower than intended.
- Don't set --rate higher than the API's documented limit. Throttle paces output, it doesn't queue or retry on 429s. Use vrk coax for retries.
