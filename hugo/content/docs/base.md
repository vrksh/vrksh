---
title: "vrk base"
description: "Encoding converter - base64, base64url, hex, base32"
tool: base
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You need to encode a JWT payload, decode a base64 config value from a Kubernetes secret, convert a hex string to raw bytes, or check what is inside a base64url token. Each encoding lives in a different CLI tool - `base64`, `xxd`, `basenc`, `python3 -c` - and each behaves differently on Linux versus macOS. `base64 -d` is `-D` on macOS. `xxd -r -p` requires a specific whitespace layout. `basenc --base32` is not available on BSD at all.

`vrk base` handles all four encodings with a single consistent interface. Same flags everywhere, two subcommands, and it accepts both positional arguments and stdin so it fits in any pipeline.

## The fix

```bash
echo 'hello' | vrk base encode --to base64
vrk base decode --from base64 'aGVsbG8='
```

## Walkthrough

**Encoding to each format**

All four encodings use the same `encode --to` flag. The output is always a single line of ASCII.

```bash
echo 'hello' | vrk base encode --to base64
echo 'hello' | vrk base encode --to base64url
echo 'hello' | vrk base encode --to hex
echo 'hello' | vrk base encode --to base32
```

<!-- output: verify against binary -->

`base64url` uses the URL-safe alphabet (`-` and `_` instead of `+` and `/`) with no padding. `hex` output is lowercase. `base32` output is uppercase with `=` padding.

**Decoding from each format**

```bash
echo 'aGVsbG8=' | vrk base decode --from base64
echo 'aGVsbG8' | vrk base decode --from base64url
echo '68656c6c6f' | vrk base decode --from hex
echo 'NBSWY3DPEB3W64TMMQ======' | vrk base decode --from base32
```

Decoded output is raw bytes with no trailing newline added. If you decode to text and pipe to another tool, that tool receives the exact bytes the encoder produced.

**Positional argument form**

Both `encode` and `decode` accept a positional argument so you can hash inline without `echo`:

```bash
vrk base encode --to hex 'hello world'
vrk base decode --from hex '68656c6c6f20776f726c64'
```

<!-- output: verify against binary -->

**Handling trailing newlines**

`echo` appends a newline. `vrk base` strips exactly one trailing newline from stdin before encoding - so `echo 'hello' | vrk base encode --to base64` encodes `hello`, not `hello\n`. If you need to encode the newline too, use `printf 'hello\n'` and pipe it, then supply the raw bytes rather than relying on the strip behavior. The strip is documented and stable.

**Decoding a JWT payload**

JWT payloads are base64url-encoded with no padding. Combine `vrk jwt` to extract the payload claim, then decode it:

```bash
vrk jwt --claim payload "$TOKEN" | vrk base decode --from base64url
```

<!-- output: verify against binary -->

**Decoding a Kubernetes secret**

```bash
kubectl get secret my-secret -o jsonpath='{.data.password}' | vrk base decode --from base64
```

<!-- output: verify against binary -->

**Empty input**

Empty input after stripping the trailing newline produces no output and exits 0. This is intentional - empty input is not an error.

## Pipeline example

```bash
vrk jwt --claim payload "$TOKEN" | vrk base decode --from base64url
```

Extract the raw payload claim from a JWT and decode it from base64url to inspect the JSON contents. Combine with `vrk kv set` to cache the decoded payload, or pipe to `vrk prompt` to summarise it.

## Flags

**Subcommand: `encode`**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--to` | | string | `""` | Target encoding: `base64`, `base64url`, `hex`, `base32` (required) |
| `--quiet` | `-q` | bool | false | Suppress stderr output; exit codes are unaffected |

**Subcommand: `decode`**

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--from` | | string | `""` | Source encoding: `base64`, `base64url`, `hex`, `base32` (required) |
| `--quiet` | `-q` | bool | false | Suppress stderr output; exit codes are unaffected |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success - encoded or decoded output written to stdout, or empty input after strip |
| 1 | Runtime error - invalid encoded input (bad characters, wrong padding) |
| 2 | Usage error - missing subcommand, unknown subcommand, `--to`/`--from` missing or unsupported, interactive TTY with no input |
