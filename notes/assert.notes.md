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
