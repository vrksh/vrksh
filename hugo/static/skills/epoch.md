# epoch - Timestamp converter - unix to ISO, relative time, --tz

When to use: convert between Unix timestamps and ISO 8601, or compute relative times for pipelines.
Composes with: kv, jwt, prompt

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--iso` | | bool | Output as ISO 8601 instead of Unix integer |
| `--json` | `-j` | bool | Emit `{input, unix, iso, ref?, tz?}` |
| `--tz` | | string | Timezone: IANA name or +HH:MM offset (requires --iso or --json) |
| `--now` | | bool | Print current Unix timestamp and exit |
| `--at` | | int | Override reference time for relative input |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success
Exit 2: unsupported format, missing sign on relative time, ambiguous timezone, --tz without --iso/--json

Example:

    echo '+3d' | vrk epoch --at 1740009600 --iso

Anti-pattern:
- Don't use timezone abbreviations (IST, EST) -- they are ambiguous and exit 2. Use IANA names or numeric offsets.
