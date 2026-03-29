---
title: "vrk epoch"
description: "Convert between Unix timestamps and ISO dates. Supports relative time like +3d or -1h."
og_title: "vrk epoch - Unix timestamp and ISO date converter"
tool: epoch
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Converts between Unix timestamps and human-readable dates. You can pass a timestamp, an ISO date, or a relative offset like +3d or -1h. Works the same on macOS and Linux, unlike the date command which differs between platforms.

## The problem

You have a Unix timestamp from an API response and need to know what date it is. The date command syntax differs between macOS and Linux. date -d works on Linux, date -r works on macOS, and neither handles relative offsets like "+3d" natively.

## Before and after

**Before**

```bash
# Linux
date -d @1740009600
# macOS
date -r 1740009600
# relative? write Python
```

**After**

```bash
vrk epoch 1740009600 --iso
```

## Example

```bash
vrk epoch 1740009600 --iso
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (I/O failure) |
| 2 | Unsupported format, ambiguous timezone, --tz without --iso/--json |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--iso` |   | bool | Output as ISO 8601 string instead of Unix integer |
| `--json` | -j | bool | Emit JSON with all representations |
| `--tz` |   | string | Timezone for --iso or --json output (IANA name or offset) |
| `--now` |   | bool | Print current Unix timestamp without reading stdin |
| `--at` |   | string | Reference timestamp for relative input (makes scripts deterministic) |
| `--quiet` | -q | bool | Suppress stderr output |

