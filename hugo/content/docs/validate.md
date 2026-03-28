---
title: "vrk validate"
description: "JSONL schema validator - --schema, --strict, --fix, --json"
tool: validate
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

Your LLM returned JSON. You think it matches your schema. You won't
find out it doesn't until something downstream breaks -- usually in
production, usually at 2am.

## The fix

```bash
$ echo '{"name":"Alice","age":30}' | vrk validate --schema '{"name":"string","age":"number"}'
{"name":"Alice","age":30}
```

Valid records pass through to stdout. No output is added, no fields are
modified. Silent success is the Unix convention -- if it printed nothing
extra, it worked.

When a record fails:

```bash
$ echo '{"name":"Alice","age":"thirty"}' | vrk validate --schema '{"name":"string","age":"number"}'
warning: validation failed: age expected number, got string
```

The invalid record is dropped from stdout and a warning goes to stderr.
Exit 0, because in default mode the stream continues. Use `--strict`
to make any failure exit 1 immediately.

## In a pipeline (after prompt)

```bash
echo "$CONTENT" | vrk prompt --system "Extract person data as JSON." \
  | vrk validate --schema '{"name":"string","age":"number"}' --strict
```

If validate exits 1, nothing downstream runs. The `--strict` flag is
the difference between "warn and continue" and "stop the pipeline."

When the output comes from `vrk prompt` directly, you can also use
`--schema` on prompt itself -- it instructs the LLM to produce valid
JSON and validates before emitting. Use `validate` separately when the
JSON comes from somewhere else (an API, a file, another tool).

## What a schema file looks like

```json
{"name":"string","age":"number","active":"boolean"}
```

A JSON object mapping field names to types. The five allowed types are
`string`, `number`, `boolean`, `array`, and `object`. Every key in the
schema is a required field. Extra keys in records are ignored.

For complex schemas, save to a file and pass the path:

```bash
cat records.jsonl | vrk validate --schema person.json
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--schema` | `-s` | string | `""` | JSON schema inline or path to a `.json` file (required) |
| `--strict` | | bool | `false` | Exit 1 on the first invalid record |
| `--fix` | | bool | `false` | Send invalid records to `vrk prompt` for repair |
| `--json` | `-j` | bool | `false` | Append `{"_vrk":"validate","total":N,"passed":N,"failed":N}` after all output |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | All records passed (or all failures repaired by `--fix`) |
| 1 | A record failed in `--strict` mode, or a scanner error occurred |
| 2 | Usage error - `--schema` missing, schema JSON invalid, unknown flag |
