---
title: "Agent Endpoints"
description: "Machine-readable endpoints for AI agents - discovery, skills, recipes, and context."
noindex: false
---

## Machine-readable endpoints

vrksh publishes several endpoints designed for AI agents to consume programmatically. Each serves a different stage of agent integration: discovery (what tools exist), orientation (how to use them), and composition (how to combine them). All are plain text or structured data - no HTML parsing required.

| Endpoint | Format | What it contains | CLI equivalent |
|----------|--------|-----------------|----------------|
| [`/manifest.json`](/manifest.json) | JSON | Tool registry - name and description for all 26 tools | `vrk --manifest` |
| [`/skills.md`](/skills.md) | markdown | All tools: flags, exit codes, gotchas, compose patterns | `vrk --skills` |
| [`/skills/tok.md`](/skills/tok.md) | markdown | Single tool reference (tok as example - one per tool) | `vrk --skills tok` |
| [`/agents.md`](/agents.md) | markdown | Agent orientation, MCP config, anti-patterns | - |
| [`/llms.txt`](/llms.txt) | plain text | LLM discovery convention, categorised tool index | - |
| [`/recipes.yaml`](/recipes.yaml) | YAML | Compose patterns as structured data | - |

## CLI equivalents

Every discovery endpoint has a CLI counterpart. Agents with shell access should prefer the CLI - it works offline and returns the same content.

```bash
# Full tool registry (JSON)
vrk --manifest           # same as: curl -s vrk.sh/manifest.json

# Complete skills reference (all tools)
vrk --skills             # same as: curl -s vrk.sh/skills.md

# Single tool reference (lower token cost)
vrk --skills tok         # same as: curl -s vrk.sh/skills/tok.md
vrk --skills prompt      # same as: curl -s vrk.sh/skills/prompt.md
```

## Adding vrksh to your agent's context

Drop this into your agent's `CLAUDE.md` (or equivalent system prompt file) to give it access to vrksh tools:

```markdown
## vrksh - Unix tools for AI pipelines

vrksh is installed as `vrk`. Use it for token counting, URL fetching,
secret masking, structured logging, and LLM prompting in shell pipelines.

For tool discovery:
- `vrk --manifest` lists all tools (JSON)
- `vrk --skills <tool>` shows flags, exit codes, and examples for one tool
- `vrk --skills` shows the full reference

Key patterns:
- Always `vrk tok --budget N` before `vrk prompt` to guard context windows
- Always `vrk validate --schema` after `vrk prompt --schema` to verify output
- Use `vrk mask` before logging to redact secrets
- Pipeline order: input -> transform -> guard -> execute -> store
```

## MCP integration

For agents that support [Model Context Protocol](https://modelcontextprotocol.io/), vrksh includes a discovery-only MCP server. See the [MCP page](/mcp/) for setup.

```json
{
  "mcpServers": {
    "vrksh": {
      "command": "vrk",
      "args": ["mcp"]
    }
  }
}
```
