---
title: "Cache LLM response"
meta_title: "Cache LLM response - vrk pipeline recipe"
description: "Avoids duplicate API calls for identical prompts - the hash keys the cache so reruns are free. Send a prompt, get the request hash, and store the ..."
why: "Avoids duplicate API calls for identical prompts - the hash keys the cache so reruns are free."
body: "Send a prompt, get the request hash, and store the response in kv."
slug: "cache-llm-response"
steps:
  - "RESULT=$(cat doc.txt | vrk prompt --json)"
  - "HASH=$(echo \"$RESULT\" | jq -r '.request_hash')"
  - "echo \"$RESULT\" | jq -r '.response' | vrk kv set \"cache:$HASH\""
tags:
  - "prompt"
  - "kv"
---
