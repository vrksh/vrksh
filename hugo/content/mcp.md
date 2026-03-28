---
title: "MCP"
description: "MCP server for vrksh - discovery layer for AI agents."
noindex: false
---

## MCP server

vrksh includes a discovery-only MCP server. It exposes all pipeline tools via the [Model Context Protocol](https://modelcontextprotocol.io/) so agent frameworks can discover what tools are available.

The MCP layer handles **discovery, not execution**. Tools run as shell commands - the MCP server tells your agent what exists and what flags each tool accepts.

## Claude Code

Add to your MCP config:

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

Every tool becomes visible in Claude Code's tool list with its full input schema.

## How it works

`vrk mcp` starts a JSON-RPC 2.0 server over stdio. It responds to:

- **`initialize`** - returns server capabilities
- **`tools/list`** - returns all tool definitions with input schemas

`tools/call` is intentionally not implemented. The agent executes tools via shell:

```bash
echo "hello" | vrk tok --json
```

This keeps the execution model simple and auditable - every tool invocation is a shell command that appears in your agent's logs.

## Alternative: `--manifest`

If you don't need MCP, use `vrk --manifest` for a JSON tool registry:

```bash
vrk --manifest | jq '.tools[].name'
```

Or `vrk --skills` for the full reference with flags, exit codes, and gotchas:

```bash
vrk --skills tok
```

See the [agent endpoints page](/agents/) for a complete index of all machine-readable surfaces.
