---
title: "Diff review budget"
meta_title: "Diff review budget - vrk pipeline recipe"
description: "A diff that is too big fails fast and cheap, instead of being cut off halfway through the review. Check the token count of a git diff, then send it ..."
why: "A diff that is too big fails fast and cheap, instead of being cut off halfway through the review."
body: "Check the token count of a git diff, then send it to the model for review only if it fits."
slug: "diff-review-budget"
steps:
  - "git diff main | vrk tok --check 20000 | vrk prompt --system \"Review this diff\""
tags:
  - "tok"
  - "prompt"
---

## The problem

Asking a model to review a diff is one of the most useful things you can do with one. The trouble is the diff nobody planned for: a lockfile update, a generated file, a rename across the tree. It may be 80,000 tokens. The model either refuses, truncates it, or reviews the first part and sounds confident about the rest.

## How the pipeline works

`git diff main` prints your changes against `main`. `vrk tok --check 20000` counts the tokens. At 20,000 or fewer, it passes the diff through unchanged. Over that, it exits 1 with nothing on stdout, and `vrk prompt` has nothing to send.

`vrk prompt` sends the diff with the system prompt `Review this diff`. Put your own review rules in the system prompt, or load them from a file with `--system @review-rules.txt`.

## When the diff is too big

Pick one of these:

- Review part of it: `git diff main -- src/`, or look at `git diff main --stat` first.
- Leave out noise: `git diff main -- . ':!package-lock.json'`.
- Split it with `vrk chunk` and review each piece.

In a script, test the exit code:

```bash
if ! git diff main | vrk tok --check 20000 > review-input.diff; then
  echo "diff too big for one review, see git diff --stat" >&2
  exit 1
fi
vrk prompt --system "Review this diff" < review-input.diff
```

## Set the number from your model

20,000 is an example. Use your model's context window, minus room for the system prompt and the reply.

