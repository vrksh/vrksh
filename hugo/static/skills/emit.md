# emit - Structured logger - wraps stdin lines as JSONL log records

When to use: wrap unstructured command output into JSONL log records with timestamps and levels. Use --parse-level to auto-detect ERROR/WARN/INFO prefixes, --tag to label by pipeline stage.
Composes with: prompt, mask, kv, validate

| Flag            | Short | Type   | Description                                                   |
|-----------------|-------|--------|---------------------------------------------------------------|
| `--level`       | `-l`  | string | Log level: debug, info (default), warn, error                 |
| `--tag`         |       | string | Add "tag" field to every record                               |
| `--msg`         |       | string | Override message; stdin treated as JSON to merge extra fields |
| `--parse-level` |       | bool   | Auto-detect level from line prefix (ERROR, WARN, INFO, DEBUG) |

Exit 0: success
Exit 1: I/O error
Exit 2: interactive stdin, unknown flag, invalid --level value

Example:

    ./deploy.sh 2>&1 | vrk emit --tag deploy --parse-level

Anti-pattern:
- Don't use emit on data that's already JSONL. Emit wraps plain text lines as JSON records. For existing JSONL, use validate or assert instead.
