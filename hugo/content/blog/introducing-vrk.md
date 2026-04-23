---
title: The 50-year-old idea behind vrk
description: "A design philosophy from 1974 turns out to be exactly right for LLM pipelines. Here's why the Unix contract is load-bearing infrastructure for agents."
date: 2026-04-23
lastmod: 2026-04-23
tags: [Unix, Agents, AI, LLM, CLI]
keywords: ["vrk", "AI CLI", "LLM pipeline", "token counter", "unix tools", "agent tools", "no silent failures"]
comments: true
summary: "LLMs are probabilistic. The tools around them shouldn't be. vrk is 26 Unix-style tools for LLM pipelines -- built on the same contract that made grep reliable fifty years ago."
category: engineering
slug: "introducing-vrk"
og_image: "/og/default.png"
og_image_alt: "vrk | Unix tools for the agent era"
featured: true
---

At Yahoo! in 2005, my QA system was a pipe character.

Not a testing framework. Not a dashboard. I was working on a large data processing pipeline and my way of verifying it was to pipe the output through `cut`, `join`, `zcat`, `zgrep`, and `sort`, look at what came out, and pipe it somewhere else. Each tool did one thing. You chained them together. If something looked wrong, you broke the chain at that point and inspected it.

The individual tools were boring. What they composed into was not.

I didn't think much about why this worked. It just worked. You'd run a filter, look at what came out, move on. The Unix contract, stdin in, stdout out, clean exits, was so simple it was almost invisible.

Twenty one years later I'm building vrk and I keep arriving at the same contract. Which made me think: maybe this wasn't just a convenient old interface. Maybe it was the right design, and we're only now understanding why.


An agent needs to fetch documentation from a URL, make sure it's not too large to send to a model, redact any secrets in it, call the model, and validate the output against a schema. Five steps. In Python you'd write maybe 25 lines. It would work once, on your machine, if nothing changes.

Or you could do this:

```bash
vrk grab https://api.example.com/docs \
  | vrk tok --check 8000 \
  | vrk mask \
  | vrk prompt --system "extract breaking changes" \
  | vrk validate --schema changes.json
```

The point is not that the second version is shorter. The point is that each step either works or stops. `tok --check 8000` exits with an error if the document is over the token limit. The pipeline stops there. Nothing downstream runs on bad input. You know exactly where it failed and why.

This is what "no silent failures" means in practice. The Python version might pass a truncated document to the model and produce a confident, wrong answer. You'd never know.

---

Silent failures are the hard problem in LLM pipelines.

A model doesn't crash when its input is wrong. It guesses. It fills in the gaps with plausible-sounding text. If you fed it a truncated document, it'll summarise what it got and never mention what it didn't see. You find out three steps later, or you don't find out at all.

LLMs are probabilistic. That's not a bug, it's the point. But the tools around them shouldn't be. A tool that exits cleanly on success, loudly on failure, and never shrugs on bad input -- that's load-bearing infrastructure for an agent. The composability is nice. The predictable exits are essential.

---

That's why vrk exists.

The tools in the pipeline above -- `grab`, `tok`, `mask`, `prompt`, `validate` -- each one exits cleanly or not at all. They don't know about each other. They don't need to. The design constraints were the same ones Unix was built on fifty years ago: one thing per tool, clean exits, no hidden state. The only difference is what you're piping through them.

```bash
curl -fsSL vrk.sh/install.sh | sh
```

One binary. Zero runtime dependencies. Linux and macOS on amd64 and arm64.

Fifty years of software history and we're back to the same design. Maybe it was the right design all along. We're betting on it.
