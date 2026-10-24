---
title: "vrk chunk"
description: "Split text into token-aware chunks that fit LLM context windows. JSONL output, never splits a character."
meta_lead: "vrk chunk splits text into token-counted pieces that fit within an LLM context window."
og_title: "vrk chunk - token-aware text splitter for LLM context windows"
tool: chunk
group: v1
mcp_callable: true
noindex: false
---

<!-- generated - do not edit below this line -->

vrk chunk splits text into token-counted pieces that fit within an LLM context window.

## The problem

A 38,000-token document needs to go through a model with an 8,192-token window. `split -l 100` divides by line count, which has no relationship to tokens. One chunk is 800 tokens, the next is 4,900. An obligation clause that spans a split boundary gets lost because neither chunk has the full sentence.

## The solution

`vrk chunk` splits text into token-counted pieces that fit within an LLM context window. Each chunk is emitted as a JSONL record with its index, text, and exact token count. Configurable overlap preserves context at boundaries so entities that span a split point appear in both adjacent chunks.

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

| Flag        | Short | Type   | Description                                                                                                   |
|-------------|-------|--------|---------------------------------------------------------------------------------------------------------------|
| `--size`    |       | int    | Max tokens per chunk (required)                                                                               |
| `--overlap` |       | int    | Token overlap between adjacent chunks                                                                         |
| `--by`      |       | string | Chunking strategy: paragraph                                                                                  |
| `--model`   | -m    | string | OpenAI model or encoding to count for: gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo, o200k_base (default), cl100k_base |
| `--quiet`   | -q    | bool   | Suppress stderr output                                                                                        |


<!-- notes - edit in notes/chunk.notes.md -->

## How the output works

Every record contains three fields:

- `index` - zero-based chunk number, sequential
- `text` - the chunk content as a string
- `tokens` - exact token count of `text`, the same number `vrk tok` gives for it (at most `--size`, see below for the one exception)

```bash
$ cat technical-spec.md | vrk chunk --size 30
{"index":0,"text":"# Title\n\nvrk is a set of small Unix tools for AI pipelines. Each tool reads stdin, writes stdout, and uses exit codes to report","tokens":30}
{"index":1,"text":" failure. The tools compose with pipes.\n\n## Details\n\nSee [the docs](https://vrk.sh/docs) for more.","tokens":26}
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
{"index":0,"text":"The vendor shall deliver all components within 30 calendar","tokens":10}
{"index":1,"text":" days of the signed agreement.","tokens":6}
```

With overlap, the boundary region appears in both chunks:

```bash
$ printf 'The vendor shall deliver all components within 30 calendar days of the signed agreement.' | vrk chunk --size 10 --overlap 4
{"index":0,"text":"The vendor shall deliver all components within 30 calendar","tokens":10}
{"index":1,"text":" within 30 calendar days of the signed agreement.","tokens":10}
```

Now "within 30 calendar" appears in both chunks, and the second chunk holds the complete obligation.

**Rule of thumb:** set `--overlap` to 5-10% of `--size`. For `--size 4000`, use `--overlap 200`.

### --by paragraph

Splits at paragraph breaks (double newlines) and never breaks a paragraph across chunks:

```bash
cat article.md | vrk chunk --size 4000 --by paragraph
```

Use `--by paragraph` when your content has natural paragraph structure and you want each chunk to contain complete paragraphs. Paragraphs packed into one chunk keep the blank line between them. If a single paragraph exceeds `--size`, it falls back to token-level splitting for that paragraph.

## Multi-byte text

One token can hold part of a character (Hindi, emoji, Greek, CJK). `chunk` never cuts a character in half: each chunk ends and starts on a character boundary, so every `text` is valid UTF-8. With `--overlap 0`, joining every `text` in order gives back the input exactly.

To stay on a boundary, a chunk ends a token or two early. In the rare case where one character spans more tokens than `--size` allows (for example `--size 1` on an emoji), the chunk grows to fit that character, up to 3 tokens over `--size`. The `tokens` field always reports the real count.

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
usage error: chunk: --size is required
$ echo $?
2
```

Overlap >= size:

```bash
$ cat document.txt | vrk chunk --size 100 --overlap 100
usage error: chunk: --overlap must be less than --size
$ echo $?
2
```
