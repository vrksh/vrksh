---
title: "vrk mask"
description: "vrk mask redacts secrets such as API keys, Bearer tokens, and high-entropy strings from text. It reads line by line."
meta_lead: "vrk mask redacts secrets from text using pattern matching and entropy analysis, streaming line by line."
og_title: "vrk mask - automatic secret redaction for pipeline output"
tool: mask
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk mask redacts secrets from text using pattern matching and entropy analysis, streaming line by line.

## The problem

You pipe 500 lines of application logs to an LLM to diagnose an outage. The logs contain database connection strings, API keys in HTTP headers, and Bearer tokens. All of it goes to a third-party API. Three months later a security audit finds production credentials in the provider's request logs.

## The solution

`vrk mask` redacts secrets from text using built-in patterns for Bearer tokens, passwords, and known API key prefixes (AWS, OpenAI, Anthropic, Stripe, GitHub, Slack, Google, and more). It also flags high-entropy strings that look like secrets, while leaving URLs, paths, UUIDs, digests, and image refs readable for the LLM. It reads line by line, so it handles any file size. Use `--pattern` to add custom regexes.

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

| Flag        | Short | Type     | Description                                                                                                                       |
|-------------|-------|----------|-----------------------------------------------------------------------------------------------------------------------------------|
| `--pattern` |       | []string | Additional Go regex to match and redact (repeatable). Complex patterns with many alternations reduce throughput on large streams. |
| `--entropy` |       | float64  | Shannon entropy threshold; lower catches more                                                                                     |
| `--json`    | -j    | bool     | Append metadata trailer after output                                                                                              |
| `--quiet`   | -q    | bool     | Suppress stderr output                                                                                                            |


<!-- notes - edit in notes/mask.notes.md -->

## What gets redacted

`vrk mask` replaces secrets such as API keys, bearer tokens, and passwords with `[REDACTED]` before you send text to an LLM.

Built-in patterns detect:

- **Bearer tokens**: `Bearer eyJ...`, `Bearer ghp_...`
- **Keyword secrets**: any key ending in `password`, `secret`, `token`, `api_key` and similar words (`DB_PASS`, `X-API-Key`, `access_token`), written as `key=v`, `key: v`, `"key": "v"`, `<key>v</key>` or `--key v`. Quoted values are redacted up to the closing quote and the quotes stay, so JSON stays valid
- **Credentials in transit**: `Authorization:` headers with any scheme (`Basic`, `token`, ...), the password in `scheme://user:pass@host`, and `curl -u user:pass`
- **Known key prefixes**: AWS (`AKIA`, `ASIA`), OpenAI and Anthropic (`sk-`), Stripe (`sk_live_`, `rk_test_`), GitHub (`ghp_`, `github_pat_`), Slack (`xoxb-`), Google (`AIza`), Hugging Face (`hf_`), npm (`npm_`), SendGrid (`SG.`), and JWTs (`eyJ...`). These match anywhere in a line, including inside URLs and paths
- **High-entropy strings**: any token with Shannon entropy at or above the threshold (default 4.0) that looks like a secret - API keys, connection strings. Random hex can't go above 4.0, so runs of 32+ hex characters use 3.0 instead

```bash
$ printf 'API key: sk-proj-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz\nBearer ghp_FAKE0000example0000notreal0000000000\nPassword: s3cr3t-p4ssw0rd-v3ry-l0ng\nNormal text stays untouched\n' | vrk mask
API key: [REDACTED]
Bearer [REDACTED]
Password: [REDACTED]
Normal text stays untouched
```

JSON records keep their shape, and keys like `tokens` or `max_tokens` are left alone:

```bash
$ printf '%s\n' '{"id": 0, "password": "pw0", "token": "f9c0b7fb5a83541b7e4924b2d17670dbdf99c483"}' '{"tokens":42,"model":"o200k_base"}' 'Authorization: Basic YWRtaW46aHVudGVyMg==' 'amqp://guest:guest@mq' | vrk mask
{"id": 0, "password": "[REDACTED]", "token": "[REDACTED]"}
{"tokens":42,"model":"o200k_base"}
Authorization: Basic [REDACTED]
amqp://guest:[REDACTED]@mq
```

### What entropy scanning leaves alone

An LLM reading a log needs its URLs, paths, and IDs. These are never redacted as a whole by entropy:

- URLs with no userinfo (`user@` or `user:pass@`). In a URL with `user:pass@` the password is redacted by pattern and the rest of the URL is not checked.
- A 40-character hex git SHA on a line that mentions git (`commit`, `merge`, `sha`, `rev`, ...). Other bare runs of 32+ hex characters, such as `md5sum` output, are redacted; write a digest as `sha256:<hex>` to keep it
- Paths that start with `/`, `./`, `../`, or `~/`
- Canonical UUIDs (`7cbc81bf-c747-466a-a777-f3f88dcdfe38`)
- Digests with an algorithm prefix (`sha256:...`, `sha1:...`, `md5:...`)
- Container image refs (`registry.example.com/api-gateway:2.31.4`)
- `name=value` tokens whose value is one of the above (`request_id=<uuid>`)

Each `/` segment of a URL, path, or image ref is still checked on its own, so a random segment such as a webhook secret becomes `[REDACTED]` while the rest stays. Query and fragment values are checked only when the parameter name looks secret (`key`, `api_key`, `apikey`, `token`, `access_token`, `secret`, `code`, `sig`, `signature`, `password`, `pass`, `auth`, or a name ending in one of them, like `client_secret`). The built-in patterns above always apply to the whole line.

Punctuation around a secret is kept: a trailing `,.;:)]}'"` and one leading `('"[{` stay in place.

```bash
$ printf '2026-10-15T05:31:15Z deploy api-gateway v2.31.4\npulling image registry.example.com/api-gateway:2.31.4\nGET https://payments.internal/v1/charges failed: 503 after 3 retries\nretrying with STRIPE_KEY=sk_test_FAKE0000example0000notreal, request_id=7cbc81bf-c747-466a-a777-f3f88dcdfe38\nconfig loaded from /etc/payments/config.yaml\n' | vrk mask
2026-10-15T05:31:15Z deploy api-gateway v2.31.4
pulling image registry.example.com/api-gateway:2.31.4
GET https://payments.internal/v1/charges failed: 503 after 3 retries
retrying with STRIPE_KEY=[REDACTED], request_id=7cbc81bf-c747-466a-a777-f3f88dcdfe38
config loaded from /etc/payments/config.yaml
```

### The --json flag

Appends a metadata trailer showing what was redacted:

```bash
$ printf 'Bearer ghp_FAKE0000example0000notreal0000000000\nNormal line\n' | vrk mask --json
Bearer [REDACTED]
Normal line
{"_vrk":"mask","lines":2,"redacted":1,"patterns_matched":["bearer"]}
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

Mask **before** you send text to an LLM, never after. The secret leaves your machine in the request, not in the response. Masking the output is too late.

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

<!-- doccheck: skip (only fails on an interactive terminal, doccheck stdin is not a TTY) -->
```bash
$ vrk mask
usage error: mask: no input: pipe text to stdin
$ echo $?
2
```

Invalid regex in --pattern:

```bash
$ echo 'test' | vrk mask --pattern '[invalid'
usage error: mask: invalid pattern "[invalid": error parsing regexp: missing closing ]: `[invalid`
$ echo $?
2
```
