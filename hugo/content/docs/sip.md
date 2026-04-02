---
title: "vrk sip"
description: "Sample lines from a stream. Take the first N, every Nth, or a random percentage. Works on unbounded input."
og_title: "vrk sip - sample lines from streams and large files"
tool: sip
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## The problem

`head -1000` gives you the first 1,000 lines, not a representative sample. `shuf -n 1000` gives a random sample but loads the entire file into memory. On a 10GB log file, `shuf` gets OOM-killed. Reservoir sampling in Python is 20 lines for something that should be one command.

## The solution

`vrk sip` samples lines from stdin using four strategies: `--first N` (first N lines), `--count N` (reservoir sampling, O(N) memory), `--every N` (every Nth line), or `--sample N` (each line with N% probability). `--seed` makes samples reproducible.

## Before and after

**Before**

```bash
shuf -n 1000 large-file.jsonl
# Loads entire file into memory. OOM on large files.
```

**After**

```bash
cat large-file.jsonl | vrk sip --count 1000 --seed 42
```

## Example

```bash
cat access.log | vrk sip --count 1000 --seed 42
```

## Exit codes

| Code | Meaning                                                                             |
|------|-------------------------------------------------------------------------------------|
| 0    | Success                                                                             |
| 1    | I/O failure reading stdin                                                           |
| 2    | No strategy specified, multiple strategies, --sample outside 1-100, interactive TTY |

## Flags

| Flag       | Short | Type  | Description                                   |
|------------|-------|-------|-----------------------------------------------|
| `--first`  |       | int   | Take first N lines                            |
| `--count`  | -n    | int   | Reservoir sample of exactly N lines           |
| `--every`  |       | int   | Emit every Nth line                           |
| `--sample` |       | int   | Include each line with N% probability (1-100) |
| `--seed`   |       | int64 | Random seed for reproducibility               |
| `--json`   | -j    | bool  | Append metadata record after all output       |
| `--quiet`  | -q    | bool  | Suppress stderr output                        |


<!-- notes - edit in notes/sip.notes.md -->

## Sampling strategies

Exactly one must be specified.

### --first N (deterministic, take first N)

```bash
$ seq 100 | vrk sip --first 5
1
2
3
4
5
```

Like `head -n` but integrated with sip's `--json` metadata.

### --count N (random reservoir sample)

```bash
$ seq 100 | vrk sip --count 3 --seed 42
7
55
58
```

Uniform random sample using reservoir sampling. Uses O(N) memory regardless of input size. A 10-million-line file sampled to 1,000 lines uses the same memory as sampling 100 lines.

### --every N (deterministic, every Nth line)

```bash
$ seq 20 | vrk sip --every 5
5
10
15
20
```

Useful for systematic sampling at regular intervals.

### --sample N (probabilistic, N% chance per line)

```bash
seq 1000 | vrk sip --sample 10
```

Each line has a 10% chance of being included. The output count is approximate.

### Deterministic output (--seed)

```bash
$ seq 100 | vrk sip --count 5 --seed 42
# Same output every time
```

Use `--seed` for reproducible samples in tests and CI pipelines.

### JSON metadata (--json)

```bash
$ seq 100 | vrk sip --count 3 --seed 42 --json
7
55
58
{"_vrk":"sip","strategy":"count","requested":3,"emitted":3,"seed":42}
```

## Pipeline integration

### Sample from a large dataset for testing

```bash
# Take a 1,000-line sample from a 10M-line dataset
cat production-logs.jsonl | vrk sip --count 1000 --seed 42 | \
  vrk validate --schema '{"level":"string","msg":"string"}'
```

### Sample before expensive LLM processing

```bash
# Process a random 10% of records through an LLM
cat records.jsonl | vrk sip --sample 10 | \
  while IFS= read -r record; do
    echo "$record" | vrk prompt --system 'Classify this record'
  done
```

### Rate-limit combined with sampling

```bash
# Sample 100 records, then throttle to 5/s for API calls
cat data.jsonl | vrk sip --count 100 | vrk throttle --rate 5/s | process-each
```

## When it fails

No strategy specified:

```bash
$ seq 10 | vrk sip
usage error: sip: specify exactly one of --first, --count, --every, --sample
$ echo $?
2
```

Multiple strategies:

```bash
$ seq 10 | vrk sip --first 5 --count 3
usage error: sip: specify exactly one of --first, --count, --every, --sample
$ echo $?
2
```
