---
title: "vrk validate"
description: "JSONL schema validator - --schema, --strict, --fix, --json"
tool: validate
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

Your pipeline produces JSONL. Maybe it comes from an LLM, maybe from a transformation step, maybe from an external API. Before you send those records downstream - to a database, to another tool, to a reporting system - you need to know every record has the right shape. A missing field or a wrong type silently corrupts everything that reads it. Without a gate in the pipeline, you find out at write time, or worse, at query time.

`vrk validate` is that gate. Valid records pass through to stdout. Invalid records are warned to stderr. At the end, you know exactly how many passed and how many failed.

## The fix

```bash
cat records.jsonl | vrk validate --schema '{"name":"string","age":"number"}'
```

Schema is a JSON object mapping field names to types. The five allowed types are `string`, `number`, `boolean`, `array`, and `object`. Schema keys are required fields - extra keys in records are ignored.

## Walkthrough

**Basic validation - pass-through with warnings**

Valid records pass through to stdout. Invalid ones are warned to stderr and dropped. The pipeline continues.

```bash
cat records.jsonl | vrk validate --schema '{"id":"string","score":"number"}'
```

**Strict mode - stop on first failure**

When you need all-or-nothing guarantees, `--strict` exits 1 the moment any record fails. Nothing invalid makes it downstream.

```bash
cat records.jsonl | vrk validate --schema '{"id":"string","score":"number"}' --strict
```

**Schema from a file**

For complex or reused schemas, write the schema to a JSON file and pass the path to `--schema`.

```bash
cat schema.json
# {"user_id":"string","event":"string","ts":"number","payload":"object"}

cat events.jsonl | vrk validate --schema schema.json
```

**Counting results with `--json`**

The `--json` flag appends a metadata record after all data records:

```bash
cat records.jsonl | vrk validate --schema '{"name":"string"}' --json
```

The final line looks like:

```json
{"_vrk":"validate","total":100,"passed":98,"failed":2}
```

**Auto-repair with `--fix`**

When `--fix` is active, invalid records are sent to `vrk prompt` with the schema and a repair instruction. If the model produces a valid record, it replaces the original. If not, the original is warned and dropped. This uses LLM credits - use it for small correction jobs, not bulk pipelines.

```bash
cat records.jsonl | vrk validate --schema '{"name":"string","score":"number"}' --fix
```

**Combining `--fix` and `--strict`**

With both flags, `--fix` attempts repair first. Only records that still fail after the repair attempt trigger a strict exit.

```bash
cat records.jsonl | vrk validate --schema '{"name":"string"}' --fix --strict
```

## Pipeline example

```bash
vrk grab https://api.example.com/data | vrk jsonl | vrk validate --schema '{"name":"string","age":"number"}' --strict
```

Fetch a JSON API response, convert it to JSONL (one record per line), then validate every record against the schema. `--strict` means the first malformed record stops the pipeline with exit 1, so nothing invalid reaches whatever comes next.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--schema` | `-s` | string | `""` | JSON schema inline or path to a `.json` file (required) |
| `--strict` | | bool | false | Exit 1 on the first invalid record instead of continuing |
| `--fix` | | bool | false | Send invalid records to `vrk prompt` for repair before failing |
| `--json` | `-j` | bool | false | Append a `{"_vrk":"validate","total":N,"passed":N,"failed":N}` record after all output |
| `--quiet` | `-q` | bool | false | Suppress stderr output; exit codes are unaffected |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | All records passed (or all failures were repaired by `--fix`) |
| 1 | A record failed validation in `--strict` mode, or a scanner error occurred |
| 2 | Usage error - `--schema` is missing, schema JSON is invalid, or unknown flag |
