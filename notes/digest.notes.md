## How it works

`vrk digest` prints the hash (sha256 by default, also md5 and sha512) or the HMAC of its input.

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
sha256:c5ac14f502d9bc308ca094278d0adddaaad100b9fe09e38db93f1c17d0c09c9a
```

digest reads the file as a stream, so it works on files larger than memory.

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

To check if files have identical content, compare them:

```bash
vrk digest --file original.txt --file copy.txt --compare
```

### HMAC computation and verification

```bash
# Compute HMAC
$ SIG=$(echo 'message' | vrk digest --hmac --key 'secret')

# Verify HMAC (constant-time comparison)
$ echo 'message' | vrk digest --hmac --key 'secret' --verify "$SIG"
$ echo $?
0
```

`--key` puts the secret on the command line, where `ps` and shell history can see it. `--key-env NAME` reads it from an environment variable instead, and `--key-file PATH` from a file (one trailing newline is dropped). Pass only one of the three flags.

```bash
$ export WEBHOOK_SECRET='secret'
$ SIG=$(echo 'message' | vrk digest --hmac --key-env WEBHOOK_SECRET)
$ echo 'message' | vrk digest --hmac --key-env WEBHOOK_SECRET --verify "$SIG"
$ echo $?
0
```

`--verify` accepts the `sha256:<hex>` form that digest prints, or bare hex. If the prefix names a different algorithm than `--algo`, digest exits 2. An empty value (an unset `$SIG`, or just `sha256:`) is a mismatch and exits 1, so a missing signature header never passes. `--verify` reads stdin or a positional argument. If you use it with `--file`, digest exits 2.

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
usage error: digest: unknown algorithm "sha384": want sha256, md5, sha512
$ echo $?
2
```

HMAC without key:

```bash
$ echo 'hello' | vrk digest --hmac
usage error: digest: --hmac requires --key, --key-env or --key-file
$ echo $?
2
```

Verify mismatch:

```bash
$ echo 'hello' | vrk digest --hmac --key 'secret' --verify 'sha256:0000000000000000000000000000000000000000000000000000000000000000'
error: digest: hmac mismatch
$ echo $?
1
```
