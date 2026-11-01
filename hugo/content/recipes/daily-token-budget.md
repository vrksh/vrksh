---
title: "Daily token budget"
meta_title: "Daily token budget - vrk pipeline recipe"
description: "The counter refuses the call once the daily limit is reached, so a runaway loop cannot use up your API budget. Add each prompt's token count to a kv ..."
why: "The counter refuses the call once the daily limit is reached, so a runaway loop cannot use up your API budget."
body: "Add each prompt's token count to a kv counter named after today's date, and use assert to stop the call when the total is over the limit."
slug: "daily-token-budget"
steps:
  - "DAY=$(date +%F)"
  - "N=$(vrk tok < prompt.txt)"
  - "USED=$(vrk kv incr \"budget:$DAY\" --by \"$N\")"
  - "echo \"$USED\" | vrk assert '. <= 100000' && vrk prompt < prompt.txt"
tags:
  - "tok"
  - "kv"
  - "assert"
  - "prompt"
---

## The problem

An agent in a loop can call a model hundreds of times before anyone looks. Provider spend limits help, but they are per account and they fail after the money is gone. You want a limit that lives in your pipeline and fails the step.

## How the pipeline works

The key is the date, so the budget resets itself at midnight. No cron job.

1. `vrk tok` counts the tokens in the prompt.
2. `vrk kv incr "budget:$DAY" --by "$N"` adds that count to today's total and prints the new total. A missing key starts at 0.
3. `vrk assert '. <= 100000'` reads that total from the pipe, passes it through, and exits 1 if it is over the limit. When that happens, `&&` skips `vrk prompt`.

The check runs after the add, so the call that crosses the line is the first one refused. The counter has already included it. If you would rather not count refused calls, subtract `N` again with `vrk kv decr`.

## Reading the counter

```bash
vrk kv get "budget:$(date +%F)"
```

Add `--ns` to keep budgets for different agents apart:

```bash
vrk kv incr --ns agent-a "budget:$DAY" --by "$N"
```

## What it counts

As written, only prompt tokens, counted by `vrk tok` before the call. Replies are not in the total. To include them, run `vrk prompt --json`, read `response_tokens` from the envelope, and add that to the same counter with a second `vrk kv incr --by`. The tokenizer is an estimate for non-OpenAI models, so set the limit a little under your real one.

