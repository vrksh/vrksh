# sip - Stream sampler - --first, --count, --every, --sample

When to use: sample lines from a large stream without loading the entire file into memory.
Composes with: throttle, emit, validate, jsonl

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--first` | | int | Take first N lines |
| `--count` | `-n` | int | Reservoir sample of exactly N lines (random) |
| `--every` | | int | Emit every Nth line |
| `--sample` | | int | Include each line with N% probability |
| `--seed` | | int | Fix random seed for deterministic output |
| `--json` | `-j` | bool | Append `{"_vrk":"sip","strategy":"...","requested":N,"returned":N,"total_seen":N}` |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success (including sample larger than population)
Exit 1: I/O error reading stdin
Exit 2: no strategy flag, multiple strategies, invalid value, interactive terminal

Example:

    cat events.log | vrk sip --count 1000 --seed 42

Anti-pattern:
- Don't use --sample for exact counts -- it's probabilistic. Use --count for exact reservoir sampling.
