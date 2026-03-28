# jwt - JWT inspector - decode, --claim, --expired, --valid

When to use: decode a JWT payload, extract claims, or check expiry without signature verification.
Composes with: kv, epoch, pct, base

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--claim` | `-c` | string | Print value of a single claim |
| `--expired` | `-e` | bool | Exit 1 if the token is expired |
| `--valid` | | bool | Exit 1 if expired, nbf in future, or iat in future |
| `--json` | `-j` | bool | Structured JSON output (shape depends on other flags) |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success or token is valid
Exit 1: token expired/invalid, claim not found, runtime error
Exit 2: no input, unknown flag, bad format

Example:

    SUB=$(vrk jwt --claim sub "$TOKEN")
    vrk kv get "user:$SUB"

Anti-pattern:
- Don't use --json alone to check expiry -- it never exits 1 for an expired token. Use --expired explicitly.
