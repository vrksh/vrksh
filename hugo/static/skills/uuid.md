# uuid - UUID generator - v4/v7, --count, --json

When to use: generate unique IDs for pipeline runs, database keys, or correlation.
Composes with: kv, prompt, chunk

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--v7` | | bool | Generate v7 (time-ordered) UUID instead of v4 |
| `--count` | `-n` | int | Number of UUIDs to generate (default: 1) |
| `--json` | `-j` | bool | Emit `{uuid, version, generated_at}` per UUID (JSONL) |
| `--quiet` | `-q` | bool | Suppress stderr |

Exit 0: success
Exit 2: --count < 1, unknown flag

Example:

    ID=$(vrk uuid) && vrk prompt "summarise" < doc.txt | vrk kv set "result:$ID"

Anti-pattern:
- Don't pipe input to uuid -- stdin is silently ignored. It generates from embedded randomness only.
