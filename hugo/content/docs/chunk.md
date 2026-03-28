---
title: "vrk chunk"
description: "Token-aware text splitter - JSONL chunks within a token budget."
tool: chunk
group: core
mcp_callable: true
noindex: false
---

## The problem

LLMs have context windows. A document that fits comfortably in your terminal might be 50,000 tokens - ten times what a model can see at once. Splitting by character count is wrong because token boundaries do not align with character boundaries. Splitting by word count is closer but still inaccurate. Most ad-hoc chunking libraries count tokens incorrectly or do not support overlap, which means adjacent chunks have no shared context and coherence breaks at every boundary.

## The fix

```bash
cat article.txt | vrk chunk --size 2000
```

Each chunk is a JSONL record with its index, text, and exact token count:

```
{"index":0,"text":"...","tokens":1987}
{"index":1,"text":"...","tokens":2000}
{"index":2,"text":"...","tokens":431}
```

`--size` is required. There is no default because silently guessing a budget leads to silent token overflows downstream.

## Walkthrough

**Happy path** - split a document into 2000-token chunks and count how many you get:

```bash
cat long_report.txt | vrk chunk --size 2000 | wc -l
```

Each `index` field is zero-based and sequential. The last chunk will have fewer tokens than `--size` if the input does not divide evenly.

**Empty input** - an empty pipe produces no output and exits 0. This is intentional: if nothing went in, nothing should come out.

```bash
printf '' | vrk chunk --size 1000
# (no output, exit 0)
```

**Failure cases** - `--size` is required; omitting it is a usage error:

```bash
cat article.txt | vrk chunk
# usage error: chunk: --size is required
```

`--overlap` must be less than `--size`. If they are equal or overlap is larger, the split would never advance:

```bash
cat article.txt | vrk chunk --size 100 --overlap 100
# usage error: chunk: --overlap must be less than --size
```

**Overlap** - `--overlap` prepends the last N tokens of the previous chunk to the start of the next. This gives the model context about where the chunk came from:

```bash
cat article.txt | vrk chunk --size 2000 --overlap 200
```

The `tokens` field in each record reflects the actual token count of that chunk's text, including any prepended overlap tokens.

**Paragraph-aware chunking** - `--by paragraph` splits at double-newline boundaries first, then greedily packs paragraphs into chunks that fit within `--size`. A paragraph that is itself larger than `--size` falls back to token-level splitting automatically.

This preserves natural text boundaries: a sentence that starts one chunk will not be split mid-word into the next.

```bash
cat essay.txt | vrk chunk --size 1500 --by paragraph
```

**Positional argument** - like every vrk tool, chunk accepts input as a positional arg:

```bash
vrk chunk --size 500 "This is the text to split into chunks."
```

**Combining with prompt** - the most common use. Each chunk goes to the model independently; results can be aggregated:

```bash
cat long_doc.txt | vrk chunk --size 2000 --overlap 100 | \
  while IFS= read -r chunk; do
    echo "$chunk" | jq -r '.text' | vrk prompt --system "Summarize this section in one sentence."
  done
```

## Pipeline example

Chunk a fetched article, send each chunk to a model, collect summaries, then store the final one:

```bash
vrk grab --text https://example.com/long-article \
  | vrk chunk --size 2000 --overlap 200 --by paragraph \
  | while IFS= read -r chunk; do
      echo "$chunk" | jq -r '.text' | vrk prompt --system "Extract the key claim from this passage."
    done \
  | vrk kv set --ns research article_claims
```

Check that no chunk exceeds a token budget before sending to a model:

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
| `--size` | | int | (required) | Max tokens per chunk. Must be >= 1. No default - always specify explicitly. |
| `--overlap` | | int | 0 | Token overlap between adjacent chunks. Must be less than `--size`. |
| `--by` | | string | `""` | Chunking strategy. Supported: `paragraph` (split at double-newlines, then pack greedily). Default is fixed token-window. |
| `--quiet` | `-q` | bool | false | Suppress stderr output |

## Exit codes

| Exit | Meaning |
|------|---------|
| 0 | Success, including empty input (no chunks emitted) |
| 1 | Runtime error - tokenizer failure, I/O error |
| 2 | Usage error - `--size` missing or < 1, `--overlap` >= `--size`, unknown `--by` mode, interactive TTY with no input |
