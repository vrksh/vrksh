## How it works

### Split a JSON array into JSONL

```bash
$ echo '[{"name":"Alice"},{"name":"Bob"},{"name":"Carol"}]' | vrk jsonl
{"name":"Alice"}
{"name":"Bob"}
{"name":"Carol"}
```

Each array element becomes one line. Pipe to `while read`, `vrk validate`, or any line-oriented tool.

### Collect JSONL back into an array (--collect)

```bash
$ printf '{"name":"Alice"}\n{"name":"Bob"}\n' | vrk jsonl --collect
[{"name":"Alice"},{"name":"Bob"}]
```

Use this when a downstream tool or API expects a JSON array.

### Metadata trailer (--json)

```bash
$ echo '[{"a":1},{"b":2}]' | vrk jsonl --json
{"a":1}
{"b":2}
{"_vrk":"jsonl","count":2}
```

## Pipeline integration

### Split an API response for validation

```bash
# API returns a JSON array; split it for per-record validation
curl -s https://api.example.com/users | \
  vrk jsonl | \
  vrk validate --schema '{"name":"string","email":"string"}' --strict
```

### Process array records through an LLM

```bash
# Split array, process each record, collect results back
cat data.json | vrk jsonl | \
  while IFS= read -r record; do
    echo "$record" | vrk prompt --system 'Classify this record'
  done | vrk jsonl --collect > results.json
```

### Sample from a large array

```bash
# Split a large JSON array, sample 100 records
cat large-dataset.json | vrk jsonl | vrk sip --count 100 --seed 42
```

### Throttle array processing

```bash
# Split array and rate-limit processing to 5 records per second
cat data.json | vrk jsonl | vrk throttle --rate 5/s | \
  while IFS= read -r record; do
    process "$record"
  done
```

## When it fails

Invalid JSON input:

```bash
$ echo 'not json' | vrk jsonl
error: jsonl: invalid JSON
$ echo $?
1
```

No input:

```bash
$ vrk jsonl
usage error: jsonl: no input: pipe JSON to stdin
$ echo $?
2
```
