# emit - Structured logger - wraps stdin lines as JSONL log records

When to use: wrap unstructured text output as structured JSONL with timestamps and levels.
Composes with: prompt, mask, kv, validate

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--level` | `-l` | string | Log level: debug, info (default), warn, error |
| `--tag` | | string | Add "tag" field to every record |
| `--msg` | | string | Override message; stdin treated as JSON to merge extra fields |
| `--parse-level` | | bool | Auto-detect level from line prefix (ERROR, WARN, INFO, DEBUG) |

Exit 0: success
Exit 1: I/O error
Exit 2: interactive stdin, unknown flag, invalid --level value

Example:

    ./deploy.sh 2>&1 | vrk emit --tag deploy --parse-level

Anti-pattern:
- Don't expect a --json flag -- emit output is already JSONL. Every line is a JSON record.
