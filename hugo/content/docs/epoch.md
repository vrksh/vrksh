---
title: "vrk epoch"
description: "Convert between Unix timestamps and ISO dates. Supports relative time like +3d or -1h."
meta_lead: "vrk epoch converts between Unix timestamps and ISO 8601, and handles relative offsets like +3d or -1h."
og_title: "vrk epoch - Unix timestamp and ISO date converter"
tool: epoch
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk epoch converts between Unix timestamps and ISO 8601, and handles relative offsets like +3d or -1h.

## The problem

`date -d @1740009600` works on Linux. On macOS, `-d` means something different and you need `date -r 1740009600` instead. Relative times are worse: `date -d '+3 days'` is Linux-only. Every timestamp conversion in a cross-platform script becomes a compatibility puzzle.

## The solution

`vrk epoch` converts between Unix timestamps and ISO 8601, and handles relative offsets like `+3d` or `-1h`. Works identically on macOS and Linux. Use `--at` to pin the reference time for deterministic pipeline output.

## Before and after

**Before**

```bash
# Linux: date -d @1740009600
# macOS: date -r 1740009600
# Neither is portable. Scripts break on the other OS.
```

**After**

```bash
vrk epoch 1740009600 --iso
```

## Example

```bash
vrk epoch '+3d' --iso
```

## Exit codes

| Code | Meaning                                                           |
|------|-------------------------------------------------------------------|
| 0    | Success                                                           |
| 1    | Runtime error (I/O failure)                                       |
| 2    | Unsupported format, ambiguous timezone, --tz without --iso/--json |

## Flags

| Flag      | Short | Type   | Description                                                          |
|-----------|-------|--------|----------------------------------------------------------------------|
| `--iso`   |       | bool   | Output as ISO 8601 string instead of Unix integer                    |
| `--json`  | -j    | bool   | Emit JSON with all representations                                   |
| `--tz`    |       | string | Timezone for --iso or --json output (IANA name or offset)            |
| `--now`   |       | bool   | Print current Unix timestamp without reading stdin                   |
| `--at`    |       | string | Reference timestamp for relative input (makes scripts deterministic) |
| `--quiet` | -q    | bool   | Suppress stderr output                                               |


<!-- notes - edit in notes/epoch.notes.md -->

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
