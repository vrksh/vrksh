---
title: "MCP discovery"
description: "Set up the vrk MCP server so your agent can list all vrk tools and their flags. The server does discovery only and runs nothing."
noindex: false
---

## What this gives your agent

The vrk MCP server lists the vrk tools and their flags to an agent. It does not run the tools.

If your agent framework supports [Model Context Protocol](https://modelcontextprotocol.io/), add the config below. 25 tools (tok, grab, mask, validate and the rest) then appear in the agent's tool list, each with its input schema. `prompt`, `kv` and `coax` are not listed, because they call an LLM, write state or run other programs. The agent sees which flags each tool accepts and decides when to use it.

The MCP layer does **discovery, not execution**. The agent still runs the tools as shell commands. This is deliberate. Each tool call is a real process that appears in logs, respects permissions, and behaves as it does in your terminal.

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

The same config works in any agent that supports MCP. The server talks over stdio with JSON-RPC 2.0. It uses no network, no ports, and no auth.

## How it works

`vrk mcp` starts a JSON-RPC 2.0 server over stdio. The server has two methods:

- **`initialize`** - returns server capabilities and the vrksh version
- **`tools/list`** - returns all 25 tool definitions with their input schemas (flags, types, descriptions)

`tools/call` is not implemented, by design. When the agent decides to use a tool, it runs the tool as a shell command:

```bash
echo "hello" | vrk tok --json
```

For example, the agent reads the `tok` definition from `tools/list`. Then it runs this command in its shell tool, not through MCP:

```bash
cat prompt.txt | vrk tok --check 4000 --json
```

To make the agent always do this, add a line to the system prompt: "vrk tools are listed by the vrksh MCP server. Run them as shell commands, for example `echo text | vrk tok`."

This keeps the setup easy to audit. No hidden execution layer sits between the agent and the tool. The agent runs a process, reads stdout, and checks the exit code, as with any other command.

## When to use MCP vs CLI discovery

**Use MCP** if your agent framework supports MCP and you want the tools to appear in the agent's tool list without prompt changes.

**Use CLI discovery** if your agent has shell access and you want less overhead. It needs no server process and no protocol:

```bash
vrk --manifest           # JSON tool registry
vrk --skills tok         # single-tool reference (flags, exit codes, examples)
vrk --skills             # full reference for all tools
```

Both methods give the agent the same information. MCP is more structured. The CLI is simpler. See the [agent endpoints page](/agents/) for the complete list of discovery surfaces.
