---
title: "vrk digest"
description: "Hash anything with sha256, md5, or sha512. Verify with --compare. Sign with --hmac. One tool, no flags to memorize."
og_title: "vrk digest - hash, verify, and HMAC from the command line"
tool: digest
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Hashes strings, stdin, or files with SHA-256, MD5, or SHA-512. Also computes HMACs for verifying webhook signatures with constant-time comparison, so timing attacks are not a concern. Streams input, so it handles files of any size without loading them into memory.

## The problem

You need to verify a webhook signature or compare file hashes and the openssl command is different for HMAC vs plain hash. You forget the -binary flag, get hex-encoded input to the HMAC, and the signature never matches.

## Before and after

**Before**

```bash
echo -n "$PAYLOAD" | openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" -binary | xxd -p
# -binary is easy to forget; without it you HMAC the hex string, not the bytes
# output format differs between openssl versions
```

**After**

```bash
echo -n "$PAYLOAD" | vrk digest --hmac --key "$WEBHOOK_SECRET" --bare
```

## Example

```bash
vrk digest 'hello'
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success, hash written, or --verify matched |
| 1 | File not found, read error, or --verify mismatch |
| 2 | Unknown algorithm, --hmac without --key, --verify without --hmac |

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

