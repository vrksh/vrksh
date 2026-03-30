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

You need to hash a file and compare it against a known checksum. `sha256sum` exists on Linux but not macOS. `shasum -a 256` exists on macOS but the output format differs. You end up with `if [ "$(shasum -a 256 < file | cut -d' ' -f1)" = "$EXPECTED" ]` and it works until someone passes a filename with spaces.

`vrk digest` hashes with SHA-256 (default), MD5, or SHA-512. It reads from stdin, positional arguments, or files with `--file`. Compare hashes with `--compare`, compute HMACs with `--hmac`, and verify them with `--verify`. Uses constant-time comparison to resist timing attacks.

## The problem

You download a binary and want to verify its SHA-256 checksum. On macOS you use shasum -a 256. On Linux you use sha256sum. The output formats differ. You write a verification script and it breaks when someone runs it on the other OS. For HMAC verification, you reach for openssl and the flag syntax is even worse.

## Before and after

**Before**

```bash
# macOS: shasum -a 256 file.tar.gz
# Linux: sha256sum file.tar.gz
# Different output formats, different commands
```

**After**

```bash
vrk digest --file file.tar.gz
```

## Example

```bash
vrk digest --file release.tar.gz --compare
```

## Exit codes

| Code | Meaning                                                          |
|------|------------------------------------------------------------------|
| 0    | Success, hash written, or --verify matched                       |
| 1    | File not found, read error, or --verify mismatch                 |
| 2    | Unknown algorithm, --hmac without --key, --verify without --hmac |

## Flags

| Flag        | Short | Type     | Description                                |
|-------------|-------|----------|--------------------------------------------|
| `--algo`    | -a    | string   | Hash algorithm: sha256, md5, sha512        |
| `--bare`    | -b    | bool     | Output hex hash only, without algo: prefix |
| `--file`    |       | []string | Path to file to hash (repeatable)          |
| `--compare` |       | bool     | Compare hashes of all --file inputs        |
| `--hmac`    |       | bool     | Compute HMAC instead of plain hash         |
| `--key`     | -k    | string   | HMAC secret key (required with --hmac)     |
| `--verify`  |       | string   | Known HMAC hex to verify against           |
| `--json`    | -j    | bool     | Emit JSON object instead of algo:hash line |
| `--quiet`   | -q    | bool     | Suppress stderr output                     |


<!-- notes - edit in notes/digest.notes.md -->

## How it works

### Hash from stdin

```bash
$ echo 'hello' | vrk digest
sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
```

Default format is `algo:hash`. Use `--bare` for just the hash:

```bash
$ echo 'hello' | vrk digest --bare
5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
```

### Hash a file

```bash
$ vrk digest --file release.tar.gz
sha256:a1b2c3d4e5f6...
```

Streams the file, so it works on files larger than available memory.

### Different algorithms

```bash
$ echo 'hello' | vrk digest --algo md5
md5:b1946ac92492d2347c6235b4d2611184

$ echo 'hello' | vrk digest --algo sha512
sha512:e7c22b994c59d...
```

### JSON output

```bash
$ echo 'hello' | vrk digest --algo md5 --json
{"algo":"md5","hash":"b1946ac92492d2347c6235b4d2611184","input_bytes":6}
```

### Compare file hashes

Check if multiple files have identical content:

```bash
vrk digest --file original.txt --file copy.txt --compare
```

### HMAC computation and verification

```bash
# Compute HMAC
$ echo 'message' | vrk digest --hmac --key 'secret'
sha256:...

# Verify HMAC (constant-time comparison)
$ echo 'message' | vrk digest --hmac --key 'secret' --verify 'expected-hash'
$ echo $?
0
```

The `--verify` flag uses constant-time comparison to prevent timing attacks.

## Pipeline integration

### Cache-key generation for LLM responses

```bash
# Use content hash as a cache key
KEY=$(cat document.txt | vrk digest --bare)
CACHED=$(vrk kv get --ns llm-cache "$KEY" 2>/dev/null)
if [ -z "$CACHED" ]; then
  RESULT=$(cat document.txt | vrk prompt --system 'Summarize')
  vrk kv set --ns llm-cache "$KEY" "$RESULT" --ttl 24h
  echo "$RESULT"
else
  echo "$CACHED"
fi
```

### Verify downloaded files

```bash
vrk digest --file download.tar.gz --bare | \
  vrk assert --contains "$EXPECTED_HASH" -m 'Checksum mismatch'
```

## When it fails

Unknown algorithm:

```bash
$ echo 'hello' | vrk digest --algo sha384
usage error: digest: unsupported algorithm "sha384"
$ echo $?
2
```

HMAC without key:

```bash
$ echo 'hello' | vrk digest --hmac
usage error: digest: --key is required with --hmac
$ echo $?
2
```

Verify mismatch:

```bash
$ echo 'hello' | vrk digest --hmac --key 'secret' --verify 'wrong'
error: digest: HMAC mismatch
$ echo $?
1
```
