---
title: "Agent Endpoints"
meta_title: "Agent Integration - Machine-Readable Endpoints for AI Agents"
description: "Find the endpoints and commands that let an AI agent discover and use vrk: manifest, skills, llms.txt, MCP, and a bootstrap script."
noindex: false
---

## Why give vrk to an agent

vrk gives an AI agent real tools to count tokens, validate output, and pace API calls. Most agents have no such tools. They approximate with a line in the system prompt ("keep responses under 4000 tokens") or skip the check. The agent runs the same tools a developer runs in the terminal, through the shell.

Add `vrk --skills` to your agent's context, and the agent learns what every tool does without extra explanation. The contract is stable: stdin in, stdout out, exit 0 means continue, and exit 1 means stop. An agent that can run a subprocess can use any vrk tool. It needs no SDK, client library, or language-specific wrapper.

The process boundary is the API.

## Give your agent vrk

Your agent needs to know three things: which tools exist, what each tool does, and how to combine them. vrk gives this information as machine-readable endpoints. Agents with shell access can use CLI commands. Agents that can fetch can use HTTP URLs. The agent does not need to parse HTML.

## Discovery endpoints

| Endpoint                          | Format     | What it provides                                         | CLI equivalent     |
|-----------------------------------|------------|----------------------------------------------------------|--------------------|
| [`/manifest.json`](/manifest.json) | JSON       | Tool registry - name and one-line description for all 28 tools | `vrk --manifest`   |
| [`/skills.md`](/skills.md)         | Markdown   | Complete reference: every flag, exit code, and compose pattern  | `vrk --skills`     |
| [`/skills/tok.md`](/skills/tok.md) | Markdown   | Single-tool reference (one file per tool, lower token cost)    | `vrk --skills tok` |
| [`/agents.md`](/agents.md)         | Markdown   | Agent orientation, anti-patterns, MCP config                   | -                  |
| [`/llms.txt`](/llms.txt)           | Plain text | LLM discovery convention, categorized tool index               | -                  |
| [`/recipes.yaml`](/recipes.yaml)   | YAML       | Compose patterns as structured data                            | -                  |
| [`/recipes/`](/recipes/)           | HTML       | Human and agent-readable pipeline patterns                     | -                  |

For common pipelines, see [vrk.sh/recipes](https://vrk.sh/recipes/). Each recipe names the problem it solves and the tools it uses.

**If your agent has shell access, use the CLI.** It works offline, costs no tokens to fetch, and always matches the installed version.

```bash
# What tools are available?
vrk --manifest

# How does tok work? (flags, exit codes, gotchas)
vrk --skills tok

# Full reference for all tools (use sparingly - large)
vrk --skills
```

## Adding vrk to your agent's context

Add this block to your agent's `CLAUDE.md`, system prompt, or tool-use instructions. It teaches the agent the key patterns without the full reference:

```markdown
## vrk tools

vrk is 28 Unix tools for working with LLMs, shipped as one static binary.

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

For the full per-tool reference, the agent can run `vrk --skills <tool>` when it needs it. This costs fewer tokens than loading everything into the context at the start.

## MCP integration

If your agent supports [Model Context Protocol](https://modelcontextprotocol.io/), use the vrk MCP server. The agent finds vrk tools in the standard MCP tool list. The server does discovery only. The agent runs the tools itself, as shell commands. See the [MCP page](/mcp/) for details.

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

For automated setups such as CI pipelines, Docker images, and agent provisioning scripts, the agent bootstrap installs vrk and prints an onboarding block:

```bash
curl -fsSL vrk.sh/agent.sh | sh
```

The onboarding block shows the installed version, the three most important tools, and the discovery commands. The agent can read this output and start to use vrk.
