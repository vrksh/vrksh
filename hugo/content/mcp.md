---
title: "MCP"
description: "MCP server for vrksh - let your agent discover 26 tools through Model Context Protocol."
noindex: false
---

## What this gives your agent

Your agent framework supports [Model Context Protocol](https://modelcontextprotocol.io/) and you want it to know about vrk tools. You add three lines of config and every tool - tok, prompt, grab, mask, validate, all 26 of them - appears in your agent's tool list with full input schemas. The agent sees what's available, knows what flags each tool accepts, and can decide when to use them.

The MCP layer handles **discovery, not execution**. It tells your agent what exists. The agent still runs tools as shell commands. This is deliberate - every tool invocation is a real process that appears in logs, respects permissions, and behaves exactly like it would if you ran it yourself from the terminal.

## Setup

### Claude Code

Add to your MCP settings (`.claude/settings.json` or project-level):

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

### Other MCP-compatible agents

The same config structure works anywhere MCP is supported. The server communicates over stdio using JSON-RPC 2.0 - no network, no ports, no auth.

## How it works

`vrk mcp` starts a JSON-RPC 2.0 server over stdio. It implements two methods:

- **`initialize`** - returns server capabilities and the vrksh version
- **`tools/list`** - returns all 26 tool definitions with their input schemas (flags, types, descriptions)

`tools/call` is intentionally not implemented. When the agent decides to use a tool, it executes it as a shell command:

```bash
echo "hello" | vrk tok --json
```

This keeps things auditable. There's no hidden execution layer between the agent and the tool. The agent runs a process, reads stdout, checks the exit code. Same as any other command.

## When to use MCP vs CLI discovery

**Use MCP** when your agent framework speaks MCP natively and you want tools to show up automatically in the agent's tool list without prompt engineering.

**Use CLI discovery** when your agent has shell access and you want lower overhead. No server process, no protocol - just:

```bash
vrk --manifest           # JSON tool registry
vrk --skills tok         # single-tool reference (flags, exit codes, examples)
vrk --skills             # full reference for all tools
```

Both approaches give the agent the same information. MCP is more structured. CLI is simpler. See the [agent endpoints page](/agents/) for the complete list of discovery surfaces.
