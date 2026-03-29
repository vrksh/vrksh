---
title: "vrk uuid"
description: "UUID generator - v4/v7, --count, --json"
tool: uuid
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Generates UUIDs - v4 (random) or v7 (time-ordered). v7 UUIDs sort by creation time, which makes them useful as database primary keys that stay roughly ordered. Consistent output format across macOS and Linux.

## The problem

You need a UUID and uuidgen is not installed, or it only generates v4 and you need v7 for time-ordered database keys. Python's uuid module does not support v7. The uuidgen output format varies across platforms.

## Before and after

**Before**

```bash
uuidgen
# macOS: uppercase, Linux: lowercase
# no v7 support on either platform
# python3 -c "import uuid; print(uuid.uuid4())" for portable v4
```

**After**

```bash
vrk uuid --v7
```

## Example

```bash
vrk uuid --v7
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (generation failure) |
| 2 | --count less than 1, unknown flag |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--v7` |   | bool | Generate a v7 (time-ordered) UUID instead of v4 |
| `--count` | -n | int | Number of UUIDs to generate |
| `--json` | -j | bool | Emit JSON with uuid, version, generated_at |
| `--quiet` | -q | bool | Suppress stderr output |

