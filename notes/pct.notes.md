## How it works

### Encode (path mode, default)

```bash
$ echo 'hello world' | vrk pct --encode
hello%20world

$ echo 'key=value&other=data' | vrk pct --encode
key%3Dvalue%26other%3Ddata
```

Spaces become `%20`. All RFC 3986 reserved characters are encoded.

### Encode (form mode)

```bash
$ echo 'hello world' | vrk pct --encode --form
hello+world
```

Spaces become `+` instead of `%20`. Use this for HTML form data and `application/x-www-form-urlencoded` content.

### Decode

```bash
$ echo 'hello%20world' | vrk pct --decode
hello world

$ echo 'hello+world' | vrk pct --decode --form
hello world
```

Without `--form`, `+` is left as-is. With `--form`, `+` decodes to a space.

### Batch processing

Processes line by line:

```bash
$ printf 'hello world\nfoo bar\n' | vrk pct --encode
hello%20world
foo%20bar
```

### JSON output

```bash
$ echo 'hello world' | vrk pct --encode --json
{"input":"hello world","output":"hello%20world","op":"encode","mode":"path"}
```

## Pipeline integration

### Build a URL with encoded parameters

```bash
QUERY=$(echo "$SEARCH_TERM" | vrk pct --encode)
vrk grab "https://api.example.com/search?q=$QUERY" | vrk prompt --system 'Summarize'
```

### Decode URL components

```bash
# Parse a URL and decode the query parameter
vrk urlinfo --field query.q 'https://example.com?q=hello%20world' | vrk pct --decode
```

## When it fails

Both --encode and --decode:

```bash
$ echo 'test' | vrk pct --encode --decode
usage error: pct: specify --encode or --decode, not both
$ echo $?
2
```

Neither flag:

```bash
$ echo 'test' | vrk pct
usage error: pct: specify --encode or --decode
$ echo $?
2
```

Invalid percent sequence:

```bash
$ echo '%ZZ' | vrk pct --decode
error: pct: invalid percent-encoding
$ echo $?
1
```
