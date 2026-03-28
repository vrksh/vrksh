---
title: "vrk chunk"
description: "Token-aware text splitter - JSONL chunks within a token budget."
tool: chunk
group: core
mcp_callable: true
noindex: false
---

## The problem

A document is too long to fit in a single LLM context window. You could
truncate it -- but you lose content. You could chunk it manually -- but now
you are writing Python to split on sentence boundaries, count tokens, and
reassemble later. Splitting by character count is wrong because token boundaries
do not align with character boundaries. There is no Unix tool that does this
cleanly with a known token budget per chunk.

## The fix

```bash
$ cat long-doc.md | vrk chunk --size 4000
{"index":0,"text":"...first chunk...","tokens":3847}
{"index":1,"text":"...second chunk...","tokens":4000}
{"index":2,"text":"...last chunk...","tokens":1293}
```

One JSONL record per chunk. Each record has the chunk index, the text, and the
exact token count. `--size` is required -- there is no default because silently
guessing a budget leads to silent token overflows downstream.

The last chunk will have fewer tokens than `--size` if the input does not
divide evenly. Empty input produces no output and exits 0.

## Processing each chunk

The most common pattern: send each chunk to a model independently.

```bash
cat long-doc.md | vrk chunk --size 4000 \
  | while IFS= read -r chunk; do
      echo "$chunk" | jq -r '.text' | vrk prompt \
        --system "Summarize this section."
    done
```

Line by line: `vrk chunk` splits the document into JSONL records. The `while`
loop reads one record at a time. `jq -r '.text'` extracts the plain text from
each JSON record. That text is piped to `vrk prompt` as the user message.

## Overlap for context continuity

```bash
$ cat long-doc.md | vrk chunk --size 4000 --overlap 200
```

The last 200 tokens of each chunk are repeated at the start of the next. This
prevents losing context at chunk boundaries. The `tokens` field in each record
reflects the actual token count including any overlap tokens.

When to use overlap:

- **Summarization** -- usually do not need it. Each chunk stands alone.
- **Entity extraction** -- use it. Entities span sentences, and a split at the
  wrong point means you miss one.
- **Q&A over documents** -- use it. The answer to a question might straddle
  two chunks.

`--overlap` must be less than `--size`. If they are equal or overlap is larger,
the split would never advance:

```bash
$ cat doc.txt | vrk chunk --size 100 --overlap 100
usage error: chunk: --overlap must be less than --size
```

## Paragraph-aware chunking

```bash
$ cat essay.txt | vrk chunk --size 1500 --by paragraph
```

`--by paragraph` splits at double-newline boundaries first, then greedily packs
paragraphs into chunks that fit within `--size`. This preserves natural text
boundaries -- a sentence that starts in one chunk will not be split mid-word
into the next. A paragraph that is itself larger than `--size` falls back to
token-level splitting automatically.

## JSON output fields

Each line of output is a JSON object with three fields:

```json
{"index":0,"text":"The quick brown fox...","tokens":30}
```

| Field | Type | Description |
|-------|------|-------------|
| `index` | int | Zero-based chunk number, sequential |
| `text` | string | The chunk content |
| `tokens` | int | Exact token count of this chunk's text |

## Pipeline example

Chunk a fetched article, summarize each section, collect results:

```bash
vrk grab --text https://example.com/long-article \
  | vrk chunk --size 2000 --overlap 200 --by paragraph \
  | while IFS= read -r chunk; do
      echo "$chunk" | jq -r '.text' \
        | vrk prompt --system "Extract the key claim from this passage."
    done
```

Verify no chunk exceeds a token budget before sending to a model:

```bash
cat document.txt \
  | vrk chunk --size 3000 \
  | while IFS= read -r chunk; do
      echo "$chunk" | jq -r '.text' | vrk tok --budget 3000
    done
```

## Flags

| Flag | Short | Type | Default | Description |
|------|-------|------|---------|-------------|
| `--size` | | int | (required) | Max tokens per chunk, must be >= 1 |
| `--overlap` | | int | `0` | Token overlap between adjacent chunks, must be < `--size` |
| `--by` | | string | `""` | Chunking strategy: `paragraph` (split at double-newlines, pack greedily) |
| `--quiet` | `-q` | bool | `false` | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success, including empty input |
| 1 | I/O error |
| 2 | No input, `--size` missing or < 1, `--overlap` >= `--size`, unknown flag |
