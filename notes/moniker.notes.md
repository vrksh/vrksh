## How it works

`vrk moniker` generates short, readable names like `distant-shore` that you can use as run IDs, labels or directory names.

```bash
$ vrk moniker --seed 42
distant-shore
```

### Deterministic output (--seed)

```bash
$ vrk moniker --count 3 --seed 42
distant-shore
slate-contour
northern-brine
```

The same seed always gives the same names. Use a seed in tests or when you need names you can reproduce.

### Custom separator

```bash
$ vrk moniker --separator _ --seed 42
distant_shore
```

### Longer names (--words)

More words give more unique names. Use `--words 3` when two-word names are not enough:

```bash
$ vrk moniker --words 3 --seed 42
foggy-hazy-furrow
```

### JSON output

```bash
$ vrk moniker --json --seed 42
{"name":"distant-shore","words":["distant","shore"]}
```

## Pipeline integration

### Label pipeline runs

```bash
RUN_NAME=$(vrk moniker)
vrk kv set --ns pipeline current_run "$RUN_NAME"
echo "Starting run: $RUN_NAME" | vrk emit --tag pipeline
# ... pipeline stages ...
echo "Completed run: $RUN_NAME" | vrk emit --tag pipeline
```

### Create labeled temp directories

```bash
WORKDIR="/tmp/$(vrk moniker)"
mkdir -p "$WORKDIR"
# Process files in $WORKDIR
```

### Tag batch jobs with memorable names

```bash
# Each batch gets a human-readable name for incident response
BATCH=$(vrk moniker)
vrk kv set --ns batch "$BATCH" "$(vrk epoch --now)" --ttl 168h
echo "Batch $BATCH started" | vrk emit --tag batch --level info
```

## When it fails

If you ask for more names than the word lists can make, moniker exits 1:

```bash
$ vrk moniker --count 100000
error: moniker: requested 100000 names but only 87024 unique 2-word combinations exist
$ echo $?
1
```

If the count is below 1, moniker exits 2:

```bash
$ vrk moniker --count 0
usage error: moniker: --count must be >= 1, got 0
$ echo $?
2
```
