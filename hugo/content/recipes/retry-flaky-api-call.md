---
title: "Retry flaky API call"
meta_title: "Retry flaky API call - vrk pipeline recipe"
description: "Transient 500s don't kill the pipeline - coax retries with backoff so one bad request doesn't stop the run. Wrap an LLM prompt in coax for ..."
why: "Transient 500s don't kill the pipeline - coax retries with backoff so one bad request doesn't stop the run."
body: "Wrap an LLM prompt in coax for exponential backoff retries."
slug: "retry-flaky-api-call"
steps:
  - "vrk coax --times 3 --backoff exp:1s --on 1 -- vrk prompt --system \"Summarise this\" < doc.txt"
tags:
  - "coax"
  - "prompt"
---
