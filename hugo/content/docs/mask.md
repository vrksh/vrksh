---
title: "vrk mask"
description: "Secret redactor - entropy + pattern-based, streaming"
tool: mask
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

Your pipeline processes text that might contain secrets -- API keys,
bearer tokens, passwords, high-entropy strings. Passing these to an LLM
leaks them. Writing to a log file exposes them. There is no Unix tool that
strips secrets from a stream before it goes downstream.

## The fix

```bash
$ echo "Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.test.sig" | vrk mask
Authorization: Bearer [REDACTED]
```

Secrets are replaced with `[REDACTED]`. The rest of the text passes
through unchanged.

## What gets masked by default

The built-in patterns cover common credential shapes. Each matches
case-insensitively; the key prefix is preserved, only the value is replaced.

```bash
$ echo "password=mysecretpass123" | vrk mask
password=[REDACTED]

$ echo "My key is sk-proj-abc123def456ghi789jkl012mno345" | vrk mask
My key is [REDACTED]
```

Built-in patterns: `Bearer <token>`, `password=<value>`, `secret=<value>`,
`api_key=<value>`, `token=<value>`, and common key prefixes like `sk-`,
`pk-`.

After pattern matching, a Shannon entropy pass catches anything the
patterns missed -- random base64 strings, opaque session tokens, any
whitespace-delimited token of 8+ characters with entropy above the
threshold (default 4.0 bits/char).

## Custom patterns

```bash
$ echo "Ticket PROJ-1234 is open" | vrk mask --pattern 'PROJ-[0-9]+'
Ticket [REDACTED] is open
```

`--pattern` takes a Go regex. The entire match is replaced. Repeat the
flag for multiple patterns:

```bash
cat output.txt | vrk mask --pattern 'ghp_[A-Za-z0-9]+' --pattern 'xoxb-[0-9A-Za-z-]+'
```

## In a pipeline (always before prompt)

```bash
cat document.txt | vrk mask | vrk prompt --system "Summarize this document."
```

Mask before prompt, every time, no exceptions. Secrets that enter an
LLM's context window cannot be unlearned.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--pattern` | | []string | none | Additional Go regex to match and redact (repeatable) |
| `--entropy` | | float64 | `4.0` | Shannon entropy threshold; lower catches more |
| `--json` | `-j` | bool | `false` | Append `{"_vrk":"mask","lines":N,"redacted":N,"patterns_matched":[...]}` after output |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | All lines processed |
| 1 | Runtime error - stdin scanner error or write failure |
| 2 | Usage error - interactive TTY with no piped input, invalid regex |
