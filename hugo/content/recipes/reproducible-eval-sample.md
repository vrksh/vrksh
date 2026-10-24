---
title: "Reproducible eval sample"
meta_title: "Reproducible eval sample - vrk pipeline recipe"
description: "Two eval runs see the same prompts, so a score change comes from your change and not from the sample. Sample 50 lines from a JSONL file with a fixed ..."
why: "Two eval runs see the same prompts, so a score change comes from your change and not from the sample."
body: "Sample 50 lines from a JSONL file with a fixed seed."
slug: "reproducible-eval-sample"
steps:
  - "vrk sip --count 50 --seed 7 < prompts.jsonl"
tags:
  - "sip"
---

## The problem

You have thousands of test prompts and you can only afford to run a few dozen per change. A random sample is fine, until you compare two runs on different samples. The score moved, but was it your change or a luckier sample? You cannot tell.

## How the pipeline works

`vrk sip --count 50` keeps 50 lines chosen at random from the stream. It reads one line at a time and holds only 50 in memory, so the file can be as large as you like.

`--seed 7` fixes the random generator. The same seed on the same file gives the same 50 lines. Change the seed to get a different sample. Zero is a valid seed too.

Keep the seed in your eval script or commit it next to the results. That is all the reproducibility you need.

## Feeding it to a model

The sample is still JSONL, so it goes straight into `vrk prompt --field`:

```bash
vrk sip --count 50 --seed 7 < prompts.jsonl \
  | vrk prompt --field text --json > results.jsonl
```

## Notes

The sample is the same only while the input file is the same. Add a line and the sample can change. If the file grows over time, sample from a frozen copy.

`--json` adds a trailer record with the counts, so scripts can confirm they got 50 lines.

