---
title: "vrk mask"
description: "Redact secrets from pipeline output before logging. Detects API keys, tokens, and high-entropy strings automatically."
og_title: "vrk mask - automatic secret redaction for pipeline output"
tool: mask
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

You pipe log output to an LLM for analysis and accidentally send API keys, Bearer tokens, and passwords in the prompt. The LLM provider now has your production credentials. You don't find out until a security audit three months later flags the API call in the provider's logs.

`vrk mask` redacts secrets from text before it reaches an LLM or a log file. It catches Bearer tokens, passwords, API keys, and other credentials using built-in pattern matching. It also flags high-entropy strings that look like secrets but don't match a known pattern. Add `--pattern` for custom regexes to catch internal identifiers. Streams input line-by-line, so it handles any file size safely.

## The problem

You're debugging a production issue. You pipe the last 500 lines of your application log to an LLM and ask it what went wrong. The logs contain database connection strings with passwords, API keys in HTTP headers, and Bearer tokens. All of it just went to a third-party API. Your security team asks why production credentials appeared in an external service's request logs.

## Before and after

**Before**

```bash
cat output.log | sed 's/Bearer [^ ]*/Bearer ***/g' | \
  sed 's/password=[^ ]*/password=***/g'
# misses API keys, high-entropy secrets, custom patterns
```

**After**

```bash
cat output.log | vrk mask
```

## Example

```bash
cat deploy.log | vrk mask | vrk prompt --system 'What errors occurred?'
```

## Exit codes

| Code | Meaning                                            |
|------|----------------------------------------------------|
| 0    | All lines processed                                |
| 1    | Stdin scanner error or write failure               |
| 2    | Interactive TTY with no piped input, invalid regex |

## Flags

| Flag        | Short | Type     | Description                                          |
|-------------|-------|----------|------------------------------------------------------|
| `--pattern` |       | []string | Additional Go regex to match and redact (repeatable) |
| `--entropy` |       | float64  | Shannon entropy threshold; lower catches more        |
| `--json`    | -j    | bool     | Append metadata trailer after output                 |
| `--quiet`   | -q    | bool     | Suppress stderr output                               |


<!-- notes - edit in notes/mask.notes.md -->

## What gets redacted

Built-in patterns detect:

- **Bearer tokens**: `Bearer eyJ...`, `Bearer ghp_...`
- **Passwords**: `password=...`, `passwd:...` and similar patterns
- **High-entropy strings**: any token with Shannon entropy above the threshold (default 4.0) that looks like a secret - API keys, connection strings, hex tokens

```bash
$ printf 'API key: sk-proj-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz\nBearer ghp_xK9mN2pL4qR7sT0uW3yZ5bD8fH1jM6nP9rV2\nPassword: s3cr3t-p4ssw0rd-v3ry-l0ng\nNormal text stays untouched\n' | vrk mask
API key: [REDACTED]
Bearer [REDACTED]
Password: [REDACTED]
Normal text stays untouched
```

### The --json flag

Appends a metadata trailer showing what was redacted:

```bash
$ printf 'Bearer ghp_xK9mN2pL4qR7sT0uW3yZ5bD8fH1jM6nP9rV2\nNormal line\n' | vrk mask --json
Bearer [REDACTED] [REDACTED]
Normal line
{"_vrk":"mask","lines":2,"redacted":1,"patterns_matched":["bearer","entropy"]}
```

### Custom patterns (--pattern)

Add your own regex patterns for internal identifiers, ticket numbers, or employee IDs:

```bash
$ printf 'Error in ticket PROJ-1234 for employee EMP-56789\n' | vrk mask --pattern 'PROJ-\d+' --pattern 'EMP-\d+'
Error in ticket [REDACTED] for employee [REDACTED]
```

The `--pattern` flag is repeatable. Custom patterns are applied alongside the built-in ones.

### Adjusting entropy sensitivity (--entropy)

Lower the threshold to catch more potential secrets (more aggressive, more false positives). Raise it to only catch obvious high-entropy strings:

```bash
# More aggressive - catches shorter secrets
cat logs.txt | vrk mask --entropy 3.5

# Less aggressive - only obvious high-entropy strings
cat logs.txt | vrk mask --entropy 4.5
```

## The pipeline position rule

Always mask **before** sending to an LLM, never after. The data goes to the API in the request, not in the response. Masking the output is too late.

```bash
# Correct: mask before prompt
cat deploy.log | vrk mask | vrk prompt --system 'What went wrong?'

# Wrong: mask after prompt (secrets already sent to API)
cat deploy.log | vrk prompt --system 'What went wrong?' | vrk mask
```

## Pipeline integration

### Mask logs before LLM analysis

```bash
# Redact credentials, check token budget, then analyze
cat production.log | vrk mask | vrk tok --check 12000 | \
  vrk prompt --system 'Identify the root cause of errors in this log'
```

### Mask and emit structured logs

```bash
# Redact secrets from application output, then wrap as structured JSONL
./run-pipeline.sh 2>&1 | vrk mask | vrk emit --tag pipeline --parse-level
```

### Mask before chunking a large file

```bash
# For large log files: mask, chunk, then process each piece
cat large-debug.log | vrk mask | vrk chunk --size 4000 | \
  while IFS= read -r record; do
    echo "$record" | jq -r '.text' | \
      vrk prompt --system 'Summarize errors in this log section'
  done
```

## When it fails

Interactive terminal with no input:

```bash
$ vrk mask
usage error: mask: no input: pipe text to stdin
$ echo $?
2
```

Invalid regex in --pattern:

```bash
$ echo 'test' | vrk mask --pattern '[invalid'
usage error: mask: invalid regex: error parsing regexp: missing closing ]
$ echo $?
2
```
