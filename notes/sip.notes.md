## Sampling strategies

`vrk sip` takes a sample of lines from stdin, by position, at random or by chance. Set exactly one strategy.

### --first N (deterministic, take first N)

```bash
$ seq 100 | vrk sip --first 5
1
2
3
4
5
```

This works like `head -n`, and `--json` adds metadata.

### --count N (random reservoir sample)

```bash
$ seq 100 | vrk sip --count 3 --seed 42
7
55
58
```

This is a uniform random sample, taken with reservoir sampling. Memory use depends on N, not on the input size. A 10-million-line file sampled to 1,000 lines uses the same memory as a 1,000-line file sampled to 1,000 lines.

### --every N (deterministic, every Nth line)

```bash
$ seq 20 | vrk sip --every 5
5
10
15
20
```

Use this to take lines at regular intervals.

### --sample N (probabilistic, N% chance per line)

```bash
seq 1000 | vrk sip --sample 10
```

Each line has a 10% chance of being in the output. The output count is approximate.

### Deterministic output (--seed)

```bash
$ seq 100 | vrk sip --count 5 --seed 42
11
67
71
80
81
```

Use `--seed` to get the same sample every time in tests and CI pipelines.

### JSON metadata (--json)

```bash
$ seq 100 | vrk sip --count 3 --seed 42 --json
7
55
58
{"_vrk":"sip","strategy":"reservoir","requested":3,"returned":3,"total_seen":100}
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

### Rate-limit a sample

```bash
# Sample 100 records, then throttle to 5/s for API calls
cat data.jsonl | vrk sip --count 100 | vrk throttle --rate 5/s | process-each
```

## When it fails

If you set no strategy, sip exits 2:

```bash
$ seq 10 | vrk sip
usage error: sip: a sampling flag is required
$ echo $?
2
```

If you set more than one strategy, sip exits 2:

```bash
$ seq 10 | vrk sip --first 5 --count 3
usage error: sip: only one sampling strategy may be used at a time
$ echo $?
2
```
