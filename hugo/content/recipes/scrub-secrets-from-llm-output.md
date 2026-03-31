---
title: "Scrub secrets from LLM output"
meta_title: "Scrub secrets from LLM output - vrk pipeline recipe"
description: "Catches secrets the model echoes back before they reach storage - one leaked API key in kv is a breach. Mask any accidentally leaked secrets before ..."
why: "Catches secrets the model echoes back before they reach storage - one leaked API key in kv is a breach."
body: "Mask any accidentally leaked secrets before storing a result."
slug: "scrub-secrets-from-llm-output"
steps:
  - "vrk prompt --system \"summarise this\" < doc.txt | vrk mask | vrk kv set summary"
tags:
  - "prompt"
  - "mask"
  - "kv"
---
