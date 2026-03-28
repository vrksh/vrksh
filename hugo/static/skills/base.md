# base - Encoding converter - base64, base64url, hex, base32

When to use: encode or decode binary data for transport or storage.
Composes with: digest, jwt, kv

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--to` | | string | Target encoding for `encode`: base64, base64url, hex, base32 |
| `--from` | | string | Source encoding for `decode`: same set |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success (including empty input)
Exit 1: invalid input data for the chosen decoding
Exit 2: no subcommand, missing --to/--from, unsupported encoding, unknown flag, interactive terminal

Example:

    echo 'hello' | vrk base encode --to base64
    # aGVsbG8=

Anti-pattern:
- Don't pipe line-wrapped base64 (e.g. from `base64 -w76`) -- strip internal newlines with `tr -d '\n'` first.
