# validate - JSONL schema validator - --schema, --strict, --fix, --json

When to use: check JSONL records against a type schema. Valid records pass through, invalid ones are dropped or halt the pipeline (--strict).
Composes with: prompt, mask, emit, kv, jsonl

| Flag       | Short | Type   | Description                                                  |
|------------|-------|--------|--------------------------------------------------------------|
| `--schema` | `-s`  | string | Inline JSON schema or file path (required)                   |
| `--strict` |       | bool   | Exit 1 on first invalid line                                 |
| `--fix`    |       | bool   | Attempt LLM repair of invalid lines via vrk prompt           |
| `--json`   | `-j`  | bool   | Append `{"_vrk":"validate","total":N,"passed":N,"failed":N}` |

Exit 0: all valid, or invalid found but --strict not set
Exit 1: --strict and invalid line found; I/O error
Exit 2: --schema missing or invalid, unknown schema type, unknown flag

Example:

    cat records.jsonl | vrk validate --schema '{"name":"string","age":"number"}' --strict

Anti-pattern:
- Don't use validate as a retry trigger by default. Use prompt --schema --retry instead - it validates and retries in one call without a second pipeline stage.
- Don't validate after storing data. Validate between the LLM and the database so bad records never reach storage.
