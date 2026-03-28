# assert - Pipeline condition check - jq conditions, --contains, --matches

When to use: gate a pipeline on a condition -- pass data through or kill the pipeline.
Composes with: prompt, validate, grab, kv

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `<condition>` | | string | jq-compatible condition (positional, repeatable) |
| `--contains` | | string | Assert stdin contains substring (plain text mode) |
| `--matches` | | string | Assert stdin matches Go regex (plain text mode) |
| `--message` | `-m` | string | Custom failure message |
| `--json` | `-j` | bool | Emit `{"passed":bool,...}` to stdout |
| `--quiet` | `-q` | bool | Suppress stderr on failure |

Exit 0: assertion passed, stdin passed through byte-for-byte
Exit 1: assertion failed, JSON parse error, I/O error
Exit 2: no condition, no stdin, mode conflict, invalid regex

Example:

    echo '{"score":0.9}' | vrk assert '.score > 0.8' | vrk kv set result

Anti-pattern:
- Don't use positional jq conditions on plain text input -- it will exit 1 with a JSON parse error. Use --contains or --matches instead.
