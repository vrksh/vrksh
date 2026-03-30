## How it works

### Full JSON output

```bash
$ vrk urlinfo 'https://api.example.com:8080/v1/search?q=llm+tools&limit=10#results'
{"scheme":"https","host":"api.example.com","port":8080,"path":"/v1/search","query":{"limit":"10","q":"llm tools"},"fragment":"results","user":""}
```

Every URL component is extracted into a structured JSON object. Query parameters are parsed into a nested object.

### Extract a single field (--field)

```bash
$ vrk urlinfo --field host 'https://api.example.com:8080/v1/search'
api.example.com

$ vrk urlinfo --field path 'https://api.example.com:8080/v1/search'
/v1/search

$ vrk urlinfo --field query.q 'https://api.example.com?q=llm+tools'
llm tools
```

Dot-path syntax reaches into query parameters: `query.q`, `query.page`, etc.

### Batch processing

```bash
$ printf 'https://a.com/path\nhttps://b.com/other\n' | vrk urlinfo --field host
a.com
b.com
```

Processes multiple URLs, one per line.

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

### Parse links from a web page by domain

```bash
# Grab a page, extract links, group by domain
vrk grab https://example.com | vrk links --bare | \
  while IFS= read -r url; do
    vrk urlinfo --field host "$url"
  done | sort | uniq -c | sort -rn
```

## When it fails

Invalid URL:

```bash
$ vrk urlinfo 'not a url'
error: urlinfo: invalid URL
$ echo $?
1
```

No input:

```bash
$ vrk urlinfo
usage error: urlinfo: no URL provided
$ echo $?
2
```
