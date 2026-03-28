---
title: "vrk digest"
description: "Universal hasher - sha256/md5/sha512, --hmac, --compare"
tool: digest
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You need to verify a download, check whether two files are identical, or compute an HMAC for a webhook signature. You reach for `shasum`, `md5sum`, or `openssl dgst` and immediately run into the friction: the output format differs between Linux and macOS, the flag order for HMAC varies by tool, and piping into a hex comparison requires stripping whitespace and the filename from the output. Three tools, three flag conventions, zero consistency.

`vrk digest` does all of it with one interface. Same flags on every platform, streaming input so large files never buffer into memory, and HMAC support built in with constant-time verification.

## The fix

```bash
echo 'hello' | vrk digest
vrk digest 'hello'
```

Both produce the same output: `sha256:<hex>`. SHA-256 is the default. Swap to another algorithm with `--algo`.

## Walkthrough

**Hashing a string or stdin**

Positional argument and stdin are equivalent. The positional form hashes the string as-is, without adding a newline. The stdin form hashes bytes verbatim, including any trailing newline from `echo`.

```bash
$ vrk digest 'hello world'
sha256:b94d27b9934d3e08a52e52d7da7dabfac484efe37a5380ee9088f7ace2efcde9
$ echo 'hello world' | vrk digest
sha256:a948904f2f0f479b8f8197694b30184b0d2ed1c1cd2a1ec0fb85d299a192a447
```

Note: these two produce different hashes because `echo` appends a newline.

**Choosing an algorithm**

```bash
$ vrk digest --algo md5 'hello'
md5:5d41402abc4b2a76b9719d911017c592
$ vrk digest --algo sha512 'hello'
sha512:9b71d224bd62f3785d96d46ad3ea3d73319bfbc2890caadae2dff72519673ca72323c3d99ba5c11d7c7acc6e14b8c5da0c4663475c2e5c3adef46f73bcdec043
```

`--bare` strips the `algo:` prefix when you need just the hex:

```bash
$ vrk digest --bare 'hello'
2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824
```

**Hashing a file**

`--file` streams the file through the hash. Safe for arbitrarily large inputs.

```bash
vrk digest --file /path/to/archive.tar.gz
```

Repeat `--file` for multiple files:

```bash
vrk digest --file a.tar.gz --file b.tar.gz
```

**Comparing two files**

`--compare` requires at least two `--file` values and reports whether all hashes match. Exit code is always 0 - the match result is in the output.

```bash
vrk digest --compare --file original.tar.gz --file copy.tar.gz
```

With `--json`:

```json
{"files":["original.tar.gz","copy.tar.gz"],"algo":"sha256","hashes":["...","..."],"match":true}
```

**Computing an HMAC**

`--hmac` requires `--key`. The key is the raw string value of the flag - no hex encoding.

```bash
echo 'payload' | vrk digest --hmac --key 'my-secret'
```

**Verifying a webhook signature**

`--verify` takes the known hex and exits 0 on match, 1 on mismatch. Uses constant-time comparison so there is no timing oracle.

```bash
echo 'payload' | vrk digest --hmac --key "$WEBHOOK_SECRET" --verify "$X_HUB_SIGNATURE"
```

Exit 0 means the signature is valid. Exit 1 means it is not.

**JSON output**

`--json` emits a single object instead of the `algo:hash` line. For HMAC mode, the field is `hmac` instead of `hash`.

```bash
$ echo 'data' | vrk digest --json
{"algo":"sha256","hash":"6667b2d1aab6a00caa5aee5af8ad9f1465e567abf1c209d15727d57b3e8f6e5f","input_bytes":5}
```

## Pipeline example

```bash
vrk grab --raw https://example.com/file.tar.gz | vrk digest --algo sha256 --bare
```

Download a file and print only the SHA-256 hex - no filename, no prefix. Paste it directly into a checksum comparison.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--algo` | `-a` | string | `"sha256"` | Hash algorithm: `sha256`, `md5`, `sha512` |
| `--bare` | `-b` | bool | false | Output the hex hash only, without the `algo:` prefix |
| `--file` | | []string | none | Path to a file to hash (repeatable; takes priority over stdin) |
| `--compare` | | bool | false | Compare hashes of all `--file` inputs and report `match: true/false`; always exits 0 |
| `--hmac` | | bool | false | Compute HMAC-<algo> instead of a plain hash |
| `--key` | `-k` | string | `""` | HMAC secret key (required with `--hmac`) |
| `--verify` | | string | `""` | Known HMAC hex to verify against; exits 0 on match, 1 on mismatch |
| `--json` | `-j` | bool | false | Emit a JSON object instead of the `algo:hash` line |
| `--quiet` | `-q` | bool | false | Suppress stderr output; exit codes are unaffected |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success - hash written, or `--compare` completed (regardless of match result), or `--verify` matched |
| 1 | Runtime error - file not found, read error, write error, or `--verify` mismatch |
| 2 | Usage error - unknown algorithm, `--hmac` without `--key`, `--verify` without `--hmac`, `--compare` with fewer than two files, `--bare` and `--json` together, interactive TTY with no input |
