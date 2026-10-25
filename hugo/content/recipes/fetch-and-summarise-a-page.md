---
title: "Fetch and summarise a page"
meta_title: "Fetch and summarise a page - vrk pipeline recipe"
description: "If the page is too long, the pipeline stops before the API call, so you do not pay for a request that cannot fit. Fetch a web page as markdown, check ..."
why: "If the page is too long, the pipeline stops before the API call, so you do not pay for a request that cannot fit."
body: "Fetch a web page as markdown, check its token count, and send it to an LLM for a summary."
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

Fetch a page and extract only the links. `links --bare` prints a URL every time it appears, so `sort -u` removes the duplicates:

```bash
vrk grab https://example.com | vrk links --bare | sort -u
```

Fetch, strip markdown formatting, then count tokens:

```bash
vrk grab https://example.com | vrk plain | vrk tok
```

