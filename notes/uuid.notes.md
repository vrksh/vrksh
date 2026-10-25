## How it works

`vrk uuid` generates UUIDs: random v4 by default, or time-ordered v7 with `--v7`.

### v4 (random, default)

<!-- doccheck: skip (random output) -->
```bash
$ vrk uuid
98d06ca5-e747-4a63-8fc9-045f0e02f8d4
```

A v4 UUID is random. Use it for correlation IDs, file names, and anywhere you need a unique ID but not an order.

### v7 (time-ordered)

<!-- doccheck: skip (random output) -->
```bash
$ vrk uuid --v7
019d3fdb-2de7-756e-a340-de561cfa43ab
```

The first 48 bits of a v7 UUID are a millisecond timestamp. v7 UUIDs sort by time, so a newer ID is always greater than an older one. Use v7 for database primary keys to keep B-tree indexes efficient.

### Batch generation

<!-- doccheck: skip (random output) -->
```bash
$ vrk uuid --count 3
c00bb1e4-51fd-4cfc-96b2-2c0123268c80
c4fd6dd9-c340-4f42-9929-7f0eae856230
58cbbd73-e561-43de-b20a-ccd5f5f92222

$ vrk uuid --v7 --count 3 --json
{"uuid":"019d3fdb-...","version":7,"generated_at":1774891000}
{"uuid":"019d3fdb-...","version":7,"generated_at":1774891000}
{"uuid":"019d3fdb-...","version":7,"generated_at":1774891000}
```

## Pipeline integration

### Generate a run ID for a batch pipeline

```bash
RUN_ID=$(vrk uuid --v7)
vrk kv set --ns pipeline run_id "$RUN_ID"
echo "Starting run $RUN_ID"
```

### Use UUIDs as chunk IDs

```bash
# Assign unique IDs to each chunk of a document
cat document.md | vrk chunk --size 4000 | \
  while IFS= read -r record; do
    CHUNK_ID=$(vrk uuid --v7)
    echo "$record" | jq --arg id "$CHUNK_ID" '. + {id: $id}'
  done
```

### Generate test data

```bash
# Create 100 unique user records
vrk uuid --count 100 | while IFS= read -r id; do
  echo "{\"id\":\"$id\",\"name\":\"user-$RANDOM\"}"
done
```

## When it fails

If the count is below 1, uuid exits 2:

```bash
$ vrk uuid --count 0
usage error: uuid: count must be >= 1
$ echo $?
2
```
