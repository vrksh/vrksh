## What gets redacted

Built-in patterns detect:

- **Bearer tokens**: `Bearer eyJ...`, `Bearer ghp_...`
- **Passwords**: `password=...`, `passwd:...` and similar patterns
- **Known key prefixes**: AWS (`AKIA`, `ASIA`), OpenAI and Anthropic (`sk-`), Stripe (`sk_live_`, `rk_test_`), GitHub (`ghp_`, `github_pat_`), Slack (`xoxb-`), Google (`AIza`), Hugging Face (`hf_`), npm (`npm_`), SendGrid (`SG.`), and JWTs (`eyJ...`). These match anywhere in a line, including inside URLs and paths
- **High-entropy strings**: any token with Shannon entropy at or above the threshold (default 4.0) that looks like a secret - API keys, connection strings, hex tokens

```bash
$ printf 'API key: sk-proj-abc123def456ghi789jkl012mno345pqr678stu901vwx234yz\nBearer ghp_FAKE0000example0000notreal0000000000\nPassword: s3cr3t-p4ssw0rd-v3ry-l0ng\nNormal text stays untouched\n' | vrk mask
API key: [REDACTED]
Bearer [REDACTED]
Password: [REDACTED]
Normal text stays untouched
```

### What entropy scanning leaves alone

An LLM reading a log needs its URLs, paths, and IDs. These are never redacted as a whole by entropy:

- URLs with no userinfo (`user@` or `user:pass@`). A URL with userinfo is entropy-checked as one token.
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
Bearer [REDACTED] [REDACTED]
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
usage error: mask: invalid pattern "[invalid": error parsing regexp: missing closing ]: `[invalid`
$ echo $?
2
```
