# coax - Retry wrapper - --times, --backoff, --on, --until

When to use: retry a flaky command with configurable backoff and exit code filtering.
Composes with: prompt, grab, kv

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--times` | | int | Number of retries (default: 3); total attempts = N+1 |
| `--backoff` | | string | Delay spec: `100ms` (fixed) or `exp:100ms` (exponential) |
| `--backoff-max` | | string | Cap for exponential backoff |
| `--on` | | int | Retry only on this exit code (repeatable) |
| `--until` | | string | Shell command; retry until it exits 0 |
| `--quiet` | `-q` | bool | Suppress coax progress lines |

Exit 0: command succeeded on some attempt
Exit (last): all retries exhausted, passes through last command's exit code
Exit 2: --times < 1, no command, unknown flag, bad --backoff format

Example:

    vrk coax --times 3 --backoff exp:1s --on 1 -- vrk prompt --system "summarise" < doc.txt

Anti-pattern:
- Don't forget `--` to separate coax flags from the retried command -- without it, coax exits 2.
