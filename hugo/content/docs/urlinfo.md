---
title: "vrk urlinfo"
description: "URL parser - scheme, host, port, path, query, --field"
tool: urlinfo
group: utilities
mcp_callable: true
noindex: false
---

## The problem

You need to extract the host from a URL in a shell script, pull a query parameter from a webhook log, or check which port a list of service endpoints is using. Bash parameter expansion can't parse URLs reliably - it breaks on ports, query strings, and anything with `@` in the userinfo. Python one-liners work but aren't composable. You need a tool that understands RFC 3986 and fits in a pipeline.

`urlinfo` is a pure string parser - no network calls, no DNS lookups. It decomposes a URL into its parts and emits them as JSON. With `--field` you can extract a single component, including nested query parameters via dot-path syntax.

## The fix

```bash
$ echo "https://api.example.com:8443/v2/users?page=2&limit=50" | vrk urlinfo
{"scheme":"https","host":"api.example.com","port":8443,"path":"/v2/users","query":{"limit":"50","page":"2"},"fragment":"","user":""}
```

## Walkthrough

### Full decomposition

```bash
echo "https://user@api.example.com:8443/v2/resource?a=1&b=2#section" | vrk urlinfo
```

<!-- output: verify against binary -->

The output object always has the same keys in the same order: `scheme`, `host`, `port`, `path`, `query`, `fragment`, `user`. `port` is an integer - `0` when the URL has no explicit port. `query` is a `map[string]string`. Passwords are deliberately excluded from the output regardless of what's in the URL; they're stripped silently.

### Extract a single field

```bash
echo "https://api.example.com/path" | vrk urlinfo --field host
echo "https://api.example.com:9200/path" | vrk urlinfo --field port
```

<!-- output: verify against binary -->

`--field` emits the value of that key as plain text (no JSON wrapping). For string fields, just the value. For `port`, the integer as a decimal string. For `query`, the full query map as JSON.

### Extract a query parameter

```bash
echo "https://example.com/search?q=hello+world&page=3" | vrk urlinfo --field query.page
```

<!-- output: verify against binary -->

Dot-path syntax lets you reach into the query map. `query.page` returns the value of the `page` parameter as plain text. If the parameter is absent, the output is empty and the exit code is still 0.

### Invalid URL

```bash
echo "not a url" | vrk urlinfo
```

<!-- output: verify against binary -->

Exits 1. The error message goes to stderr (or stdout as JSON if `--json` is active). A URL with a missing scheme (like `example.com/path`) is parsed as a path-only relative reference, not an error - `scheme` will be empty.

### Batch mode

```bash
cat urls.txt | vrk urlinfo --field host | sort -u
```

One JSON object (or field value) per line. Blank lines produce blank output lines. An invalid URL on any line exits 1 after processing remaining lines.

### JSON metadata trailer

```bash
echo "https://example.com/path" | vrk urlinfo --json
```

With `--json`, a metadata trailer is appended after all records:

```json
{"_vrk":"urlinfo","count":1}
```

## Pipeline example

Extract all unique hosts from links in a README, sorted by frequency:

```bash
vrk links --bare < README.md \
  | vrk urlinfo --field host \
  | sort \
  | uniq -c \
  | sort -rn
```

Check whether any of your service endpoints are still using plain HTTP:

```bash
cat service-urls.txt \
  | vrk urlinfo --field scheme \
  | grep -c "^http$" \
  | xargs -I{} echo "{} services using plain HTTP"
```

Pull a specific query parameter from a log of redirect URLs:

```bash
cat redirects.log \
  | grep "Location:" \
  | awk '{print $2}' \
  | vrk urlinfo --field query.token
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--field` | `-F` | string | `""` | Extract a single field as plain text; supports dot-path for query params (e.g. `query.page`) |
| `--json` | `-j` | bool | `false` | Append `{"_vrk":"urlinfo","count":N}` metadata trailer |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error - invalid URL that cannot be parsed, I/O error |
| 2 | Usage error - interactive TTY with no stdin or positional arg |
