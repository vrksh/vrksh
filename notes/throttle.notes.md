## How it works

`vrk throttle` limits how fast lines pass from stdin to stdout, so a pipeline stays under the rate limit of an API.

### Rate limiting

```bash
# Allow 10 lines per second
seq 20 | vrk throttle --rate 10/s

# Allow 100 lines per minute
cat records.jsonl | vrk throttle --rate 100/m
```

throttle delays lines to keep the target rate. The output is the same as the input, but slower.

### Burst (--burst)

`--burst N` lets the first N lines through at once. After that, throttle enforces the rate:

```bash
cat records.jsonl | vrk throttle --rate 5/s --burst 10
```

The first 10 lines arrive at once. After that, 5 lines arrive per second. Use this for APIs that allow short bursts but limit the sustained rate.

### Token-aware rate limiting (--tokens-field)

If records use different amounts of API quota, count tokens instead of lines:

```bash
cat chunks.jsonl | vrk throttle --rate 100000/m --tokens-field tokens
```

throttle adds up the `tokens` field of each JSONL record instead of counting lines. This keeps you under a tokens-per-minute limit when the chunk sizes differ.

### JSON metadata (--json)

```bash
$ seq 5 | vrk throttle --rate 10/s --json
1
2
3
4
5
{"_vrk":"throttle","rate":"10/s","lines":5,"elapsed_ms":...}
```

## Pipeline integration

### Rate-limit LLM calls

```bash
# Process JSONL records through an LLM at 10 requests per second
cat data.jsonl | vrk throttle --rate 10/s | \
  while IFS= read -r record; do
    echo "$record" | jq -r '.text' | \
      vrk prompt --system 'Classify this text'
  done
```

### Throttle web fetches

```bash
# Fetch URLs at a polite rate
cat urls.txt | vrk throttle --rate 2/s | \
  while IFS= read -r url; do
    vrk grab "$url" | vrk tok --json | jq -r '.tokens'
  done
```

### Sample, throttle, then process

```bash
# Take a sample, pace the processing, and log results
cat large-dataset.jsonl | \
  vrk sip --count 100 --seed 42 | \
  vrk throttle --rate 5/s | \
  while IFS= read -r record; do
    RESULT=$(echo "$record" | vrk prompt --system 'Analyze')
    echo "$RESULT" | vrk emit --tag analysis
  done
```

## When it fails

If you leave out `--rate`, throttle exits 2:

```bash
$ seq 10 | vrk throttle
usage error: throttle: --rate is required
$ echo $?
2
```

If the rate format is invalid, throttle exits 2:

```bash
$ seq 10 | vrk throttle --rate 10/h
usage error: throttle: invalid rate format: use N/s or N/m
$ echo $?
2
```
