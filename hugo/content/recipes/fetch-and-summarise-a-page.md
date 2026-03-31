---
title: "Fetch and summarise a page"
meta_title: "Fetch and summarise a page - vrk pipeline recipe"
description: "Catches oversized pages before the API call - no wasted request on a doc that won't fit in context. Grab a URL, check token count, then summarise ..."
why: "Catches oversized pages before the API call - no wasted request on a doc that won't fit in context."
body: "Grab a URL, check token count, then summarise with an LLM."
slug: "fetch-and-summarise-a-page"
steps:
  - "vrk grab https://example.com | vrk tok --check 8000 | vrk prompt --system \"Summarise this page.\""
tags:
  - "grab"
  - "tok"
  - "prompt"
---
