---
title: "vrk sip"
description: "Stream sampler - --first, --count, --every, --sample"
tool: sip
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You have a million-line log file and need a representative sample for analysis. `head` gives you the first N lines - which are always the oldest, always biased. `shuf` reads the entire file into memory before it produces a single line of output. For a 10 GB JSONL file, neither works. You need sampling that's memory-efficient, statistically sound, and composable with the rest of your pipeline.

`sip` is a stream filter with four sampling strategies. Reservoir sampling (`--count`) gives you a uniform random sample of exactly N lines while reading the stream only once and holding only N lines in memory. `--every` gives you a deterministic 1-in-N sample. `--first` is `head` with the same interface. `--sample` gives probabilistic inclusion at a given percentage. Pick one strategy per invocation.

## The fix

```bash
cat huge.jsonl | vrk sip --count 100 --seed 42
```

<!-- output: verify against binary -->

100 lines drawn uniformly at random from the entire stream, using a fixed seed for reproducibility.

## Walkthrough

### Reservoir sample (--count)

```bash
cat events.jsonl | vrk sip --count 1000
```

<!-- output: verify against binary -->

Uses Vitter's Algorithm R. Memory usage is bounded by the sample size N, not the stream size. The full stream is read once. Output order is the original line order of the sampled records - not shuffled. Every line in the stream had an equal probability of being selected.

### Deterministic seed

```bash
cat events.jsonl | vrk sip --count 100 --seed 42
cat events.jsonl | vrk sip --count 100 --seed 42
```

Both invocations produce identical output when the input is the same. `--seed 0` is valid and deterministic - it's not "no seed". If you need a different random run, change the seed or omit it entirely.

### Every Nth line (--every)

```bash
cat metrics.jsonl | vrk sip --every 10
```

<!-- output: verify against binary -->

Emits line 10, 20, 30, ... - deterministic, no randomness, no memory accumulation. Useful for downsampling a high-frequency time series to a manageable rate while preserving the shape of the data.

### Probabilistic inclusion (--sample)

```bash
cat access.log | vrk sip --sample 5
```

<!-- output: verify against binary -->

Each line is included independently with a 5% probability. The output size is approximately 5% of the input but is not guaranteed. Suitable for quick exploration where approximate counts are fine. Use `--count` when you need an exact N.

### First N lines (--first)

```bash
cat data.jsonl | vrk sip --first 50
```

<!-- output: verify against binary -->

Equivalent to `head -n 50` but integrated into the same interface so you can swap strategies by changing one flag. The stream is stopped immediately after N lines - no unnecessary reading.

### Metadata trailer

```bash
cat data.jsonl | vrk sip --count 10 --json
```

The sampled lines are emitted first, then a metadata record:

```json
{"_vrk":"sip","strategy":"reservoir","requested":10,"returned":10,"total_seen":2847}
```

`total_seen` is the total number of lines read from the stream. `returned` may be less than `requested` if the stream had fewer lines than the sample size.

## Pipeline example

Sample a large JSONL log, validate each record has the expected shape, then summarize:

```bash
cat huge.jsonl \
  | vrk sip --count 100 --seed 42 \
  | vrk validate --schema '{"msg":"string","ts":"number"}' \
  | vrk prompt "What patterns do you see in these log entries?"
```

Or check that a 1% sample of your data passes a quality assertion before running an expensive job:

```bash
cat dataset.jsonl \
  | vrk sip --sample 1 \
  | vrk assert '.score != null' \
  && echo "sample passed - proceeding" \
  || echo "data quality issue detected"
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--first` | | int | `0` | Take first N lines |
| `--count` | `-n` | int | `0` | Reservoir sample of exactly N lines |
| `--every` | | int | `0` | Emit every Nth line (1-indexed) |
| `--sample` | | int | `0` | Include each line with N% probability (1–100) |
| `--seed` | | int64 | `0` | Random seed for reproducibility (0 is a valid seed) |
| `--json` | `-j` | bool | `false` | Append metadata record after all output |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

**Input:** stdin only. `sip` is a stream filter - positional arguments are not accepted.

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (I/O failure reading stdin) |
| 2 | Usage error - no strategy specified, multiple strategies specified, `--sample` value outside 1–100, interactive TTY |
