---
title: "About"
meta_title: "About vrksh - Unix CLI Tools for AI Pipelines"
description: "vrk is one static binary with 28 Unix tools for LLM pipelines. Learn why it exists, what its contract is, and when to use it."
noindex: false
---

## Why vrksh exists

vrk is one static binary with 28 Unix tools for working with LLMs. Each tool reads stdin, writes stdout, and returns an exit code that a pipeline can trust.

If you work with LLMs, the same questions come up again and again. How many tokens is this document? Did the model's JSON match the schema? Is there a secret in this log that I am about to send to an API?

These are not hard problems, but the solutions are scattered. Token counting is a Python library. Retry with backoff is another library. Secret redaction is a third, and schema validation is a fourth. Each one needs a runtime and only works inside your Python process. Your agent cannot call them. Your CI pipeline cannot call them. A developer on a production box at 2 AM cannot call them.

vrk puts all of this in one place. The 28 tools cover token counting, API calls, schema validation, secret redaction, retry logic, rate limiting, and state storage. Each tool is a Unix command: stdin in, stdout out, and exit codes that mean something.

You can run the tools from the terminal. A Python script can call them as subprocesses. An agent can use them as tools, and a cron job can chain them. The binary is static, has no runtime dependencies, and works the same on every supported platform.

## The name

vrksh (वृक्ष) is the Sanskrit word for tree. The formal project name is **vrksh**, but in writing and code just call it **vrk** - that's what you type, what you see in pipelines, and what the domain (vrk.sh) spells. Reserve "vrksh" for the formal framing: the Sanskrit etymology, the Homebrew tap (`vrksh/vrksh`), and the GitHub org.

Pronunciation: **vrksh** is "vruksh", **vrk** is "vruk" (rhymes with truck).

## The contract

Every tool follows the same rules.

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

These rules make the tools easy to chain. To find out if a step failed, read the exit code. Do not parse stderr. Stdout is always the data, so you never have to guess the output format.

Every caller gets the same interface. A terminal, a Python subprocess call, an agent, and a CI step all use vrk the same way. Exit 0 means continue. Exit 1 means stop. Exit 2 means the command was wrong. You need no SDK, client library, or language-specific wrapper. The process boundary is the API.

## What vrksh is not

vrksh is not a Python library. It has no import, no runtime, and no virtualenv.
This is by design. vrk works the same way from a Python subprocess, a bash
script, a Go program, an AI agent, a CI step, or a terminal on a production box.

`vrk tok` is not tiktoken. tiktoken counts tokens inside a Python process.
`vrk tok` is a shell command, so any program that can run a subprocess can use it.

`vrk coax` is not tenacity or backoff. Those are Python decorators. `vrk coax`
wraps any shell command, in any language.

`vrk validate` is not the jsonschema library. It is a pipeline gate. If the
input does not match the schema, it exits 1 and the next command does not run.
You do not write error-handling code around it.

## When to use vrksh

- You need token counting outside a Python process - in CI, bash scripts, or agents
- You need to validate LLM output against a JSON schema before it propagates downstream
- You need secret redaction in a pipeline, not as a library call inside one program
- You want retry and backoff logic around any shell command, not just Python functions
- You're building an AI agent that calls tools via subprocess
- You want one install, no dependencies, and the same interface on every platform

## When NOT to use vrksh

- You are already inside a Python process and want a library. Use tiktoken, tenacity, or jsonschema directly. vrk is for callers outside your process.
- You need GPU-accelerated inference. `vrk prompt` calls APIs and does not run models locally.
- You want a framework that manages state and orchestrates steps. vrksh is a set of tools. Each tool does one step, and your pipeline sets the order.

## How the binary works

One binary, 28 tools. vrk uses multicall dispatch - the first argument selects which tool runs:

```bash
vrk tok              # count tokens
vrk prompt           # call an LLM
vrk grab             # fetch a URL as clean markdown
vrk mask             # redact secrets
vrk chunk            # split text into token-sized pieces
```

26 of 28 tools need no configuration. Only [prompt](/docs/prompt/) and [kv](/docs/kv/) read [environment variables](/env/). No config file.

Two built-in flags for agent discovery:

```bash
vrk --manifest       # JSON tool list (for programmatic discovery)
vrk --skills         # full reference with flags, exit codes, gotchas
vrk --skills tok     # reference for a single tool (lower token cost)
```

The binary contains the manifest and the skills reference. An agent can run `vrk --skills tok` to learn about a tool without reading documentation or using the network.

## Bare mode

Typing `vrk` before every command is slow in an interactive session. [Bare mode](/docs/bare/) creates symlinks so you can call each tool by its own name:

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

If a file already exists at a symlink path, bare mode skips it and warns you. Use `--force` to overwrite it. `--remove` only deletes symlinks that point to vrk. It never touches a file it did not create.

## Shell completions

Tab completion for bash, zsh, and fish:

```bash
vrk completions bash > ~/.bash_completion.d/vrk
vrk completions zsh > ~/.zsh/completions/_vrk
vrk completions fish > ~/.config/fish/completions/vrk.fish
```

After you source the file, `vrk <tab>` completes tool names and `vrk tok --<tab>` completes flags. The binary generates the completions, so they match the version you have installed.

## FAQ

<details>
<summary>Why not just write a Python script?</summary>

You can. But a Python script only works inside Python. It needs a virtualenv, a runtime, and an import. vrk works from bash, Go, a cron job, an AI agent, or a CI step. Any caller gets the same interface, with no SDK, client library, or language-specific wrapper.
</details>

<details>
<summary>Why not jq + curl + shell functions?</summary>

jq reshapes JSON and curl makes HTTP requests. Neither counts tokens, validates against a JSON schema, redacts secrets by entropy, or retries with exponential backoff. vrk covers these tasks for LLM pipelines. Every vrk tool uses the same exit code contract (0/1/2), so error handling is the same across all 28 tools.
</details>

<details>
<summary>Does this work with local models (Ollama, vLLM)?</summary>

Yes. `vrk prompt --endpoint http://localhost:11434/v1` works with any OpenAI-compatible API. Ollama, vLLM, LiteLLM, and LocalAI all work.
</details>

<details>
<summary>Why 28 tools? Isn't that scope creep?</summary>

Every tool solves a problem that comes up often when you work with LLMs. Agents need the utility tools (uuid, epoch, slug, recase) all the time, for example to make run IDs, convert timestamps, and normalize names. These tools ship in the same binary, so they add no install step. If you do not need a tool, do not call it.
</details>

<details>
<summary>Does vrk work offline?</summary>

Every tool except `vrk prompt` and `vrk grab` works offline. Token counting, secret masking, schema validation, encoding, and hashing all run locally.
</details>
