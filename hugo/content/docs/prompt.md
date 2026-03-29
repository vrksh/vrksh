---
title: "vrk prompt"
description: "LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain."
tool: prompt
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

Sends a prompt to an LLM and prints the response to stdout. Works with Anthropic and OpenAI APIs. You pipe content in, optionally set a system prompt, and get the response back - no curl commands, no JSON escaping, no response parsing. Temperature defaults to 0 for deterministic output.

## The problem

You need to call an LLM from a shell script. You write a curl command with JSON escaping, header management, and response parsing. The prompt changes and you break the JSON. The API returns an error and you parse it wrong.

## Before and after

**Before**

```bash
curl -s https://api.anthropic.com/v1/messages \
  -H "x-api-key: $ANTHROPIC_API_KEY" \
  -H "content-type: application/json" \
  -d '{"model":"claude-sonnet-4-6","max_tokens":1024,"messages":[{"role":"user","content":"Summarize this"}]}'
```

**After**

```bash
cat article.md | vrk prompt --system 'Summarize this' --model claude-sonnet-4-6
```

## Example

```bash
cat article.md | vrk prompt --system 'Summarize this' --model claude-sonnet-4-6 --json
```

## Exit codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | API failure, budget exceeded, or schema mismatch |
| 2 | Usage error - no input, missing flags |

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--model` | -m | string | LLM model (default from VRK_DEFAULT_MODEL or claude-sonnet-4-6) |
| `--system` |   | string | System prompt text, or @file.txt to read from file |
| `--budget` |   | int | Exit 1 if prompt exceeds N tokens |
| `--fail` | -f | bool | Fail on non-2xx API response or schema mismatch |
| `--json` | -j | bool | Emit response as JSON envelope with metadata |
| `--quiet` | -q | bool | Suppress stderr output |
| `--schema` | -s | string | JSON schema for response validation |
| `--explain` |   | bool | Print equivalent curl command, no API call |
| `--retry` |   | int | Retry N times on schema mismatch (escalates temperature) |
| `--endpoint` |   | string | OpenAI-compatible API base URL |

