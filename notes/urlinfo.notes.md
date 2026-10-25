## How it works

`vrk urlinfo` splits a URL into its parts (scheme, host, port, path, query, fragment, user) and prints them as JSON.

### Full JSON output

```bash
$ vrk urlinfo 'https://api.example.com:8080/v1/search?q=llm+tools&limit=10#results'
{"scheme":"https","host":"api.example.com","port":8080,"path":"/v1/search","query":{"limit":"10","q":"llm tools"},"fragment":"results","user":""}
```

urlinfo puts every URL part in one JSON object. It parses the query parameters into a nested object.

### Extract a single field (--field)

```bash
$ vrk urlinfo --field host 'https://api.example.com:8080/v1/search'
api.example.com

$ vrk urlinfo --field path 'https://api.example.com:8080/v1/search'
/v1/search

$ vrk urlinfo --field query.q 'https://api.example.com?q=llm+tools'
llm tools
```

A dot path reaches a query parameter, for example `query.q` or `query.page`.

### Batch processing

```bash
$ printf 'https://a.com/path\nhttps://b.com/other\n' | vrk urlinfo --field host
a.com
b.com
```

urlinfo reads one URL per line.

### JSON metadata (--json)

```bash
$ printf 'https://a.com\nhttps://b.com\n' | vrk urlinfo --json
{"scheme":"https","host":"a.com",...}
{"scheme":"https","host":"b.com",...}
{"_vrk":"urlinfo","count":2}
```

## Available fields

| Field | Example value |
|-------|--------------|
| `scheme` | `https` |
| `host` | `api.example.com` |
| `port` | `8080` (0 if not specified) |
| `path` | `/v1/search` |
| `query` | `{"q":"llm tools","limit":"10"}` |
| `query.<key>` | value of a specific parameter |
| `fragment` | `results` |
| `user` | username (from `user@host` URLs) |

## Pipeline integration

### Group URLs by domain

```bash
# Extract unique domains from a list of URLs
cat urls.txt | while IFS= read -r url; do
  vrk urlinfo --field host "$url"
done | sort -u
```

### Extract and decode query parameters

```bash
# Get a query parameter and decode it
vrk urlinfo --field query.q 'https://example.com?q=hello%20world' | vrk pct --decode
```

### Count the links on a web page by domain

```bash
# Grab a page, extract links, group by domain
vrk grab https://example.com | vrk links --bare | \
  while IFS= read -r url; do
    vrk urlinfo --field host "$url"
  done | sort | uniq -c | sort -rn
```

## When it fails

If the URL is invalid, urlinfo exits 1:

```bash
$ vrk urlinfo 'not a url'
error: urlinfo: invalid URL: not a url
$ echo $?
1
```

If you give no input on an interactive terminal, urlinfo exits 2:

<!-- doccheck: skip (needs an interactive terminal, doccheck runs with empty stdin) -->
```bash
$ vrk urlinfo
usage error: urlinfo: no input: pipe a URL to stdin
$ echo $?
2
```
