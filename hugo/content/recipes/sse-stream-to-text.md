---
title: "SSE stream to text"
meta_title: "SSE stream to text - vrk pipeline recipe"
description: "Raw SSE is unusable without parsing - sse extracts the content field so downstream tools get clean text, not protocol framing. Parse an SSE stream ..."
why: "Raw SSE is unusable without parsing - sse extracts the content field so downstream tools get clean text, not protocol framing."
body: "Parse an SSE stream and extract text tokens."
slug: "sse-stream-to-text"
steps:
  - "curl -sN $API | vrk sse --event content_block_delta --field data.delta.text | tr -d '\n'"
tags:
  - "sse"
---
