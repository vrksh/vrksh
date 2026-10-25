---
title: "Repo token map"
meta_title: "Repo token map - vrk pipeline recipe"
description: "You see which files are too big for your context window before you paste them into a prompt. Count the tokens in every source file and list the ..."
why: "You see which files are too big for your context window before you paste them into a prompt."
body: "Count the tokens in every source file and list the biggest files first."
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

You want to hand a model part of a codebase. You do not know which files are cheap and which ones eat half the window. File size is a poor guide. A 40 KB generated file and a 40 KB hand-written file do not tokenise the same.

## How the pipeline works

`find` lists the source files and skips the tests. For each one, `vrk tok` prints the token count. `printf` puts the count next to the path, `sort -rn` puts the biggest first, and `head` keeps the top ten.

The output looks like this:

```
   1840  ./src/big.go
     14  ./src/small.go
```

`vrk tok` counts with o200k_base, the GPT-5 tokenizer. It is exact for OpenAI models and close for Claude, which is good enough for ranking files.

## Variations

Change the glob for other languages: `-name '*.py'`, `-name '*.ts'`. Skip a folder with `-not -path './vendor/*'`.

Add up the whole tree to see if it fits at all:

```bash
find . -name '*.go' ! -name '*_test.go' -exec cat {} + | vrk tok
```

Each `vrk tok` call is a separate process, one per file. On a repo with thousands of files that adds up. Counting a typical source file takes about 25 ms.

