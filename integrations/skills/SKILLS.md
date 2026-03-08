# vrksh Skills

This file is embedded in the `vrk` binary and served via `vrk --skills`.
It is the agent-facing reference for using vrksh tools in AI pipelines.

---

## jwt — JWT Inspector

Decodes a JWT and prints the payload as JSON. Does not verify signatures.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--claim <name>` | `-c` | Print a single claim value as plain text |
| `--expired` | `-e` | Exit 1 if the token is expired |
| `--json` | `-j` | Emit full envelope: `{header, payload, expires_in}` |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success — token decoded, condition met |
| 1 | Runtime error — invalid token, expired (with `--expired`), claim not found |
| 2 | Usage error — no input provided, unknown flag |

### Examples

```bash
# Decode a token
vrk jwt "$TOKEN"

# Extract a single claim
vrk jwt --claim sub "$TOKEN"

# Full envelope with expiry info
vrk jwt --json "$TOKEN"

# Guard: exit 1 if token is expired
vrk jwt --expired "$TOKEN"

# Pipe form
echo "$TOKEN" | vrk jwt --claim sub
```

### Compose patterns

```bash
# Extract sub and use as a key lookup
SUB=$(vrk jwt --claim sub "$TOKEN")
vrk kv get "user:$SUB"

# Decode token from an env var and check expiry before making an API call
vrk jwt --expired "$AUTH_TOKEN" && curl -H "Authorization: Bearer $AUTH_TOKEN" ...

# Inspect a token mid-pipeline
echo "$TOKEN" | vrk jwt --json | jq '.payload.exp'
```

### Gotchas

- `--expired` exits 1 only if the `exp` claim is present **and** in the past.
  A token with no `exp` claim is treated as never-expiring and exits 0.
- `--json` never exits 1 for an expired token — it just sets `expires_in` to `"expired"`.
  Use `--expired` when you need the exit code guard.
- Default output (no flags) prints the payload only. Use `--json` to also get the header.
- Stdout is always empty on error — errors go to stderr only.
