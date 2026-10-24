---
title: "Repo token map"
meta_title: "Repo token map - vrk pipeline recipe"
description: "Shows which files blow your context window before you paste them into a prompt. Count tokens for every source file and list the biggest ones first."
why: "Shows which files blow your context window before you paste them into a prompt."
body: "Count tokens for every source file and list the biggest ones first."
slug: "repo-token-map"
steps:
  - |-
    find . -name '*.go' ! -name '*_test.go' \
      | while read -r f; do printf '%7d  %s\n' "$(vrk tok < "$f")" "$f"; done \
      | sort -rn | head
tags:
  - "tok"
---

## The problem

You want to hand a model part of a codebase. You do not know which files are cheap and which ones eat half the window. Guessing from file size is bad. A 40 KB generated file and a 40 KB hand-written file do not tokenise the same.

## How the pipeline works

`find` lists the source files and skips the tests. For each one, `vrk tok` prints the token count. `printf` puts the count next to the path, `sort -rn` puts the biggest first, and `head` keeps the top ten.

The output looks like this:

```
   1840  ./src/big.go
     14  ./src/small.go
```

`vrk tok` counts with cl100k_base. It is exact for GPT-4 and close for Claude, which is good enough for ranking files.

## Variations

Change the glob for other languages: `-name '*.py'`, `-name '*.ts'`. Skip a folder with `-not -path './vendor/*'`.

Add up the whole tree to see if it fits at all:

```bash
find . -name '*.go' ! -name '*_test.go' -exec cat {} + | vrk tok
```

Each `vrk tok` call is a separate process, one per file. On a repo with thousands of files that adds up. Counting a typical source file takes about 25 ms.

