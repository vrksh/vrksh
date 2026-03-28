---
title: "vrk tok"
description: "Token counter - cl100k_base, --budget guard, --json."
tool: tok
group: core
mcp_callable: true
noindex: false
---

## The problem

LLM APIs have context windows. Send more tokens than the window allows and the
API either silently truncates your input or returns an error. You don't find out
until the response is wrong or missing. There is no built-in way to check before
you send.

## The fix

```bash
$ echo "Hello world" | vrk tok
2
```

One number. That is the token count. You can check it before making an API call
that costs money and takes seconds to fail.

A larger input:

```bash
$ cat my_system_prompt.txt | vrk tok
12847
```

If that number is higher than your model's context window, you know before you
send -- not after.

## Guarding a budget

The `--budget` flag turns tok into a gate. If the input fits within the budget,
it prints the count and exits 0. If it exceeds the budget, it prints an error
to stderr and exits 1.

```bash
cat prompt.txt | vrk tok --budget 4000
```

When the input exceeds the budget:

```
error: tok: 12847 tokens exceeds budget of 4000
```

Exit code 1. The pipeline stops here.

This is the primary pattern -- check the count, then send:

```bash
cat prompt.txt | vrk tok --budget 4000 && cat prompt.txt | vrk prompt
```

The `&&` means: only run the right side if the left side exits 0. If tok
exits 1 (over budget), prompt never runs. No API call, no wasted money.

## JSON output

```bash
$ echo "Hello world" | vrk tok --json
{"tokens":2,"model":"cl100k_base"}
```

When `--budget` is set and the count exceeds it, the error goes to stdout
as JSON instead of stderr:

```bash
$ echo "Hello world this is a longer sentence" | vrk tok --json --budget 3
{"code":1,"error":"tok: 7 tokens exceeds budget of 3"}
```

Use `--json` when a script needs to parse the count or log it. Use plain
output when a human is reading.

## Real pipeline

```bash
CONTENT=$(vrk grab https://example.com/article)
echo "$CONTENT" | vrk tok --budget 8000 \
  && echo "$CONTENT" | vrk prompt --system "Summarize in 3 bullet points."
```

We capture the content in a variable first to avoid fetching the URL twice.
Fetching twice is wasteful and can return different content if the page
changes between requests.

## Accuracy note

cl100k_base is exact for GPT-4 and GPT-4o. For Claude models, it is
roughly 95% accurate (Claude's tokenizer is not public). Set `--budget`
at 90% of the actual context limit to absorb the margin.

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--json` | `-j` | bool | `false` | Emit JSON with token count and metadata |
| `--budget` | | int | `0` | Exit 1 if token count exceeds N |
| `--model` | `-m` | string | `cl100k_base` | Tokenizer model (currently cl100k_base only) |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success, within budget |
| 1 | Over budget or I/O error |
| 2 | No input, unknown flag |
