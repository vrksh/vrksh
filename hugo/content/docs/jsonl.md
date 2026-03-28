---
title: "vrk jsonl"
description: "JSON array ↔ JSONL converter - --collect, --json."
tool: jsonl
group: utilities
mcp_callable: true
noindex: false
---

## The problem

APIs return JSON arrays. Unix tools - `grep`, `awk`, `sort`, `uniq` - work on lines. Every time you want to process individual records from an API response, you reach for `jq` to explode the array, and every time you want to send records back to an API, you reach for `jq` again to wrap them. The round-trip is awkward, and large arrays read into `jq` all at once can be slow or blow memory.

## The fix

Split a JSON array into one record per line:

```bash
echo '[{"id":1,"name":"Alice"},{"id":2,"name":"Bob"}]' | vrk jsonl
# {"id":1,"name":"Alice"}
# {"id":2,"name":"Bob"}
```

Collect JSONL lines back into an array:

```bash
printf '{"id":1}\n{"id":2}\n' | vrk jsonl --collect
# [{"id":1},{"id":2}]
```

## Walkthrough

**Split mode (default)** - reads a JSON array from stdin using a streaming decoder. Arrays larger than memory are handled safely because elements are decoded one at a time.

```bash
vrk grab https://api.example.com/users | vrk jsonl
```

If the input is not a JSON array, jsonl exits 1 with a clear error suggesting `--collect` if that's what you meant:

```bash
echo '{"a":1}' | vrk jsonl
# error: jsonl: input is not a JSON array; for JSONL → array use --collect
```

**Collect mode** - reads JSONL lines and emits a single JSON array. Empty stdin in collect mode outputs `[]`. Blank lines are skipped. A line that is not valid JSON exits 1 with the line number:

```bash
printf '{"a":1}\n\nnot-json\n' | vrk jsonl --collect
# error: jsonl: invalid JSON on line 3
```

**Metadata trailer** - `--json` appends a count record after all split records. Only available in split mode:

```bash
echo '[{"a":1},{"b":2}]' | vrk jsonl --json
# {"a":1}
# {"b":2}
# {"_vrk":"jsonl","count":2}
```

**Large numbers** - jsonl uses `UseNumber()` internally, so large integers (IDs, Unix timestamps) are preserved exactly. You will not lose precision on values above 2^53.

**Positional argument** - input can be passed as an argument instead of piped:

```bash
vrk jsonl '[{"x":1},{"x":2}]'
```

## Pipeline example

Fetch a user list, split it into records, and redact sensitive fields:

```bash
vrk grab https://api.example.com/users | vrk jsonl | vrk mask
```

Collect output from multiple tools into a single array to post back to an API:

```bash
{
  echo '{"status":"ok","ts":1740009600}'
  echo '{"status":"warn","ts":1740010000}'
} | vrk jsonl --collect | curl -sf -X POST -d @- https://api.example.com/batch
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--collect` | `-c` | bool | false | Collect JSONL lines into a JSON array. Empty input → `[]`. |
| `--json` | `-j` | bool | false | Append `{"_vrk":"jsonl","count":N}` after all records. Split mode only. |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success, including empty input |
| 1 | Runtime error - invalid JSON, I/O error |
| 2 | Usage error - interactive TTY with no input, unknown flag |
