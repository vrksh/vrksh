---
title: "vrk jwt"
description: "vrk jwt decodes a JWT on your machine, prints its claims, and checks expiry with --expired. It does not verify the signature."
meta_lead: "vrk jwt decodes JWT tokens locally without sending them anywhere, and checks expiry and claims."
og_title: "vrk jwt - JWT decoder and claim inspector"
tool: jwt
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk jwt decodes JWT tokens locally without sending them anywhere, and checks expiry and claims.

## The problem

A CI pipeline gets a 401. The token looks valid. You paste it into jwt.io to inspect it, which sends a production token to a third-party server. Or you base64-decode it manually and fight padding errors for twenty minutes. The answer was that the token expired two hours ago.

## The solution

`vrk jwt` decodes JWT tokens locally, without sending them anywhere. It prints the payload as JSON, extracts individual claims with `--claim`, and checks expiry with `--expired`. It does not verify signatures. Use it to inspect a token, not to authenticate one.

## Before and after

**Before**

```bash
echo "$TOKEN" | base64 -d 2>/dev/null | jq .
# base64 padding errors, no expiry check
# or worse: paste into jwt.io
```

**After**

```bash
echo $TOKEN | vrk jwt --expired
```

## Example

```bash
echo $TOKEN | vrk jwt --claim sub
```

## Exit codes

| Code | Meaning                                 |
|------|-----------------------------------------|
| 0    | Success or token is valid               |
| 1    | Token expired/invalid or runtime error  |
| 2    | Usage error - bad format, too many args |

## Flags

| Flag        | Short | Type   | Description                                               |
|-------------|-------|--------|-----------------------------------------------------------|
| `--json`    | -j    | bool   | Emit decoded token as JSON                                |
| `--claim`   | -c    | string | Print value of a single claim                             |
| `--expired` | -e    | bool   | Exit 1 if the token is expired                            |
| `--valid`   |       | bool   | Exit 1 if expired, not-yet-valid, or issued in the future |
| `--quiet`   | -q    | bool   | Suppress stderr output                                    |


<!-- notes - edit in notes/jwt.notes.md -->

## How it works

`vrk jwt` decodes a JWT (JSON Web Token) and prints its payload, one claim, or an expiry check. It never verifies the signature.

### Decode a token

```bash
$ echo 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZXhwIjoxNjAwMDAwMDAwLCJpYXQiOjE1MTYyMzkwMjJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c' | vrk jwt
{"exp":1600000000,"iat":1516239022,"name":"John Doe","sub":"1234567890"}
```

jwt prints the full payload as JSON, sorted alphabetically. It does not verify the signature, so use it for inspection only.

### Extract a single claim

```bash
$ TOKEN='eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZXhwIjoxNjAwMDAwMDAwLCJpYXQiOjE1MTYyMzkwMjJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c'
$ echo "$TOKEN" | vrk jwt --claim name
John Doe

$ echo "$TOKEN" | vrk jwt --claim sub
1234567890
```

jwt prints the raw value with no JSON wrapping. Use it to get a user ID or email in a script.

### Check if a token is expired

```bash
$ TOKEN='eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZXhwIjoxNjAwMDAwMDAwLCJpYXQiOjE1MTYyMzkwMjJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c'
$ echo "$TOKEN" | vrk jwt --expired
error: jwt: token expired ... ago (exp: 2020-09-13T12:26:40Z)
$ echo $?
1
```

Exits 0 if the token is still valid. Exits 1 if expired. The error message shows when it expired and how long ago.

### Full time validity check

```bash
echo "$TOKEN" | vrk jwt --valid
```

`--valid` checks three things: not expired (`exp`), not before the valid time (`nbf`), and not issued in the future (`iat`). If any check fails, it exits 1. If a time claim is present but is not a number (a string, `null`), that also fails, so a malformed token cannot pass. Fractional dates like `1700000000.5` are compared as real dates.

`--valid` and the `valid` field in `--json` cover the time claims only. The signature is never checked, and `--json` says so with `"signature_verified":false`.

### JSON output

```bash
echo "$TOKEN" | vrk jwt --json
```

Wraps the decoded payload in a JSON envelope with header information.

## Pipeline integration

### Check token before API call

```bash
# Only call the API if the token is still valid
echo "$TOKEN" | vrk jwt --expired --quiet && \
  curl -H "Authorization: Bearer $TOKEN" https://api.example.com/data
```

### Extract user info from a token and store it

```bash
USER_ID=$(echo "$TOKEN" | vrk jwt --claim sub)
USER_EMAIL=$(echo "$TOKEN" | vrk jwt --claim email)
vrk kv set --ns auth "user:$USER_ID" "$USER_EMAIL" --ttl 1h
```

### Decode and convert timestamps

```bash
# Get the expiry time as a human-readable date
EXP=$(echo "$TOKEN" | vrk jwt --claim exp)
vrk epoch "$EXP" --iso
```

## When it fails

Expired token:

```bash
$ TOKEN='eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZXhwIjoxNjAwMDAwMDAwLCJpYXQiOjE1MTYyMzkwMjJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c'
$ echo "$TOKEN" | vrk jwt --expired
error: jwt: token expired ... ago (exp: 2020-09-13T12:26:40Z)
$ echo $?
1
```

Invalid token format:

```bash
$ echo "not.a.jwt" | vrk jwt
error: jwt: invalid token: header is not valid JSON
$ echo $?
1
```

No input:

```bash
$ vrk jwt
usage error: jwt: no input: provide as argument or via stdin
$ echo $?
2
```
