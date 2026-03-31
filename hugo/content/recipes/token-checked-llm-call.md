---
title: "Token-checked LLM call"
meta_title: "Token-checked LLM call - vrk pipeline recipe"
description: "Prevents silent truncation - the model never sees a prompt it can only half-fit. Count tokens before sending to an LLM - abort if too large."
why: "Prevents silent truncation - the model never sees a prompt it can only half-fit."
body: "Count tokens before sending to an LLM - abort if too large."
slug: "token-checked-llm-call"
steps:
  - "cat prompt.txt | vrk tok --check 4000 | vrk prompt --system \"Summarise this.\""
tags:
  - "tok"
  - "prompt"
---
