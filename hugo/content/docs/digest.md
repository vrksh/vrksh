---
title: "vrk digest"
description: "universal hasher - sha256/md5/sha512, --hmac, --compare"
tool: digest
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → digest → stdout`

Exit 0 Success, hash written, or --verify matched · Exit 1 File not found, read error, or --verify mismatch · Exit 2 Unknown algorithm, --hmac without --key, --verify without --hmac

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--algo` | -a | string | Hash algorithm: sha256, md5, sha512 |
| `--bare` | -b | bool | Output hex hash only, without algo: prefix |
| `--file` |   | []string | Path to file to hash (repeatable) |
| `--compare` |   | bool | Compare hashes of all --file inputs |
| `--hmac` |   | bool | Compute HMAC instead of plain hash |
| `--key` | -k | string | HMAC secret key (required with --hmac) |
| `--verify` |   | string | Known HMAC hex to verify against |
| `--json` | -j | bool | Emit JSON object instead of algo:hash line |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
vrk digest 'hello'
```
