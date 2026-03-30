# sip - Stream sampler - --first, --count, --every, --sample

When to use: sample lines from a stream without loading the entire input into memory. --count uses reservoir sampling (O(N) memory). Use --seed for reproducible samples.
Composes with: throttle, emit, validate, jsonl

| Flag       | Short | Type | Description                                                                        |
|------------|-------|------|------------------------------------------------------------------------------------|
| `--first`  |       | int  | Take first N lines                                                                 |
| `--count`  | `-n`  | int  | Reservoir sample of exactly N lines (random)                                       |
| `--every`  |       | int  | Emit every Nth line                                                                |
| `--sample` |       | int  | Include each line with N% probability                                              |
| `--seed`   |       | int  | Fix random seed for deterministic output                                           |
| `--json`   | `-j`  | bool | Append `{"_vrk":"sip","strategy":"...","requested":N,"returned":N,"total_seen":N}` |
| `--quiet`  | `-q`  | bool | Suppress stderr                                                                    |

Exit 0: success (including sample larger than population)
Exit 1: I/O error reading stdin
Exit 2: no strategy flag, multiple strategies, invalid value, interactive terminal

Example:

    cat events.log | vrk sip --count 1000 --seed 42

Anti-pattern:
- Don't use shuf -n on large files - it loads the entire file into memory. vrk sip --count uses reservoir sampling with O(N) memory regardless of input size.
