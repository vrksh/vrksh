## How it works

```bash
$ vrk moniker
woven-vent

$ vrk moniker --count 3
distant-shore
slate-contour
northern-brine
```

### Deterministic output (--seed)

```bash
$ vrk moniker --count 3 --seed 42
distant-shore
slate-contour
northern-brine
```

Same seed always produces the same names. Use this in tests or when you need reproducible identifiers.

### Custom separator

```bash
$ vrk moniker --separator _
bold_falcon
```

### Longer names (--words)

More words means more uniqueness. Use `--words 3` when two-word names aren't enough:

```bash
$ vrk moniker --words 3
bright-amber-whisper
```

### JSON output

```bash
$ vrk moniker --json
{"name":"woven-vent","adjective":"woven","noun":"vent"}
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

Word pool exhausted (unlikely with default settings):

```bash
$ vrk moniker --count 100000
error: moniker: word pool exhausted
$ echo $?
1
```

Invalid count:

```bash
$ vrk moniker --count 0
usage error: moniker: --count must be >= 1
$ echo $?
2
```
