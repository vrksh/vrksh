---
title: "Agent Endpoints"
meta_title: "Agent Integration - Machine-Readable Endpoints for AI Agents"
description: "Machine-readable endpoints for AI agents - discovery, skills, and context. CLI and HTTP."
noindex: false
---

## Why give vrk to an agent

Most agents have no way to count tokens, validate output, or pace API calls.
They either approximate this in the system prompt ("keep responses under 4000 tokens")
or skip it entirely. vrk gives agents real tools - the same ones a developer
uses from the terminal - accessible via shell.

Drop `vrk --skills` into your agent's context and it knows what every tool does
without you explaining it. The contract is stable: stdin in, stdout out, exit 0
means continue, exit 1 means stop. An agent that can run a subprocess can use
any vrk tool without an SDK, without a client library, without language-specific
wrappers.

The process boundary is the API.

## Give your agent vrk

Your agent needs to know three things: what tools exist, what each tool does, and how to compose them. vrk exposes this as machine-readable endpoints - both as CLI commands (for agents with shell access) and as HTTP URLs (for agents that can fetch). No HTML parsing, no scraping, no guessing.

## Discovery endpoints

| Endpoint                          | Format     | What it provides                                         | CLI equivalent     |
|-----------------------------------|------------|----------------------------------------------------------|--------------------|
| [`/manifest.json`](/manifest.json) | JSON       | Tool registry - name and one-line description for all 26 tools | `vrk --manifest`   |
| [`/skills.md`](/skills.md)         | Markdown   | Complete reference: every flag, exit code, and compose pattern  | `vrk --skills`     |
| [`/skills/tok.md`](/skills/tok.md) | Markdown   | Single-tool reference (one file per tool, lower token cost)    | `vrk --skills tok` |
| [`/agents.md`](/agents.md)         | Markdown   | Agent orientation, anti-patterns, MCP config                   | -                  |
| [`/llms.txt`](/llms.txt)           | Plain text | LLM discovery convention, categorized tool index               | -                  |
| [`/recipes.yaml`](/recipes.yaml)   | YAML       | Compose patterns as structured data                            | -                  |
| [`/recipes/`](/recipes/)           | HTML       | Human and agent-readable pipeline patterns                     | -                  |

For common pipeline compositions, see [vrk.sh/recipes](https://vrk.sh/recipes/) - each recipe includes the problem it solves and the tools involved.

**If your agent has shell access, use the CLI.** It works offline, costs zero tokens to fetch, and always matches the installed version. The HTTP endpoints exist for agents that can only make web requests.

```bash
# What tools are available?
vrk --manifest

# How does tok work? (flags, exit codes, gotchas)
vrk --skills tok

# Full reference for all tools (use sparingly - large)
vrk --skills
```

## Adding vrk to your agent's context

Drop this block into your agent's `CLAUDE.md`, system prompt, or tool-use instructions. It teaches the agent the key patterns without consuming the full reference:

```markdown
## vrksh tools

vrksh is installed as `vrk`. 26 Unix tools for working with LLMs.

Discovery:
- `vrk --manifest` lists all tools (JSON)
- `vrk --skills <tool>` shows one tool's flags, exit codes, and examples
- `vrk --skills` shows the full reference (all tools)

Key patterns:
- Always `vrk tok --check N` before `vrk prompt` to prevent silent truncation
- Always `vrk mask` before `vrk prompt` when input may contain secrets
- Use `vrk validate --schema` after `vrk prompt --schema` to verify output shape
- Use `vrk coax --times N --backoff exp:200ms` to wrap flaky API calls
```

For the full per-tool reference, the agent can call `vrk --skills <tool>` on demand rather than loading everything into context upfront. This is cheaper and keeps the context window focused.

## MCP integration

For agents that support [Model Context Protocol](https://modelcontextprotocol.io/), vrk includes an MCP server. This lets agents discover and call vrk tools through the standard MCP tool-use interface. See the [MCP page](/mcp/) for details.

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

## Agent bootstrap script

For automated setups - CI pipelines, Docker images, or agent provisioning scripts - the agent bootstrap installs vrk and prints an onboarding block:

```bash
curl -fsSL vrk.sh/agent.sh | sh
```

The onboarding block includes the installed version, the three most important tools, and pointers to the discovery commands. An agent reading its own setup output gets everything it needs to start using vrk immediately.
