# kv - Key-value store - SQLite-backed, namespaces, TTL, atomic counters

When to use: persist state across pipeline runs -- cache results, track counters, store metadata.
Composes with: prompt, uuid, jwt, emit

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--ns` | | string | Namespace (default: "default") |
| `--ttl` | | duration | Expiry duration for set (0 = no expiry) |
| `--dry-run` | | bool | Print intent without writing (set only) |
| `--by` | | int | Delta for incr/decr (default: 1, must be >= 1) |
| `--json` | `-j` | bool | Emit errors as JSON (get, incr, decr) |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success (set, del, list, incr, decr, get when found)
Exit 1: key not found, not a number, database error
Exit 2: missing subcommand, unknown subcommand, unknown flag, --by < 1

Example:

    vrk kv set --ns cache mykey "myvalue" --ttl 1h
    vrk kv get --ns cache mykey

Anti-pattern:
- Don't store secrets in kv without --ns isolation -- the database is plaintext SQLite at ~/.vrk.db.
