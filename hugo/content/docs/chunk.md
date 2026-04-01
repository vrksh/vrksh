---
title: "vrk chunk"
description: "Split text into token-aware chunks that fit LLM context windows. JSONL output, respects sentence boundaries."
og_title: "vrk chunk - token-aware text splitter for LLM context windows"
tool: chunk
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

## About

You have a 50-page contract and you need an LLM to extract every obligation. The document is 38,000 tokens. Your model's context window is 8,192. You split on line count - 100 lines per file. Some chunks are 800 tokens, some are 3,200. One chunk ends mid-sentence. The obligation "Vendor shall deliver... within 30 days" gets split across two chunks. Both chunks miss it.

`vrk chunk` splits long documents into token-aware pieces that fit within an LLM's context window. Each chunk is emitted as a JSONL record with its index, text, and exact token count. Splitting respects token boundaries and supports configurable overlap so context is preserved at the edges. No more guessing whether `split -l 100` maps to tokens.

## The problem

You need to process a long document through an LLM but it exceeds the context window. `split -l` splits by line count, which has no relationship to token count. `fold -w` splits by characters, which is worse. You write a Python script to split by tokens and it takes longer to debug than the actual task. Meanwhile, every entity that spans a split boundary is silently lost.

## Before and after

**Before**

```bash
split -l 100 document.txt chunk_
# chunk_aa: 312 tokens (wasted context space)
# chunk_ab: 4,891 tokens (exceeds your budget)
# chunk_ac: split mid-sentence, entity lost at boundary
```

**After**

```bash
cat document.txt | vrk chunk --size 4000 --overlap 200
```

## Example

```bash
cat contract.pdf.txt | vrk chunk --size 4000 --overlap 200
```

## Exit codes

| Code | Meaning                                                            |
|------|--------------------------------------------------------------------|
| 0    | Success, including empty input                                     |
| 1    | I/O error                                                          |
| 2    | No input, --size missing or < 1, --overlap >= --size, unknown flag |

## Flags

| Flag        | Short | Type   | Description                           |
|-------------|-------|--------|---------------------------------------|
| `--size`    |       | int    | Max tokens per chunk (required)       |
| `--overlap` |       | int    | Token overlap between adjacent chunks |
| `--by`      |       | string | Chunking strategy: paragraph          |
| `--quiet`   | -q    | bool   | Suppress stderr output                |


<!-- notes - edit in notes/chunk.notes.md -->

## How the output works

Every record contains three fields:

- `index` - zero-based chunk number, sequential
- `text` - the chunk content as a string
- `tokens` - exact token count for this chunk (always <= `--size`)

```bash
$ cat technical-spec.md | vrk chunk --size 30
{"index":0,"text":"Section one covers the system architecture. The platform uses a microservices pattern with...","tokens":30}
{"index":1,"text":"event-driven communication between services. Each service maintains its own database...","tokens":28}
```

## Flag details

### --size (required)

Sets the maximum tokens per chunk. Choose a value that leaves room for your system prompt:

```bash
# Model has 8,192-token context. System prompt is ~1,000 tokens.
# Leave 7,000 for content chunks.
cat document.txt | vrk chunk --size 7000
```

### --overlap

Repeats tokens from the end of each chunk at the start of the next one. This is how you prevent entities from being lost at boundaries.

Without overlap, a sentence that spans a chunk boundary gets split:

```bash
$ printf 'The vendor shall deliver all components within 30 calendar days of the signed agreement.' | vrk chunk --size 10
{"index":0,"text":"The vendor shall deliver all components within 30 calendar days","tokens":10}
{"index":1,"text":" of the signed agreement.","tokens":5}
```

With overlap, the boundary region appears in both chunks:

```bash
$ printf 'The vendor shall deliver all components within 30 calendar days of the signed agreement.' | vrk chunk --size 10 --overlap 4
{"index":0,"text":"The vendor shall deliver all components within 30 calendar days","tokens":10}
{"index":1,"text":" within 30 calendar days of the signed agreement.","tokens":9}
```

Now "within 30 calendar days" appears in both chunks. An LLM processing either chunk sees the complete obligation.

**Rule of thumb:** set `--overlap` to 5-10% of `--size`. For `--size 4000`, use `--overlap 200`.

### --by paragraph

Splits at paragraph breaks (double newlines) and never breaks a paragraph across chunks:

```bash
cat article.md | vrk chunk --size 4000 --by paragraph
```

Use `--by paragraph` when your content has natural paragraph structure and you want each chunk to contain complete paragraphs. If a single paragraph exceeds `--size`, it falls back to token-level splitting for that paragraph.

## Processing chunks through an LLM

Use `vrk prompt --field text` to process each chunk. One API call per record, no loop needed:

```bash
cat long-document.md | vrk chunk --size 4000 --overlap 200 | \
  vrk prompt --field text --system 'Extract all named entities as a JSON array' --json
```

The `--field text` flag reads each JSONL line, extracts the `text` field, and sends it as the prompt. With `--json`, the output merges input fields (index, tokens) with response metadata.

> **Advanced: shell loop pattern**
>
> If you need per-record shell logic beyond what `--field` provides (e.g. conditional processing, writing to different files), use a `while` loop:
>
> ```bash
> cat long-document.md | vrk chunk --size 4000 --overlap 200 | \
>   while IFS= read -r record; do
>     echo "$record" | jq -r '.text' | \
>       vrk prompt --system 'Extract all named entities as a JSON array'
>   done
> ```
>
> `IFS=` prevents word splitting. `-r` prevents backslash interpretation. Prefer `--field` for straightforward pipelines.

## Pipeline integration

### Extract entities from a large document

```bash
# Convert a PDF to text, chunk it, extract entities from each piece
pdftotext contract.pdf - | \
  vrk chunk --size 4000 --overlap 200 | \
  vrk prompt --field text \
    --schema '{"entities":"array","summary":"string"}' \
    --retry 2 \
    --system 'Extract named entities and a one-sentence summary' \
    --json | \
  vrk validate --schema '{"entities":"array","summary":"string"}' --strict
```

### Measure before chunking

```bash
# Only chunk if the document actually exceeds the budget
TOKENS=$(cat report.md | vrk tok --json | jq -r '.tokens')
if [ "$TOKENS" -le 8000 ]; then
  cat report.md | vrk prompt --system 'Summarize this report'
else
  cat report.md | vrk chunk --size 4000 --overlap 200 | \
    vrk prompt --field text --system 'Summarize this section'
fi
```

### Web page to chunked summaries

```bash
# Grab a long article, chunk it, summarize each section
vrk grab https://example.com/long-article | \
  vrk chunk --size 4000 --by paragraph | \
  vrk prompt --field text --system 'Summarize this section in one paragraph'
```

## When it fails

Missing `--size`:

```bash
$ cat document.txt | vrk chunk
usage error: chunk: --size is required (>= 1)
$ echo $?
2
```

Overlap >= size:

```bash
$ cat document.txt | vrk chunk --size 100 --overlap 100
usage error: chunk: --overlap (100) must be less than --size (100)
$ echo $?
2
```
