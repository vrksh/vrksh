---
title: "About"
description: "What vrk is, how to say it, and how the binary works."
noindex: false
---

## Philosophy

Agents need deterministic tools. The standard Unix toolkit was designed for humans who handle ambiguity well. vrk fills the gap: small tools that do one thing, fail loudly, and compose predictably. Not smart. Not magical. Reliable.

## What it is

vrksh (वृक्ष) is the Sanskrit word for tree. Pronounced "vruk" - rhymes with truck, not with "work" or "V-R-K."

The binary is `vrk`. The project is `vrksh`. The domain is `vrk.sh`.

A single static binary with 28 Unix-style tools for AI pipelines. Every tool reads stdin, writes stdout, exits 0/1/2. Nothing else.

## How the binary works

vrk uses multicall dispatch - the same binary handles every tool. The first argument selects which tool runs:

```bash
vrk tok              # runs the tok tool
vrk prompt           # runs the prompt tool
vrk --manifest       # shows all 28 tools as JSON
vrk --skills tok     # shows tok's agent skill reference
vrk --help           # top-level usage
```

To see every tool available:

```bash
vrk --manifest
```

This prints the embedded JSON tool manifest - the same manifest that agents use for discovery.

## Bare mode

By default, every tool is accessed through the `vrk` prefix: `vrk tok`, `vrk jwt`, `vrk epoch`. Bare mode creates symlinks so you can call tools directly by name - `tok`, `jwt`, `epoch` - without the prefix.

The symlinks are created in the same directory as the vrk binary. Nothing is copied. Each symlink points back to `vrk`, which detects the name it was called as and dispatches accordingly.

```bash
vrk --bare                     # link all tools
vrk --bare tok jwt epoch       # link only these three
vrk --bare --dry-run           # preview what would happen
vrk --bare --list              # show active symlinks
vrk --bare --remove            # remove all vrk-created symlinks
vrk --bare --remove tok        # remove a specific symlink
```

**Collisions are handled safely.** If a file already exists at a symlink path, bare mode skips it and tells you. Use `--force` to overwrite:

```bash
vrk --bare --force             # overwrite all collisions
vrk --bare --force uuid base   # overwrite only these two
```

Running bare mode twice is safe - it recognizes existing symlinks and reports them as already linked. `--remove` only deletes symlinks that point to vrk. It will never touch a file it did not create.

If the binary directory requires elevated permissions:

```bash
sudo vrk --bare
```

## The contract

Every vrk tool follows these rules:

```
stdin   ->  data in
stdout  ->  data out
stderr  ->  errors only
exit 0  ->  success
exit 1  ->  failure (bad input, API error, condition not met)
exit 2  ->  usage error (bad flags, missing input)
--json  ->  errors go to stdout as {"error":"...","code":N}
--help  ->  always works, always explains the tool
```

This contract is what makes vrk tools composable with each other and callable by agents without special handling. An agent does not need to parse stderr to know if a tool failed - it checks the exit code. It does not need to guess the output format - stdout is always the data.
