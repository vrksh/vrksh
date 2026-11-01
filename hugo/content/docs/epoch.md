---
title: "vrk epoch"
description: "vrk epoch converts between Unix timestamps and ISO 8601 dates. It also accepts relative times such as +3d or -1h, on macOS and Linux."
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

`vrk epoch` converts between Unix timestamps and ISO 8601, and handles relative offsets like `+3d` or `-1h`. It works the same on macOS and Linux. Use `--at` to pin the reference time, so the pipeline output is the same on every run.

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

| Code | Meaning                                                                                                  |
|------|----------------------------------------------------------------------------------------------------------|
| 0    | Success                                                                                                  |
| 1    | Invalid input value (unsupported format, invalid date, missing sign on a relative offset) or I/O failure |
| 2    | No input, unknown flag, ambiguous timezone, --tz without --iso/--json                                    |

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

`vrk epoch` converts between Unix timestamps and ISO 8601 dates, and calculates relative times such as `+3d`.

vrk epoch accepts four input forms:

| Input | Example | Meaning |
|-------|---------|---------|
| Unix integer | `1740009600` | Passed through (or converted with --iso) |
| ISO date | `2025-02-20` | Midnight UTC |
| ISO datetime | `2025-02-20T10:00:00Z` | RFC 3339 |
| Relative offset | `+3d`, `-1h` | From now (or from --at) |

epoch reads an integer as milliseconds (JavaScript `Date.now()`) if its absolute value is from 100000000000 up to, but not including, 100000000000000. So `vrk epoch --iso 1792042275000` prints `2026-10-15T05:31:15Z`. Ten-digit values stay seconds. With `--json`, millisecond input adds `"unit":"ms"`.

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
$ vrk epoch '+3d' --at 1740009600
1740268800

$ vrk epoch '+3d' --at 1740009600 --iso
2025-02-23T00:00:00Z
```

### Current time

<!-- doccheck: skip (wall-clock time) -->
```bash
$ vrk epoch --now
1774892819
```

### JSON output

```bash
$ vrk epoch '+3d' --at 1740009600 --json
{"input":"+3d","unix":1740268800,"iso":"2025-02-23T00:00:00Z","ref":"1740009600"}
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
error: epoch: unsupported format "not-a-date": natural language not supported; accepted forms: unix integer, +3d/-3d, YYYY-MM-DD, YYYY-MM-DDTHH:MM:SSZ
$ echo $?
1
```

A date that does not exist is also exit 1:

```bash
$ vrk epoch 2026-02-29
error: epoch: invalid date "2026-02-29"
$ echo $?
1
```

Missing input:

```bash
$ vrk epoch
usage error: epoch: no input: provide as argument or via stdin
$ echo $?
2
```
