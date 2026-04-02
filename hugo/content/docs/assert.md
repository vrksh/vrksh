---
title: "vrk assert"
description: "Assert conditions mid-pipeline. Exit 1 if a check fails. Catches bad data before it reaches the next stage."
meta_lead: "vrk assert evaluates conditions mid-pipeline and stops the flow if a check fails."
og_title: "vrk assert - pipeline condition checks that halt on failure"
tool: assert
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk assert evaluates conditions mid-pipeline and stops the flow if a check fails.

## The problem

Schema validation catches missing fields and wrong types. It does not catch wrong values. An LLM returns {"status":"error","message":"rate limited"} and the schema passes because status is a string. The garbage ends up in your database. You find out when a user reports it.

## The solution

`vrk assert` evaluates conditions on pipeline data and stops the flow if a check fails. In JSON mode it runs gojq expressions. In text mode it checks substrings or regex patterns. Data passes through on success, so you can insert it between any two pipeline stages as a gate without breaking the data flow.

## Before and after

**Before**

```bash
echo "$RESPONSE" | jq -e '.status == "ok"' > /dev/null
if [ $? -ne 0 ]; then exit 1; fi
echo "$RESPONSE" | next-step
# data doesn't pass through, must re-read
```

**After**

```bash
echo $RESPONSE | vrk assert '.status == "ok"' | next-step
```

## Example

```bash
echo '{"status":"ok","count":42}' | vrk assert '.count > 0'
```

## Exit codes

| Code | Meaning                                                             |
|------|---------------------------------------------------------------------|
| 0    | All conditions passed; input passed through to stdout               |
| 1    | Assertion failed, or runtime error                                  |
| 2    | No condition specified, mixed modes, invalid regex, interactive TTY |

## Flags

| Flag         | Short | Type   | Description                                                                                                               |
|--------------|-------|--------|---------------------------------------------------------------------------------------------------------------------------|
| `--contains` |       | string | Assert stdin contains this literal substring                                                                              |
| `--matches`  |       | string | Assert stdin matches this regular expression. Complex patterns with many alternations reduce throughput on large streams. |
| `--message`  | -m    | string | Custom message on failure                                                                                                 |
| `--json`     | -j    | bool   | Emit errors as JSON to stdout                                                                                             |
| `--quiet`    | -q    | bool   | Suppress stderr output on failure                                                                                         |


<!-- notes - edit in notes/assert.notes.md -->

## JSON mode

Evaluates gojq expressions against JSON input. Data passes through on success.

### Basic conditions

```bash
$ echo '{"status":"ok","count":42}' | vrk assert '.status == "ok"'
{"status":"ok","count":42}
$ echo $?
0
```

```bash
$ echo '{"status":"error"}' | vrk assert '.status == "ok"'
$ echo $?
1
```

Stderr: `assert: assertion failed: .status == "ok"`

### Common gojq expressions

```bash
# Check a field exists and has a value
echo "$JSON" | vrk assert '.items | length > 0'

# Check a numeric threshold
echo "$JSON" | vrk assert '.confidence >= 0.8'

# Check for null/absence
echo "$JSON" | vrk assert '.error == null'

# Multiple conditions (all must pass)
echo "$JSON" | vrk assert '.status == "ok"' '.items | length > 0'
```

### Custom failure message

```bash
$ echo '{"score":0.3}' | vrk assert '.score >= 0.8' -m 'Confidence too low for production'
$ echo $?
1
```

Stderr: `assert: Confidence too low for production`

## Text mode

Checks plain text input for substrings or regex patterns.

### --contains (substring check)

```bash
$ echo 'deployment successful' | vrk assert --contains 'successful'
deployment successful
$ echo $?
0

$ echo 'deployment failed' | vrk assert --contains 'successful'
$ echo $?
1
```

Stderr: `assert: assertion failed: input does not contain "successful"`

### --matches (regex check)

```bash
$ echo 'v2.3.1' | vrk assert --matches '^v[0-9]+\.[0-9]+\.[0-9]+$'
v2.3.1
$ echo $?
0

$ echo 'not-a-version' | vrk assert --matches '^v[0-9]+\.[0-9]+\.[0-9]+$'
$ echo $?
1
```

Stderr: `assert: assertion failed: input does not match "^v[0-9]+\.[0-9]+\.[0-9]+$"`

## Pipeline integration

### Guard between prompt and database

```bash
# Only store the result if it looks correct
echo "$INPUT" | \
  vrk prompt --schema '{"answer":"string","confidence":"number"}' | \
  vrk assert '.confidence >= 0.8' | \
  vrk kv set --ns results latest
```

If confidence is below 0.8, assert exits 1 and the pipeline stops before writing to kv.

### Validate API responses in a script

```bash
# Check that an API returned the expected shape
RESPONSE=$(curl -s https://api.example.com/health)
echo "$RESPONSE" | vrk assert '.status == "healthy"' '.uptime > 0' --quiet
if [ $? -ne 0 ]; then
  echo "Health check failed" | vrk emit --level error --tag monitoring
  exit 1
fi
```

### Check LLM output before piping to emit

```bash
# Ensure the LLM returned real content, not an error
echo "$DOCUMENT" | \
  vrk prompt --system 'Summarize this' | \
  vrk assert --contains 'Summary' -m 'LLM did not return a summary' | \
  vrk emit --tag summaries --level info
```

## When it fails

Assertion failed (JSON mode):

```bash
$ echo '{"status":"error"}' | vrk assert '.status == "ok"'
assert: assertion failed: .status == "ok"
$ echo $?
1
```

Assertion failed (text mode):

```bash
$ echo 'hello' | vrk assert --contains 'goodbye'
assert: assertion failed: input does not contain "goodbye"
$ echo $?
1
```

No condition specified:

```bash
$ echo '{}' | vrk assert
usage error: assert: no condition specified
$ echo $?
2
```
