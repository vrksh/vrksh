# base - Encoding converter - base64, base64url, hex, base32

When to use: encode or decode base64, base64url, hex, or base32 with consistent behavior across macOS and Linux. Two subcommands: encode and decode.
Composes with: digest, jwt, kv

| Flag      | Short | Type   | Description                                                  |
|-----------|-------|--------|--------------------------------------------------------------|
| `--to`    |       | string | Target encoding for `encode`: base64, base64url, hex, base32 |
| `--from`  |       | string | Source encoding for `decode`: same set                       |
| `--quiet` | `-q`  | bool   | Suppress stderr                                              |

Exit 0: success (including empty input)
Exit 1: invalid input data for the chosen decoding
Exit 2: no subcommand, missing --to/--from, unsupported encoding, unknown flag, interactive terminal

Example:

    echo 'hello' | vrk base encode --to base64
    # aGVsbG8=

Anti-pattern:
- Don't use the system base64 command in cross-platform scripts. macOS base64 wraps at 76 characters, Linux doesn't. vrk base output is consistent everywhere.
