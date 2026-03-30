---
title: "vrk validate"
description: "Validate LLM JSON output against a schema. Exit 1 on mismatch. Pipeline stops before bad data propagates."
og_title: "vrk validate - schema validation for LLM JSON output"
tool: validate
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Your LLM returns JSON that looks right but isn't. The `sentiment` field is sometimes a string, sometimes a number. The `confidence` key is missing on 3% of responses. You don't catch it until a downstream aggregation script crashes at 2 AM on 847 records that slipped through without the required fields.

`vrk validate` checks every JSON record in a JSONL stream against a type schema. Valid records pass through to stdout. Invalid records are dropped with a warning to stderr. Use `--strict` to halt the pipeline on the first bad record. Use `--fix` to send invalid records to an LLM for repair.

## The problem

You ask an LLM to return {"sentiment":"string","confidence":"number"} for 500 product reviews. 487 responses are perfect. 13 return confidence as a string instead of a number. Your Python analysis script crashes on float("high"). You re-run the entire pipeline because you didn't validate between the LLM and the database.

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

| Code | Meaning                                             |
|------|-----------------------------------------------------|
| 0    | All records passed                                  |
| 1    | Record failed in --strict mode, or scanner error    |
| 2    | --schema missing, schema JSON invalid, unknown flag |

## Flags

| Flag       | Short | Type   | Description                                         |
|------------|-------|--------|-----------------------------------------------------|
| `--schema` | -s    | string | JSON schema inline or path to .json file (required) |
| `--strict` |       | bool   | Exit 1 on first invalid record                      |
| `--fix`    |       | bool   | Send invalid records to vrk prompt for repair       |
| `--json`   | -j    | bool   | Append metadata trailer after all output            |
| `--quiet`  | -q    | bool   | Suppress stderr output                              |


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
{"name":"Alice","age":30}
```

Stderr shows: `warning: validation failed: age is missing`

Bob's record was missing `age`, so it was dropped. Alice's record passed through. The pipeline continues with only clean data.

### --strict halts on first failure

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob"}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}' --strict
{"name":"Alice","age":30}
$ echo $?
1
```

Stderr shows: `warning: validation failed: age is missing`

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
{"name":"Alice","age":30}
{"_vrk":"validate","total":2,"valid":1,"invalid":1}
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

Invalid schema JSON:

```bash
$ cat data.jsonl | vrk validate --schema 'not json'
usage error: validate: invalid schema JSON
$ echo $?
2
```

Strict mode on invalid record:

```bash
$ printf '{"name":"Alice"}\n' | vrk validate --schema '{"name":"string","age":"number"}' --strict
$ echo $?
1
```
