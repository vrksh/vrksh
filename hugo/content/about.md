---
title: "About"
description: "What vrk is, why it exists, and how the binary works."
noindex: false
---

## Why vrksh exists

You're working with LLMs. Maybe you're building pipelines in Python. Maybe your agent needs to call tools. Maybe you're debugging a prompt from the terminal at 11 PM. Wherever you are, the same problems keep showing up: How many tokens is this document? Did the model's JSON actually match the schema? Is there a secret in this log I'm about to send to an API?

These aren't hard problems. But the solutions are scattered. Token counting is a Python library. Retry with backoff is another library. Secret redaction is a third. Schema validation is a fourth. Each one pulls in dependencies, needs a runtime, and only works inside your Python process. Your agent can't call them. Your CI pipeline can't call them. The developer SSHed into a production box at 2 AM definitely can't call them.

vrk puts all of it in one place. 26 tools for the things that come up constantly when you work with LLMs - token counting, API calls, schema validation, secret redaction, retry logic, rate limiting, state persistence. Each one is a Unix command: stdin in, stdout out, exit codes that mean something. A developer can run them from the terminal. A Python script can shell out to them. An agent can call them as tools. A cron job can chain them together. One static binary, no runtime dependencies, works the same on every platform.

## The name

vrksh (वृक्ष) is the Sanskrit word for tree. The project is **vrksh**. The command is `vrk`. Use the full name when referring to the project; use the command name in code.

vrk is pronounced "vruk" and rhymes with truck.

## The contract

Every tool follows the same rules. No exceptions.

```
stdin   ->  data in
stdout  ->  data out
stderr  ->  errors and warnings only
exit 0  ->  success
exit 1  ->  failure (bad input, API error, condition not met)
exit 2  ->  usage error (bad flags, missing input)
--json  ->  errors go to stdout as {"error":"...","code":N}, stderr empty
--help  ->  always works, even with no stdin
```

This is what makes the tools composable. You don't parse stderr to check if something failed - you check the exit code. You don't guess the output format - stdout is always the data.

It also means every caller gets the same interface. A developer typing in a terminal, a Python subprocess call, an agent executing a tool, a CI step in a GitHub Action - they all interact with vrk the same way. Exit 0 means continue. Exit 1 means stop. Exit 2 means the command was wrong. No SDK, no client library, no language-specific wrapper. The process boundary is the API.

## What vrksh is not

vrksh is not a Python library. It has no import, no runtime, no dependency
on a virtualenv. This is the design - it means vrk works identically from a
Python subprocess, a bash script, a Go program, an AI agent, a CI step, or
a terminal at 2 AM on a production box.

`vrk tok` is not tiktoken. tiktoken counts tokens inside a Python process.
`vrk tok` is a shell command - any program that can run a subprocess can use it.

`vrk coax` is not tenacity or backoff. Those are Python decorators. `vrk coax`
wraps any shell command, any language, any binary.

`vrk validate` is not jsonschema the library. It is a pipeline gate - exit 1
on schema mismatch stops the next command from running. You do not write
error-handling code around it. The Unix pipeline handles it.

## How the binary works

One binary, 26 tools. vrk uses multicall dispatch - the first argument selects which tool runs:

```bash
vrk tok              # count tokens
vrk prompt           # call an LLM
vrk grab             # fetch a URL as clean markdown
vrk mask             # redact secrets
vrk chunk            # split text into token-sized pieces
```

Two built-in flags for agent discovery:

```bash
vrk --manifest       # JSON tool list (for programmatic discovery)
vrk --skills         # full reference with flags, exit codes, gotchas
vrk --skills tok     # reference for a single tool (lower token cost)
```

The manifest and skills reference are embedded in the binary. An agent can call `vrk --skills tok` to learn everything about a tool without reading documentation or making network calls.

## Bare mode

Typing `vrk` before every command adds friction in interactive sessions. [Bare mode](/docs/bare/) creates symlinks so you can call tools directly by name:

```bash
vrk --bare                     # link all tools
tok --check 8000 < prompt.txt  # now works without the prefix
```

Preview first, commit when ready:

```bash
vrk --bare --dry-run           # show what would be created
vrk --bare                     # create the symlinks
vrk --bare --list              # see what's linked
vrk --bare --remove            # undo everything
```

Collisions are handled safely. If a file already exists at a symlink path, bare mode skips it and warns you. Use `--force` to overwrite. `--remove` only deletes symlinks that point to vrk - it will never touch a file it didn't create.

## Shell completions

Tab completion for bash, zsh, and fish:

```bash
vrk completions bash > ~/.bash_completion.d/vrk
vrk completions zsh > ~/.zsh/completions/_vrk
vrk completions fish > ~/.config/fish/completions/vrk.fish
```

After sourcing, `vrk <tab>` completes tool names and `vrk tok --<tab>` completes flags. The completions are generated from the binary itself, so they always match the version you have installed.
