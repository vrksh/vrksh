## Input formats

vrk epoch accepts four input forms:

| Input | Example | Meaning |
|-------|---------|---------|
| Unix integer | `1740009600` | Passed through (or converted with --iso) |
| ISO date | `2025-02-20` | Midnight UTC |
| ISO datetime | `2025-02-20T10:00:00Z` | RFC 3339 |
| Relative offset | `+3d`, `-1h` | From now (or from --at) |

Relative units: `s` (seconds), `m` (minutes), `h` (hours), `d` (days), `w` (weeks).

## How it works

### Unix to ISO

```bash
$ vrk epoch 1740009600 --iso
2025-02-20T00:00:00Z
```

### ISO to Unix

```bash
$ vrk epoch '2025-02-20T00:00:00Z'
1740009600
```

### Relative time

```bash
$ vrk epoch '+3d'
1775152019

$ vrk epoch '+3d' --iso
2026-04-02T17:46:59Z
```

### Current time

```bash
$ vrk epoch --now
1774892819
```

### JSON output

```bash
$ vrk epoch '+3d' --json
{"input":"+3d","unix":1775152019,"iso":"2026-04-02T17:46:59Z"}
```

### Timezone conversion

```bash
$ vrk epoch 1740009600 --iso --tz America/New_York
2025-02-19T19:00:00-05:00

$ vrk epoch 1740009600 --iso --tz +09:00
2025-02-20T09:00:00+09:00
```

### Fixed reference point (--at)

For reproducible scripts, pin the reference time instead of using "now":

```bash
$ vrk epoch '+3d' --at 1740009600 --iso
2025-02-23T00:00:00Z
```

## Pipeline integration

### Store timestamps in kv

```bash
# Record when a pipeline ran
vrk kv set --ns pipeline last_run "$(vrk epoch --now)"

# Set a TTL relative to now
vrk kv set --ns cache result "$DATA" --ttl 24h
```

### Check JWT expiry as a date

```bash
# Decode a JWT's expiry and convert to human-readable
EXP=$(echo "$TOKEN" | vrk jwt --claim exp)
echo "Token expires: $(vrk epoch "$EXP" --iso)"
```

### Schedule-aware pipelines

```bash
# Only run if it's been more than 6 hours since last run
LAST=$(vrk kv get --ns pipeline last_run 2>/dev/null || echo "0")
SIX_HOURS_AGO=$(vrk epoch '-6h')
if [ "$LAST" -lt "$SIX_HOURS_AGO" ]; then
  run-pipeline
  vrk kv set --ns pipeline last_run "$(vrk epoch --now)"
fi
```

## When it fails

Invalid input:

```bash
$ vrk epoch 'not-a-date'
error: epoch: unrecognized input format: "not-a-date"
$ echo $?
1
```

Missing input:

```bash
$ vrk epoch
usage error: epoch: no input provided
$ echo $?
2
```
