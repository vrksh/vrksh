## JSON mode

`vrk assert` checks JSON input against gojq expressions and exits 1 if a condition is false. If every condition passes, the input goes to stdout unchanged.

### Basic conditions

```bash
$ echo '{"status":"ok","count":42}' | vrk assert '.status == "ok"'
{"status":"ok","count":42}
$ echo $?
0
```

```bash
$ echo '{"status":"error"}' | vrk assert '.status == "ok"'
assert: assertion failed: .status == "ok"
$ echo $?
1
```

The message goes to stderr. Stdout is empty.

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
assert: assertion failed: Confidence too low for production (.score >= 0.8)
$ echo $?
1
```

The error shows your message first, then the failed condition in parentheses.

## Text mode

`vrk assert` also checks plain text for a substring or a regex pattern.

### --contains (substring check)

```bash
$ echo 'deployment successful' | vrk assert --contains 'successful'
deployment successful
$ echo $?
0

$ echo 'deployment failed' | vrk assert --contains 'successful'
assert: assertion failed: input does not contain "successful"
$ echo $?
1
```

### --matches (regex check)

```bash
$ echo 'v2.3.1' | vrk assert --matches '^v[0-9]+\.[0-9]+\.[0-9]+$'
v2.3.1
$ echo $?
0

$ echo 'not-a-version' | vrk assert --matches '^v[0-9]+\.[0-9]+\.[0-9]+$'
assert: assertion failed: input does not match "^v[0-9]+\\.[0-9]+\\.[0-9]+$"
$ echo $?
1
```

The pattern is printed as a Go-quoted string, so each backslash is printed twice.

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
usage error: assert: a condition expression is required
$ echo $?
2
```

If the input is empty and the condition is a JSON expression, assert exits 1. A check with nothing to test does not pass:

```bash
$ printf '' | vrk assert '.ok == true'
error: assert: no JSON records on stdin
$ echo $?
1
```

Each line must hold exactly one JSON value. A second value on the same line is a parse error, not a pass on the first:

```bash
$ echo '{"a":1} {"a":2}' | vrk assert '.a == 1'
assert: input is not valid JSON
$ echo $?
1
```

If a condition runs longer than 5 seconds, assert stops it and the assertion fails with `condition timed out after 5s`.
