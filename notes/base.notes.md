## Supported encodings

| Encoding | Flag value | Example output |
|----------|-----------|----------------|
| Base64 | `base64` | `aGVsbG8=` |
| Base64url | `base64url` | `aGVsbG8` (no padding, URL-safe) |
| Hex | `hex` | `68656c6c6f` |
| Base32 | `base32` | `NBSWY3DP` |

## How it works

### Encode

```bash
$ echo 'hello' | vrk base encode --to base64
aGVsbG8=

$ echo 'hello' | vrk base encode --to hex
68656c6c6f

$ echo 'hello' | vrk base encode --to base64url
aGVsbG8
```

### Decode

```bash
$ echo 'aGVsbG8=' | vrk base decode --from base64
hello

$ echo '68656c6c6f' | vrk base decode --from hex
hello
```

### Trailing newline handling

`echo` appends a newline. vrk base strips exactly one trailing newline before encoding, so `echo 'hello'` and `printf 'hello'` produce the same result. Use `printf 'hello\n'` if you want the newline included.

## Pipeline integration

### Decode a base64 field from JSON

```bash
echo "$JWT_PAYLOAD" | vrk base decode --from base64url | jq .
```

### Encode content for embedding in JSON

```bash
ENCODED=$(cat binary-file | vrk base encode --to base64)
echo "{\"data\":\"$ENCODED\"}" | vrk kv set --ns cache payload
```

## When it fails

Invalid base64 input:

```bash
$ echo '!!invalid!!' | vrk base decode --from base64
error: base: illegal base64 data at input byte 0
$ echo $?
1
```

Missing encoding flag:

```bash
$ echo 'hello' | vrk base encode
usage error: base encode: --to is required
$ echo $?
2
```
