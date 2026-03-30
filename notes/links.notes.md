## How it works

### JSONL output (default)

```bash
$ printf '# Getting Started\n\nVisit [our docs](https://docs.example.com) for the full guide.\nAlso check [GitHub](https://github.com/example/repo) and https://blog.example.com/updates.\n' | vrk links
{"text":"our docs","url":"https://docs.example.com","line":3}
{"text":"GitHub","url":"https://github.com/example/repo","line":4}
{"text":"https://blog.example.com/updates.","url":"https://blog.example.com/updates.","line":4}
```

Each record includes the link text, URL, and line number. Bare URLs use the URL itself as the text.

### URLs only (--bare)

```bash
$ printf '# Getting Started\n\nVisit [our docs](https://docs.example.com) for the full guide.\nAlso check [GitHub](https://github.com/example/repo) and https://blog.example.com/updates.\n' | vrk links --bare
https://docs.example.com
https://github.com/example/repo
https://blog.example.com/updates.
```

One URL per line, no JSON. Pipe directly to `xargs`, `while read`, or `vrk grab`.

### Metadata trailer (--json)

```bash
cat README.md | vrk links --json
```

Appends `{"_vrk":"links","count":N}` as the last record.

## What gets detected

- Markdown inline links: `[text](url)`
- Markdown reference links: `[text][ref]` with `[ref]: url`
- HTML anchor tags: `<a href="url">text</a>`
- Bare URLs: `https://example.com` in plain text

## Pipeline integration

### Find broken links in documentation

```bash
# Extract all links from a doc and check each one
cat docs/README.md | vrk links --bare | while IFS= read -r url; do
  STATUS=$(curl -sI -o /dev/null -w '%{http_code}' "$url")
  if [ "$STATUS" != "200" ]; then
    echo "Broken: $url ($STATUS)" | vrk emit --level warn --tag linkcheck
  fi
done
```

### Extract links from a web page

```bash
# Grab a page, extract all links, get unique domains
vrk grab https://example.com | vrk links --bare | \
  while IFS= read -r url; do
    vrk urlinfo --field host "$url"
  done | sort -u
```

### Index links in kv

```bash
# Store all links from a document with their text for later lookup
cat document.md | vrk links | while IFS= read -r record; do
  URL=$(echo "$record" | jq -r '.url')
  TEXT=$(echo "$record" | jq -r '.text')
  vrk kv set --ns links "$(echo "$URL" | vrk slug)" "$TEXT"
done
```

## When it fails

No input:

```bash
$ vrk links
usage error: links: no input: pipe text to stdin
$ echo $?
2
```

Empty input produces no output and exits 0 - there are simply no links to extract.
