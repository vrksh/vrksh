---
title: "Batch LLM with rate limiting"
meta_title: "Batch LLM with rate limiting - vrk pipeline recipe"
description: "Prevents failure at job 847 of 10,000 - throttle paces the pipeline, tok gates each doc before the API call wastes a request. Process a large ..."
why: "Prevents failure at job 847 of 10,000 - throttle paces the pipeline, tok gates each doc before the API call wastes a request."
body: "Process a large document set without hitting API rate limits. Safe to rerun."
slug: "batch-llm-with-rate-limiting"
steps:
  - "for f in docs/*.md; do cat \"$f\" | vrk tok --check 8000 | vrk throttle --rate 60/m | vrk prompt --json | vrk kv set \"result:$(basename $f)\"; done"
tags:
  - "tok"
  - "throttle"
  - "prompt"
  - "kv"
---
