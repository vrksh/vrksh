# epoch - Timestamp converter - unix to ISO, relative time, --tz

When to use: convert between Unix timestamps and ISO 8601 dates, or compute relative offsets like +3d or -1h. Works identically on macOS and Linux, unlike the date command.
Composes with: kv, jwt, prompt

| Flag      | Short | Type   | Description                                                     |
|-----------|-------|--------|-----------------------------------------------------------------|
| `--iso`   |       | bool   | Output as ISO 8601 instead of Unix integer                      |
| `--json`  | `-j`  | bool   | Emit `{input, unix, iso, ref?, tz?}`                            |
| `--tz`    |       | string | Timezone: IANA name or +HH:MM offset (requires --iso or --json) |
| `--now`   |       | bool   | Print current Unix timestamp and exit                           |
| `--at`    |       | int    | Override reference time for relative input                      |
| `--quiet` | `-q`  | bool   | Suppress stderr                                                 |

Exit 0: success
Exit 2: unsupported format, missing sign on relative time, ambiguous timezone, --tz without --iso/--json

Example:

    echo '+3d' | vrk epoch --at 1740009600 --iso

Anti-pattern:
- Don't use the system date command in cross-platform scripts. macOS and Linux date have incompatible flags. vrk epoch behaves the same on both.
- Don't assume the reference time is UTC midnight. It's the current time unless you set --at. For reproducible scripts, always use --at.
