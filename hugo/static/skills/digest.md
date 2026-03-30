# digest - Universal hasher - sha256/md5/sha512, --hmac, --compare

When to use: hash data with SHA-256 (default), MD5, or SHA-512. Verify checksums with --compare, compute HMACs with --hmac. Works identically on macOS and Linux.
Composes with: kv, base, prompt

| Flag        | Short | Type   | Description                                                |
|-------------|-------|--------|------------------------------------------------------------|
| `--algo`    | `-a`  | string | Algorithm: sha256 (default), md5, sha512                   |
| `--bare`    | `-b`  | bool   | Hash only, no algo: prefix                                 |
| `--file`    |       | string | File to hash (repeatable)                                  |
| `--compare` |       | bool   | Compare hashes of all --file inputs                        |
| `--hmac`    |       | bool   | Compute HMAC (requires --key)                              |
| `--key`     | `-k`  | string | HMAC secret key                                            |
| `--verify`  |       | string | Compare computed HMAC against this hex; exit 1 on mismatch |
| `--json`    | `-j`  | bool   | JSON output with metadata                                  |

Exit 0: success, --compare result, --verify match
Exit 1: --verify mismatch, file not found, I/O error
Exit 2: unknown flag/algo, --hmac without --key, --bare + --json, interactive terminal

Example:

    echo 'payload' | vrk digest --hmac --key mysecret --bare

Anti-pattern:
- Don't use MD5 for security purposes - only for checksums where collision resistance doesn't matter. Use SHA-256 (the default) for integrity verification.
- Don't compare HMAC values with string equality. Use --verify for constant-time comparison that resists timing attacks.
