# pct - Percent encoder/decoder - RFC 3986, --encode, --decode, --form

When to use: percent-encode or decode text per RFC 3986. Use --form for HTML form encoding (spaces as +) vs path encoding (spaces as %20).
Composes with: urlinfo, grab, jwt

| Flag       | Short | Type | Description                                                            |
|------------|-------|------|------------------------------------------------------------------------|
| `--encode` |       | bool | Percent-encode input (RFC 3986 strict)                                 |
| `--decode` |       | bool | Percent-decode input                                                   |
| `--form`   |       | bool | Form mode: spaces as `+` instead of `%20`                              |
| `--json`   | `-j`  | bool | Emit `{"input":"...","output":"...","op":"...","mode":"..."}` per line |
| `--quiet`  | `-q`  | bool | Suppress stderr                                                        |

Exit 0: success (including empty input)
Exit 1: invalid percent-encoded sequence during decode
Exit 2: neither or both mode flags, unknown flag, interactive terminal

Example:

    echo 'hello world' | vrk pct --encode
    # hello%20world

Anti-pattern:
- Don't assume all percent-encoding is the same. Path encoding (%20 for spaces) and form encoding (+ for spaces) are different. Use --form explicitly when building form POST bodies.
