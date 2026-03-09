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

---

## epoch — Timestamp Converter

Converts between Unix timestamps and ISO 8601 dates/times.
Default output is always a Unix integer. `--iso` switches to ISO 8601.
Input: positional argument or stdin.

### Flags

| Flag | Short | Description |
|------|-------|-------------|
| `--iso` | — | Output as ISO 8601 string instead of Unix integer |
| `--tz <zone>` | — | Timezone for `--iso` output; IANA name or `+HH:MM` offset |
| `--now` | — | Print current Unix timestamp and exit |
| `--at <ts>` | — | Override reference time for relative input (unix integer) |

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 2 | Usage error — unsupported format, missing sign, ambiguous timezone, no input, `--tz` without `--iso` |

### Relative time format

Must include a sign prefix: `+3d` (3 days from now), `-3d` (3 days ago).
Bare `3d` exits 2 with "sign required". Units: `s` `m` `h` `d` `w` (no months or years).
Negative relative times (`-3d`, `-2h`, etc.) work as positional args or via stdin.

### Examples

```bash
# Current timestamp
vrk epoch --now

# Unix integer passthrough
echo '1740009600' | vrk epoch

# ISO date to unix (midnight UTC)
echo '2025-02-20' | vrk epoch

# 3 days from now
echo '+3d' | vrk epoch

# 3 days ago — positional or stdin, both work
vrk epoch -3d
echo '-3d' | vrk epoch

# 3 days from now as ISO string
echo '+3d' | vrk epoch --iso

# Deterministic: override reference time so pipelines are reproducible
echo '+3d' | vrk epoch --at 1740009600     # always 1740268800

# Convert unix to ISO with timezone offset
echo '1740009600' | vrk epoch --iso --tz +05:30

# Convert unix to ISO with IANA timezone
echo '1740009600' | vrk epoch --iso --tz America/New_York
```

### Compose patterns

```bash
# Expiry timestamp for a KV entry: set TTL 7 days from now
EXPIRY=$(echo '+7d' | vrk epoch)
vrk kv set session:abc "$TOKEN" --ttl "$EXPIRY"

# Convert a stored timestamp back to human-readable
vrk kv get created_at | vrk epoch --iso

# Deterministic timestamp in CI scripts
CUTOFF=$(vrk epoch -30d --at "$BASELINE")
```

### Gotchas

- Relative times **must** be signed: `+3d` or `-3d`. Bare `3d` exits 2.
- Timezone abbreviations (IST, EST, PST) exit 2 — they are ambiguous across regions.
  Use full IANA names (`America/New_York`) or numeric offsets (`+05:30`).
- `--tz` requires `--iso`; using it without `--iso` exits 2.
- Unix integer input is passed through unchanged — timezone has no effect on it.
- Use `--at <ts>` to make pipelines involving relative times deterministic.
- `--now` is a boolean flag (prints current timestamp and exits). Use `--at` to set a reference.
- Negative integers (`-1000`) are valid pre-epoch Unix timestamps — pass via stdin to avoid flag parsing.
- Stdout is always empty on error — errors go to stderr only.
