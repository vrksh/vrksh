---
title: "JWT-based key lookup"
meta_title: "JWT-based key lookup - vrk pipeline recipe"
description: "The JWT carries the key, so the lookup needs no custom parsing and no extra state. Read the sub claim from a JWT and use it as a key to look up data ..."
why: "The JWT carries the key, so the lookup needs no custom parsing and no extra state."
body: "Read the sub claim from a JWT and use it as a key to look up data in kv."
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

One command extracts the field you need. You do not parse JSON, decode base64, or split on dots.

## Variations

Check if the token is expired before using it. `--expired` prints the payload when the token is still valid, so the check sends that to `/dev/null` and only uses the exit code:

```bash
if ! vrk jwt --expired "$TOKEN" >/dev/null; then
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

