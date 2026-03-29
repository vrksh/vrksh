---
title: "vrk moniker"
description: "Generate memorable names for run IDs, job labels, and temp dirs. No more UUIDs in log output."
og_title: "vrk moniker - memorable name generator for run IDs and labels"
tool: moniker
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Generates memorable adjective-noun names like "bold-falcon" or "calm-otter" from a curated word list. Useful for pipeline runs, temp directories, and job labels where humans need to reference things by name. Deterministic with a seed, and no duplicates within a batch.

## The problem

During an incident you have six parallel pipeline runs. Someone on the call says "the failing one is 7f3a... no wait, 7f3b..." and everyone is looking at the wrong logs. UUIDs work as identifiers but fail as labels humans reference under pressure.

## Before and after

**Before**

```bash
RUN_ID=$(uuidgen | tr '[:upper:]' '[:lower:]')
echo "starting run $RUN_ID"
# incident call: "which run?" "7f3a9c2e... no, 7f3b..."
# six runs open, all starting with 7f3
```

**After**

```bash
RUN_ID=$(vrk moniker --seed "$BUILD_NUMBER")
echo "starting run $RUN_ID"
# incident call: "which run?" "bold-falcon"
```

## Example

```bash
vrk moniker --seed 42
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Word pool exhausted for requested count |
| 2 | --count less than 1, --words less than 2 |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--count` | -n | int | Number of names to generate |
| `--separator` |   | string | Word separator |
| `--words` |   | int | Words per name (minimum 2) |
| `--seed` |   | int64 | Random seed for deterministic output |
| `--json` | -j | bool | Emit JSON per name: {name, words} |
| `--quiet` | -q | bool | Suppress stderr output |

