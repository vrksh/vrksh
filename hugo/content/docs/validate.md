---
title: "vrk validate"
description: "vrk validate checks LLM JSON output against a schema and exits 1 on mismatch. The pipeline stops before bad data reaches downstream systems. No error-handling code required."
meta_title: "vrk validate - JSON Schema Validator for Pipelines"
meta_lead: "vrk validate is a JSON schema validator for shell pipelines - exit 1 on mismatch stops the pipeline before bad data propagates."
og_title: "vrk validate - schema validation for LLM JSON output"
tool: validate
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk validate is a JSON schema validator for shell pipelines - exit 1 on mismatch stops the pipeline before bad data propagates.

## The problem

An LLM returns {"sentiment":"string","confidence":"number"} for 500 product reviews. 487 are correct. 13 return confidence as a string instead of a number. The analysis script crashes on `float("high")`. The entire pipeline reruns because there was no validation between the LLM and the database.

## The solution

`vrk validate` checks every JSON record in a JSONL stream against a type schema. Valid records pass through to stdout. Invalid records are dropped with a warning to stderr. `--strict` halts the pipeline on the first bad record. `--fix` sends invalid records to an LLM for repair.

## Before and after

**Before**

```bash
cat records.jsonl | while read line; do
  echo "$line" | jq -e '.name | type == "string"' > /dev/null && \
  echo "$line" | jq -e '.age | type == "number"' > /dev/null && \
  echo "$line"
done
```

**After**

```bash
cat records.jsonl | vrk validate --schema '{"name":"string","age":"number"}'
```

## Example

```bash
cat llm-output.jsonl | vrk validate --schema '{"sentiment":"string","confidence":"number"}' --strict
```

## Exit codes

| Code | Meaning                                                                    |
|------|----------------------------------------------------------------------------|
| 0    | All records passed                                                         |
| 1    | Record failed in --strict mode, or scanner error                           |
| 2    | --schema missing, schema JSON invalid, schema is JSON Schema, unknown flag |

## Flags

| Flag       | Short | Type   | Description                                                                                                                                |
|------------|-------|--------|--------------------------------------------------------------------------------------------------------------------------------------------|
| `--schema` | -s    | string | Flat key-to-type map, inline or path to .json file (required). Types: string, number, boolean, array, object. JSON Schema is not supported |
| `--strict` |       | bool   | Exit 1 on first invalid record                                                                                                             |
| `--fix`    |       | bool   | Send invalid records to vrk prompt for repair                                                                                              |
| `--json`   | -j    | bool   | Append metadata trailer after all output                                                                                                   |
| `--quiet`  | -q    | bool   | Suppress stderr output                                                                                                                     |


<!-- notes - edit in notes/validate.notes.md -->

## Schema format

The schema is a JSON object where keys are required field names and values are type strings: `string`, `number`, `boolean`, `array`, `object`.

```json
{"name":"string","age":"number","active":"boolean"}
```

Extra keys in the input records are ignored - only the schema keys are checked. You can provide the schema as an inline JSON string or as a path to a `.json` file.

## How it works

### Valid records pass through silently

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob","age":25}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}'
{"name":"Alice","age":30}
{"name":"Bob","age":25}
```

No output on stderr. Exit 0. Only clean data reaches stdout.

### Invalid records are dropped with a warning

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob"}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}'
warning: validate: line 2: age is missing
{"name":"Alice","age":30}
```

Stderr shows: `warning: validate: line 2: age is missing`

Bob's record was missing `age`, so it was dropped. Alice's record passed through. The pipeline continues with only clean data.

### --strict halts on first failure

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob"}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}' --strict
error: validate: line 2: age is missing
{"name":"Alice","age":30}
$ echo $?
1
```

Stderr shows: `error: validate: line 2: age is missing`

In strict mode, the first invalid record stops the pipeline with exit 1. Use this when partial results are worse than no results.

### --fix sends invalid records to an LLM for repair

```bash
cat messy-output.jsonl | \
  vrk validate --schema '{"sentiment":"string","confidence":"number"}' --fix
```

Invalid records are sent to `vrk prompt` with instructions to fix them to match the schema. The repaired records are emitted if they now pass validation. Requires `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`.

### --json appends metadata

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob"}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}' --json
warning: validate: line 2: age is missing
{"name":"Alice","age":30}
{"_vrk":"validate","total":2,"passed":1,"failed":1}
```

The metadata trailer tells you how many records passed and failed.

## Pipeline integration

### Validate LLM output before storing

```bash
# Ask an LLM a question, validate the response shape, then store it
echo "What are the top 3 risks?" | \
  vrk prompt --schema '{"risks":"array","summary":"string"}' | \
  vrk validate --schema '{"risks":"array","summary":"string"}' --strict | \
  vrk kv set --ns analysis latest-risks
```

### Validate then assert specific values

```bash
# Validate schema shape, then check that confidence is high enough
echo "$LLM_RESPONSE" | \
  vrk validate --schema '{"answer":"string","confidence":"number"}' --strict | \
  vrk assert '.confidence >= 0.8'
```

### Process a batch and log failures

```bash
# Validate each record, emit structured logs for monitoring
cat batch-output.jsonl | \
  vrk validate --schema '{"result":"string","score":"number"}' --json | \
  vrk emit --tag validation --parse-level
```

## When it fails

Schema missing:

```bash
$ cat data.jsonl | vrk validate
usage error: validate: --schema is required
$ echo $?
2
```

A `--schema` value that is not a JSON object is treated as a schema file path:

```bash
$ cat data.jsonl | vrk validate --schema 'not json'
usage error: validate: cannot read schema file: stat not json: no such file or directory
$ echo $?
2
```

Strict mode on invalid record:

```bash
$ printf '{"name":"Alice"}\n' | vrk validate --schema '{"name":"string","age":"number"}' --strict
error: validate: line 1: age is missing
$ echo $?
1
```

A record must be a single JSON object. `null`, arrays, scalars and lines with a second value after the first are all invalid:

```bash
$ echo 'null' | vrk validate --schema '{"a":"number"}' --strict
error: validate: line 1: line is not a JSON object
$ echo $?
1
```
