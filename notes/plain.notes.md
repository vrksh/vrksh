## What gets stripped

`vrk plain` removes Markdown formatting from text and keeps the words, so a model reads fewer tokens.

| Markdown syntax | Plain text result |
|----------------|-------------------|
| `**bold**` | bold |
| `_italic_` | italic |
| `[link text](url)` | link text |
| `` `inline code` `` | inline code |
| Code fences | Content only, no fences |
| `# Headers` | Header text |
| `- bullet` | bullet |

plain drops the URL in a link and keeps the code content. It keeps the text that carries meaning and removes the formatting.

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
$ echo '**hello** world' | vrk plain --json
{"text":"hello world","input_bytes":16,"output_bytes":11}
```

The JSON envelope holds the plain text and the byte counts of the input and the output.

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

If the input has more than 1000 back-to-back nesting openers (`[a](`, `- `, `> `), plain does not parse it as Markdown, because the parser slows down quadratically on it. The input comes back as plain text with only whitespace cleanup. If any other input takes more than 10 seconds to parse, plain exits 1 with `plain: markdown too complex to parse within 10s`.

If you give no input on an interactive terminal, plain exits 2:

<!-- doccheck: skip (only fails on an interactive terminal, doccheck stdin is not a TTY) -->
```bash
$ vrk plain
usage error: plain: no input: pipe markdown to stdin
$ echo $?
2
```
