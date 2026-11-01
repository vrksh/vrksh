## Schema format

`vrk validate` checks each JSONL record (one JSON object per line) against a schema and drops the records that do not match.

The schema is a JSON object. Each key is a required field name, and each value is a type string: `string`, `number`, `boolean`, `array`, `object`.

```json
{"name":"string","age":"number","active":"boolean"}
```

validate ignores extra keys in the input records and checks only the schema keys. Give the schema as an inline JSON string or as a path to a `.json` file.

## How it works

### Valid records pass through silently

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob","age":25}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}'
{"name":"Alice","age":30}
{"name":"Bob","age":25}
```

validate writes nothing to stderr and exits 0. Only valid records reach stdout.

### Invalid records are dropped with a warning

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob"}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}'
warning: validate: line 2: age is missing
{"name":"Alice","age":30}
```

Stderr shows `warning: validate: line 2: age is missing`.

Bob's record has no `age`, so validate drops it. Alice's record passes through. The pipeline continues with only valid records.

### --strict halts on first failure

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob"}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}' --strict
error: validate: line 2: age is missing
{"name":"Alice","age":30}
$ echo $?
1
```

Stderr shows `error: validate: line 2: age is missing`.

In strict mode, the first invalid record stops the pipeline with exit 1. Use `--strict` when a partial result is worse than no result.

### --fix sends invalid records to an LLM for repair

```bash
cat messy-output.jsonl | \
  vrk validate --schema '{"sentiment":"string","confidence":"number"}' --fix
```

validate sends each invalid record to `vrk prompt` with an instruction to fix it to match the schema. If the repaired record passes validation, validate prints it. This needs `ANTHROPIC_API_KEY` or `OPENAI_API_KEY`.

### --json appends metadata

```bash
$ printf '{"name":"Alice","age":30}\n{"name":"Bob"}\n' | \
    vrk validate --schema '{"name":"string","age":"number"}' --json
warning: validate: line 2: age is missing
{"name":"Alice","age":30}
{"_vrk":"validate","total":2,"passed":1,"failed":1}
```

The metadata trailer gives the number of records that passed and failed.

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

If you leave out `--schema`, validate exits 2:

```bash
$ cat data.jsonl | vrk validate
usage error: validate: --schema is required
$ echo $?
2
```

If the `--schema` value is not a JSON object, validate treats it as a schema file path:

```bash
$ cat data.jsonl | vrk validate --schema 'not json'
usage error: validate: cannot read schema file: stat not json: no such file or directory
$ echo $?
2
```

If a record is invalid and you set `--strict`, validate exits 1:

```bash
$ printf '{"name":"Alice"}\n' | vrk validate --schema '{"name":"string","age":"number"}' --strict
error: validate: line 1: age is missing
$ echo $?
1
```

A record must be one JSON object. `null`, arrays, scalars and lines with a second value after the first are all invalid:

```bash
$ echo 'null' | vrk validate --schema '{"a":"number"}' --strict
error: validate: line 1: line is not a JSON object
$ echo $?
1
```
