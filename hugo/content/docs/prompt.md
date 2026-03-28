---
title: "vrk prompt"
description: "LLM prompt - Anthropic/OpenAI, --schema, --retry, --explain."
tool: prompt
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → prompt → stdout`

Exit 0 Success · Exit 1 API failure, budget exceeded, or schema mismatch · Exit 2 Usage error - no input, missing flags

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--model` | -m | string | LLM model (default from VRK_DEFAULT_MODEL or claude-sonnet-4-6) |
| `--budget` |   | int | Exit 1 if prompt exceeds N tokens |
| `--fail` | -f | bool | Fail on non-2xx API response or schema mismatch |
| `--json` | -j | bool | Emit response as JSON envelope with metadata |
| `--quiet` | -q | bool | Suppress stderr output |
| `--schema` | -s | string | JSON schema for response validation |
| `--explain` |   | bool | Print equivalent curl command, no API call |
| `--retry` |   | int | Retry N times on schema mismatch (escalates temperature) |
| `--endpoint` |   | string | OpenAI-compatible API base URL |

## Example

```bash
echo "Summarize this" | vrk prompt --model claude-sonnet-4-6 --json
```
