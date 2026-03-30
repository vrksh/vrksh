## How it works

### Rate limiting

```bash
# Allow 10 lines per second
seq 20 | vrk throttle --rate 10/s

# Allow 100 lines per minute
cat records.jsonl | vrk throttle --rate 100/m
```

Lines are delayed to maintain the target rate. Output is the same as input, just paced.

### Burst (--burst)

Let the first N lines through immediately, then enforce the rate:

```bash
cat records.jsonl | vrk throttle --rate 5/s --burst 10
```

The first 10 lines arrive instantly. After that, 5 per second. Use this for APIs that allow burst traffic but enforce sustained rate limits.

### Token-aware rate limiting (--tokens-field)

When different records consume different amounts of API quota:

```bash
cat chunks.jsonl | vrk throttle --rate 100000/m --tokens-field tokens
```

Instead of counting lines, throttle counts the value of the `tokens` field in each JSONL record. This keeps you under token-per-minute limits even when chunk sizes vary.

### JSON metadata (--json)

```bash
$ seq 5 | vrk throttle --rate 10/s --json
1
2
3
4
5
{"_vrk":"throttle","rate":"10/s","lines":5,"elapsed_ms":500}
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

Missing --rate:

```bash
$ seq 10 | vrk throttle
usage error: throttle: --rate is required
$ echo $?
2
```

Invalid rate format:

```bash
$ seq 10 | vrk throttle --rate 10/h
usage error: throttle: invalid rate format (use N/s or N/m)
$ echo $?
2
```
