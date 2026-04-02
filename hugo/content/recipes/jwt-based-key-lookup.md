---
title: "JWT-based key lookup"
meta_title: "JWT-based key lookup - vrk pipeline recipe"
description: "Ties storage to identity without custom parsing - the JWT carries the key, so the lookup stays stateless and auditable. Extract a claim from a JWT ..."
why: "Ties storage to identity without custom parsing - the JWT carries the key, so the lookup stays stateless and auditable."
body: "Extract a claim from a JWT and use it as a kv key."
slug: "jwt-based-key-lookup"
steps:
  - "SUB=$(vrk jwt --claim sub \"$TOKEN\")"
  - "vrk kv get \"user:$SUB\""
tags:
  - "jwt"
  - "kv"
---

## The problem

Your API receives a JWT. You need to look up user data based on the token's subject claim. The usual approach is to decode the JWT in your application code, extract the claim, and query a database. That's fine inside an application, but in a shell script, a webhook handler, or an agent workflow, you need a different approach.

## How the pipeline works

`vrk jwt --claim sub` decodes the JWT (without verification - this is for reading claims, not authentication) and prints the value of the `sub` claim. That value becomes the key for a `vrk kv get` lookup.

No JSON parsing. No base64 decoding. No splitting on dots. One command extracts exactly the field you need.

## Variations

Check if the token is expired before using it:

```bash
if ! vrk jwt --expired "$TOKEN"; then
  echo "Token expired" >&2
  exit 1
fi
SUB=$(vrk jwt --claim sub "$TOKEN")
vrk kv get "user:$SUB"
```

Decode the full payload for debugging:

```bash
vrk jwt "$TOKEN"
```

