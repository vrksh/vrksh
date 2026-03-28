---
title: "vrk kv"
description: "Key-value store - SQLite-backed, namespaces, TTL, atomic counters."
tool: kv
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → kv → stdout`

Exit 0 Success · Exit 1 Key not found, not a number, or database error · Exit 2 Usage error - unknown subcommand, missing args

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--ns` |   | string | Namespace (default "default") |
| `--quiet` | -q | bool | Suppress stderr output |
| `--ttl` |   | duration | Expiry duration (set only); 0 = no expiry |
| `--dry-run` |   | bool | Print intent without writing (set only) |
| `--json` | -j | bool | Emit errors as JSON (get, incr, decr) |
| `--by` |   | int | Delta for incr/decr (must be >= 1) |

## Example

```bash
vrk kv set --ns cache mykey "myvalue" --ttl 1h
```
