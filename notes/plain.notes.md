## What gets stripped

| Markdown syntax | Plain text result |
|----------------|-------------------|
| `**bold**` | bold |
| `_italic_` | italic |
| `[link text](url)` | link text |
| `` `inline code` `` | inline code |
| Code fences | Content only, no fences |
| `# Headers` | Header text |
| `- bullet` | bullet |

URLs in links are dropped. Code content is kept. Everything that carries meaning stays; everything that's just formatting goes.

## How it works

```bash
$ printf '**Bold text** and _italic_ and [link](http://x.com)\n\n- bullet one\n- bullet two\n\n```python\nprint("hello")\n```\n' | vrk plain
Bold text and italic and link

bullet one
bullet two

print("hello")
```

### JSON output

```bash
echo '**hello** world' | vrk plain --json
```

Wraps the plain text in a JSON envelope with byte counts.

## Pipeline integration

### Save tokens before an LLM call

```bash
# Strip markdown formatting to reduce token count, then summarize
vrk grab https://example.com/docs | vrk plain | vrk tok --check 8000 | \
  vrk prompt --system 'Summarize this document'
```

### Compare token counts before and after

```bash
RAW=$(vrk grab https://example.com/docs | vrk tok --json | jq -r '.tokens')
PLAIN=$(vrk grab https://example.com/docs | vrk plain | vrk tok --json | jq -r '.tokens')
echo "Markdown: $RAW tokens, Plain: $PLAIN tokens, Saved: $((RAW - PLAIN))"
```

### Batch processing markdown files

```bash
# Strip formatting from all docs before chunking and summarizing
for f in docs/*.md; do
  cat "$f" | vrk plain | vrk chunk --size 4000 | \
    while IFS= read -r record; do
      echo "$record" | jq -r '.text' | vrk prompt --system 'Summarize'
    done
done
```

## When it fails

No input:

```bash
$ vrk plain
usage error: plain: no input: pipe text to stdin
$ echo $?
2
```
