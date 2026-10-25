---
title: "vrk playground"
description: "Run vrk in your browser. Try tok, mask, validate, jwt, sse and 17 more tools on sample files, with no install and no API key."
---

The vrk playground runs the real vrk tools in your browser. The tools are compiled to WebAssembly, so nothing you type leaves the page.

Type a command and press Enter, or pick an example. You can pipe `echo`, `printf` or `cat <file>` into one or more `vrk` tools, the same way you do in a shell.

{{< playground >}}

## What works here

- **Tools:** 22 of the 28 tools. Run `vrk` with no arguments to list them.
- **Commands:** `vrk`, `echo`, `printf` and `cat`. Other shell commands, redirects and `$VARIABLES` do not work.
- **Files:** `cat` reads three sample files: `README.md`, `users.jsonl` and `stream.sse`.
- **Exit codes:** the status line shows the exit code of the last command in the pipe, as bash does.

## What needs your machine

`prompt`, `grab`, `kv`, `sql`, `pdf` and `coax` do not run in the browser. They need the network, a SQLite file, files on disk or other programs. [Install vrk](/install/) to use them. It is one static binary with no dependencies.
