## How it works

`vrk pct` percent-encodes and decodes text, as defined in RFC 3986, so you can build safe URLs in a pipeline.

### Encode (default)

```bash
$ echo 'hello world' | vrk pct --encode
hello%20world

$ echo 'key=value&other=data' | vrk pct --encode
key%3Dvalue%26other%3Ddata
```

A space becomes `%20`. Every RFC 3986 reserved character is encoded.

### Encode (form mode)

```bash
$ echo 'hello world' | vrk pct --encode --form
hello+world
```

A space becomes `+` instead of `%20`. Use this for HTML form data and `application/x-www-form-urlencoded` content.

### Decode

```bash
$ echo 'hello%20world' | vrk pct --decode
hello world

$ echo 'hello+world' | vrk pct --decode --form
hello world
```

Without `--form`, `+` stays as `+`. With `--form`, `+` decodes to a space.

### Batch processing

pct reads the input line by line:

```bash
$ printf 'hello world\nfoo bar\n' | vrk pct --encode
hello%20world
foo%20bar
```

### JSON output

```bash
$ echo 'hello world' | vrk pct --encode --json
{"input":"hello world","output":"hello%20world","op":"encode","mode":"percent"}
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

If you pass both `--encode` and `--decode`, pct exits 2:

```bash
$ echo 'test' | vrk pct --encode --decode
usage error: pct: flags --encode and --decode are mutually exclusive
$ echo $?
2
```

If you pass neither flag, pct exits 2:

```bash
$ echo 'test' | vrk pct
usage error: pct: either --encode or --decode is required
$ echo $?
2
```

If the input has an invalid percent sequence, pct exits 1:

```bash
$ echo '%ZZ' | vrk pct --decode
error: pct: invalid percent-encoded input: invalid URL escape "%ZZ"
$ echo $?
1
```
