---
title: "vrk completions"
description: "shell completion script generator - bash, zsh, fish"
tool: completions
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

## Contract

`stdin → completions → stdout`

Exit 0 Script emitted to stdout · Exit 1 Unknown shell argument · Exit 2 No shell argument provided

## Flags

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--json` | -j | bool | Emit errors as JSON |

## Example

```bash
vrk completions bash > ~/.bash_completion.d/vrk
```
