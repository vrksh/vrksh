## How it works

### Decode a token

```bash
$ echo 'eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiZXhwIjoxNjAwMDAwMDAwLCJpYXQiOjE1MTYyMzkwMjJ9.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c' | vrk jwt
{"exp":1600000000,"iat":1516239022,"name":"John Doe","sub":"1234567890"}
```

The full payload as JSON, sorted alphabetically. No signature verification - this is for inspection only.

### Extract a single claim

```bash
$ echo "$TOKEN" | vrk jwt --claim name
John Doe

$ echo "$TOKEN" | vrk jwt --claim sub
1234567890
```

Returns the raw value, no JSON wrapping. Useful for extracting a user ID or email in a script.

### Check if a token is expired

```bash
$ echo "$TOKEN" | vrk jwt --expired
error: jwt: token expired 48581h20m37s ago (exp: 2020-09-13T12:26:40Z)
$ echo $?
1
```

Exits 0 if the token is still valid. Exits 1 if expired. The error message shows when it expired and how long ago.

### Full time validity check

```bash
echo "$TOKEN" | vrk jwt --valid
```

Checks three things: not expired (`exp`), not before valid time (`nbf`), and not issued in the future (`iat`). Exits 1 if any check fails.

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
$ echo "$TOKEN" | vrk jwt --expired
error: jwt: token expired 48581h20m37s ago (exp: 2020-09-13T12:26:40Z)
$ echo $?
1
```

Invalid token format:

```bash
$ echo "not.a.jwt" | vrk jwt
error: jwt: invalid token: illegal base64 data
$ echo $?
1
```

No input:

```bash
$ vrk jwt
usage error: jwt: no token provided
$ echo $?
2
```
