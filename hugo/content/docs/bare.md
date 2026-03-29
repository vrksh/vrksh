---
title: "vrk --bare"
description: "Create symlinks so vrksh tools can be called without the vrk prefix."
tool: bare
group: meta
mcp_callable: false
noindex: false
---

## What it does

`vrk --bare` creates symlinks in the same directory as the `vrk` binary so
that every tool can be called directly by name, without the `vrk` prefix.
After running it, `tok`, `jwt`, `grab` and every other tool work as
standalone commands.

## When to use it

The default interface is `vrk tok`, `vrk grab`, `vrk jwt`. That is one
extra word per invocation. In pipelines that chain several tools, the
`vrk` prefix adds visual noise:

```bash
cat doc.txt | vrk mask | vrk tok --budget 8000 && cat doc.txt | vrk prompt --system "Summarize."
```

With bare symlinks:

```bash
cat doc.txt | mask | tok --budget 8000 && cat doc.txt | prompt --system "Summarize."
```

Use `--bare` when you want shorter commands in scripts, CI environments,
or personal shells. Skip it when you want a single binary with no
filesystem footprint beyond itself.

## Example

Preview what would happen without making changes:

```bash
$ vrk --bare --dry-run
Would link assert   → /usr/local/bin/assert
Would link chunk    → /usr/local/bin/chunk
Would link tok      → /usr/local/bin/tok
...
26 linked.
```

Create the symlinks:

```bash
vrk --bare
```

Link only specific tools:

```bash
vrk --bare tok jwt grab
```

List currently active bare symlinks:

```bash
vrk --bare --list
```

Remove all bare symlinks (only removes those pointing back to `vrk`):

```bash
vrk --bare --remove
```

If a file already exists at a symlink path, `--bare` skips it.
Use `--force` to overwrite:

```bash
vrk --bare --force
```

## In a pipeline

Bare mode is not used mid-pipeline. It is a one-time setup step.
The symlinks it creates are what you use in pipelines:

```bash
grab https://example.com | mask | tok --budget 8000
```

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--force` | bool | `false` | Overwrite existing files at symlink paths |
| `--remove` | bool | `false` | Remove bare symlinks (only those pointing to vrk) |
| `--list` | bool | `false` | List currently active bare symlinks |
| `--dry-run` | bool | `false` | Show what would happen, make no changes |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Filesystem error creating or removing symlinks |
| 2 | Usage error |
