---
title: "Agent-safe pipeline with secret masking"
meta_title: "Agent-safe pipeline with secret masking - vrk pipeline recipe"
description: "When agents handle user data, mask before any logging or storage - secrets should never reach an LLM or a kv store unredacted. Full guard pipeline - ..."
why: "When agents handle user data, mask before any logging or storage - secrets should never reach an LLM or a kv store unredacted."
body: "Full guard pipeline - mask secrets, gate on token budget, prompt, validate output."
slug: "agent-safe-pipeline-with-secret-masking"
steps:
  - "cat user-input.txt | vrk mask | vrk tok --check 6000 | vrk prompt --system \"Summarise\" | vrk validate --schema summary.json"
tags:
  - "mask"
  - "tok"
  - "prompt"
  - "validate"
---
