---
title: "Fetch and summarise a page"
meta_title: "Fetch and summarise a page - vrk pipeline recipe"
description: "Catches oversized pages before the API call - no wasted request on a doc that won't fit in context. Grab a URL, check token count, then summarise ..."
why: "Catches oversized pages before the API call - no wasted request on a doc that won't fit in context."
body: "Grab a URL, check token count, then summarise with an LLM."
slug: "fetch-and-summarise-a-page"
steps:
  - |-
    vrk grab https://example.com \
      | vrk tok --check 8000 \
      | vrk prompt --system "Summarise this page."
tags:
  - "grab"
  - "tok"
  - "prompt"
---

## The problem

You want an LLM to summarise a web page. But web pages are messy - HTML tags, navigation chrome, cookie banners, ads. And you don't know how long the page is until you fetch it. If it's 20,000 tokens, you've wasted an API call that will either truncate or fail.

## How the pipeline works

`vrk grab` fetches the URL and converts it to clean markdown - no HTML tags, no scripts, no navigation. `vrk tok --check 8000` counts the tokens and passes through only if the document fits in your budget. `vrk prompt` sends the cleaned, size-checked text to the LLM.

Each step can fail independently. URL unreachable? `vrk grab` exits 1. Page too long? `vrk tok` exits 1. API error? `vrk prompt` exits 1. In every case, the pipeline stops at the point of failure.

## Variations

Fetch and extract just the links:

```bash
vrk grab https://example.com | vrk links --bare
```

Fetch, strip markdown formatting, then count tokens:

```bash
vrk grab https://example.com | vrk plain | vrk tok
```

