---
title: "vrk mask"
description: "Secret redactor - entropy + pattern-based, streaming"
tool: mask
group: pipeline
mcp_callable: true
noindex: false
---

## The problem

You are logging pipeline output, writing debug traces to a file, or sending data to an LLM. Somewhere in that stream is a bearer token, an API key, a database password. It ended up in a log line because an HTTP library echoes request headers, or a shell one-liner captured both stdout and stderr, or a config dump included the full env. You need to redact secrets before the data leaves your machine - before it hits a remote log aggregator, before it appears in an LLM's context window, before it ends up in a ticket.

`vrk mask` sits in the pipe and replaces secrets with `[REDACTED]`. It runs two passes: first pattern matching (bearer tokens, passwords, api_keys, secrets, tokens), then Shannon entropy scanning to catch anything the patterns missed.

## The fix

```bash
cat application.log | vrk mask
```

`vrk mask` is stdin-only. It is a stream filter that operates line-by-line on an unbounded input. There is no positional argument form because the input is a potentially infinite stream.

## Walkthrough

**Basic redaction - known patterns**

The built-in patterns cover the most common credential shapes. Run any log or config dump through mask and the obvious secrets disappear.

```bash
cat server.log | vrk mask
```

The built-in patterns match: `Bearer <token>`, `password=<value>`, `secret=<value>`, `api_key=<value>`, `token=<value>`. Matching is case-insensitive. The key prefix is preserved; only the value is replaced.

**Entropy-based redaction**

High-entropy strings that do not match any pattern - random UUIDs used as secrets, base64-encoded keys, opaque session tokens - are caught by the entropy scanner. Any whitespace-delimited token of 8 or more characters whose Shannon entropy meets the threshold is redacted.

The default threshold is 4.0 bits per character. Lower it to catch more; raise it to be more conservative.

```bash
cat config.yaml | vrk mask --entropy 3.5
```

**Adding custom patterns**

`--pattern` accepts a Go regular expression. The entire match is replaced with `[REDACTED]`. Repeat the flag for multiple patterns.

```bash
cat output.txt | vrk mask --pattern 'sk-[a-zA-Z0-9]{32,}'
cat output.txt | vrk mask --pattern 'ghp_[A-Za-z0-9]+' --pattern 'xoxb-[0-9]+-[0-9]+-[A-Za-z0-9]+'
```

**Getting redaction statistics with `--json`**

The `--json` flag appends a metadata record after all output lines:

```bash
cat application.log | vrk mask --json
```

The final line looks like:

```json
{"_vrk":"mask","lines":842,"redacted":3,"patterns_matched":["bearer","entropy"]}
```

`patterns_matched` lists every pattern name that fired at least once across the entire run, in declaration order: builtins first, then `"entropy"`, then custom patterns in the order they were passed.

**Chaining with emit for structured redacted logs**

```bash
cat application.log | vrk mask | vrk emit --parse-level --tag app
```

Redact first, then wrap each line as a structured JSONL log record. The `--parse-level` flag on `emit` picks up ERROR/WARN/INFO/DEBUG prefixes automatically.

## Pipeline example

```bash
cat application.log | vrk mask | vrk emit --level info --tag pipeline
```

Redact secrets from a log file, then wrap each line as a structured log record tagged `pipeline`. The result is clean, structured, secret-free JSONL ready for a log aggregator or further pipeline stages.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--pattern` | | []string | none | Additional Go regex pattern to match and redact (repeatable) |
| `--entropy` | | float64 | 4.0 | Shannon entropy threshold; lower catches more, higher is more conservative |
| `--json` | `-j` | bool | false | Append a `{"_vrk":"mask","lines":N,"redacted":N,"patterns_matched":[...]}` record after all output |
| `--quiet` | `-q` | bool | false | Suppress stderr output; exit codes are unaffected |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success - all lines processed and written to stdout |
| 1 | Runtime error - stdin scanner error or write failure |
| 2 | Usage error - interactive TTY with no piped input, or invalid regex in `--pattern` |
