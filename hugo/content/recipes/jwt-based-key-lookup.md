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
