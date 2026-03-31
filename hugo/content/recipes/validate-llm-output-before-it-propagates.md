---
title: "Validate LLM output before it propagates"
meta_title: "Validate LLM output before it propagates - vrk pipeline recipe"
description: "Bad structured output exits 1 before reaching downstream systems - you catch schema drift at the source, not in production. Gate the pipeline on ..."
why: "Bad structured output exits 1 before reaching downstream systems - you catch schema drift at the source, not in production."
body: "Gate the pipeline on schema correctness. Exit 1 on mismatch stops the next stage from running."
slug: "validate-llm-output-before-it-propagates"
steps:
  - "cat doc.txt | vrk prompt --system \"Extract entities as JSON\" --json | vrk validate --schema entities.json | vrk kv set entities"
tags:
  - "prompt"
  - "validate"
  - "kv"
---
