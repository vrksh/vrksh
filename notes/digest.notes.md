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
