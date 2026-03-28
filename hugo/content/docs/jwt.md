---
title: "vrk jwt"
description: "JWT inspector - decode, --claim, --expired, --valid."
tool: jwt
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You have a JWT from an API response, a cookie, or an authorization header. You
need to see what's inside it - the claims, the expiry, whether it's still valid.
You could paste it into jwt.io, but that sends your token to a third-party
website. You could write a Python script, but that's overkill for a quick check.

## The fix

```bash
$ vrk jwt eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyXzQyIiwiZXhwIjoxNzExNjAwMDAwLCJpYXQiOjE3MTE1OTY0MDB9.test
{"exp":1711600000,"iat":1711596400,"sub":"user_42"}
```

That decodes the JWT and prints the payload to stdout. No network calls,
no dependencies, no secrets leaving your machine.

## Walkthrough

### Extract a single claim

```bash
$ vrk jwt --claim sub eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyXzQyIiwiZXhwIjoxNzExNjAwMDAwLCJpYXQiOjE3MTE1OTY0MDB9.test
user_42
```

Prints just the value of the `sub` claim. Clean output, ready for piping.

### Check if a token is expired

```bash
$ vrk jwt --expired eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiJ1c2VyXzQyIiwiZXhwIjoxNzExNjAwMDAwLCJpYXQiOjE3MTE1OTY0MDB9.test
error: jwt: token expired (exp: 2024-03-28T04:26:40Z)
$ echo $?
1
```

Exit 0 if the token is still valid. Exit 1 if it's expired. Use this as a
gate in shell scripts.

### Full validity check

```bash
vrk jwt --valid eyJhbGciOiJIUzI1NiJ9...
```

The `--valid` flag checks three things: not expired, not "not yet valid"
(`nbf`), and not issued in the future (`iat`). Exit 1 if any check fails.

## Pipeline example

Extract the subject from an auth header and store it:

```bash
vrk jwt --claim sub "$TOKEN" | vrk kv set --ns auth current_user
```

Check token expiry before making an API call:

```bash
vrk jwt --expired "$TOKEN" && vrk grab https://api.example.com/data
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--json` | `-j` | bool | `false` | Emit decoded token as JSON |
| `--claim` | `-c` | string | `""` | Print value of a single claim |
| `--expired` | `-e` | bool | `false` | Exit 1 if the token is expired |
| `--valid` | | bool | `false` | Exit 1 if expired, not-yet-valid, or issued in future |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success or token is valid |
| 1 | Token expired/invalid or runtime error |
| 2 | Usage error - bad format, too many args |
