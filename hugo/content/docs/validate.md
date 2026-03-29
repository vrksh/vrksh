---
title: "vrk validate"
description: "JSONL schema validator - --schema, --strict, --fix, --json"
tool: validate
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → validate → stdout`

Exit 0 All records passed · Exit 1 Record failed in --strict mode, or scanner error · Exit 2 --schema missing, schema JSON invalid, unknown flag

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--schema` | -s | string | JSON schema inline or path to .json file (required) |
| `--strict` |   | bool | Exit 1 on first invalid record |
| `--fix` |   | bool | Send invalid records to vrk prompt for repair |
| `--json` | -j | bool | Append metadata trailer after all output |
| `--quiet` | -q | bool | Suppress stderr output |

## Example

```bash
cat records.jsonl | vrk validate --schema '{"name":"string","age":"number"}' --strict
```
