---
title: "vrk epoch"
description: "Timestamp converter - unix↔ISO, relative time."
tool: epoch
group: utilities
mcp_callable: true
noindex: false
---

## The problem

Unix timestamps show up everywhere - API responses, log files, JWT claims, cron schedules. Reading `1740009600` tells you nothing. Converting it by hand means opening a browser tab, breaking your flow, and hoping you don't mix up seconds and milliseconds. Going the other direction is worse: if you need "three days from now" as a Unix timestamp for an API call, you're reaching for Python or bc.

## The fix

```bash
vrk epoch 1740009600 --iso
```

<!-- output: verify against binary -->

Or the other direction - get a Unix timestamp for a relative offset from now:

```bash
vrk epoch '+3d'
```

<!-- output: verify against binary -->

Both the positional argument and stdin forms work identically:

```bash
echo '1740009600' | vrk epoch --iso
```

## Walkthrough

Epoch accepts four input forms and converts between them.

**Unix passthrough** - a bare integer is returned as-is (useful for validation) or converted with `--iso`:

```bash
vrk epoch 1740009600 --iso
# 2025-02-20T00:00:00Z
```

**ISO to Unix** - parse a date or datetime back to a timestamp:

```bash
vrk epoch '2025-02-20T00:00:00Z'
# 1740009600
```

**Relative time** - the sign is required. Units: `s` seconds, `m` minutes, `h` hours, `d` days, `w` weeks:

```bash
vrk epoch '+3d'   # 3 days from now, as Unix timestamp
vrk epoch '-1h'   # 1 hour ago
```

**Current timestamp** - no input needed:

```bash
vrk epoch --now
```

**Structured output** - `--json` gives you everything at once:

```bash
vrk epoch 1740009600 --json
# {"input":"1740009600","unix":1740009600,"iso":"2025-02-20T00:00:00Z"}
```

**Timezone** - `--tz` requires `--iso` or `--json`. Use IANA names or numeric offsets. Abbreviations like `EST` are rejected as ambiguous:

```bash
vrk epoch 1740009600 --iso --tz America/New_York
vrk epoch 1740009600 --iso --tz +05:30
```

## Pipeline example

Store the expiry of a token rotation three days out:

```bash
vrk epoch '+3d' | vrk kv set --ns schedule token_expiry
```

Fetch an API response that contains a Unix timestamp, convert it, and check it:

```bash
vrk grab https://api.example.com/status \
  | vrk kv get --ns cache last_updated \
  | vrk epoch --iso
```

Pin relative times in scripts with `--at` so results are deterministic regardless of when the script runs:

```bash
vrk epoch '+7d' --at 1740009600 --json
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--iso` | | bool | false | Output as ISO 8601 string instead of Unix integer |
| `--json` | `-j` | bool | false | Emit JSON with all representations: `{input, unix, iso, ref?, tz?}` |
| `--tz` | | string | `""` | Timezone for `--iso` or `--json` output. IANA name (`America/New_York`) or offset (`+05:30`). Requires `--iso` or `--json`. |
| `--now` | | bool | false | Print current Unix timestamp without reading stdin |
| `--at` | | string | `""` | Reference timestamp for relative input (Unix integer). Makes scripts deterministic. |
| `--quiet` | `-q` | bool | false | Suppress stderr output; callers read exit codes only |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (I/O failure) |
| 2 | Usage error - unsupported format, ambiguous timezone, `--tz` without `--iso`/`--json`, too many arguments |
