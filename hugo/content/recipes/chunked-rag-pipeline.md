---
title: "Chunked RAG pipeline"
meta_title: "Chunked RAG pipeline - vrk pipeline recipe"
description: "Keeps every chunk within the embedding model's token limit - no silent truncation during indexing. Split a document into chunks and store each for ..."
why: "Keeps every chunk within the embedding model's token limit - no silent truncation during indexing."
body: "Split a document into chunks and store each for retrieval."
slug: "chunked-rag-pipeline"
steps:
  - "DOC_ID=$(vrk uuid)"
  - |-
    cat doc.txt \
      | vrk chunk --size 512 --overlap 64 \
      | while read -r chunk; do
          idx=$(echo "$chunk" | jq -r '.index')
          text=$(echo "$chunk" | jq -r '.text')
          vrk kv set "doc:${DOC_ID}:chunk:${idx}" "$text"
        done
tags:
  - "uuid"
  - "chunk"
  - "kv"
---

## The problem

RAG pipelines need to split documents into chunks that fit the embedding model's context window. Too large and you get truncation. Too small and you lose context. And every chunk needs a stable ID so you can retrieve it later.

Most chunking libraries are Python-only and split on character count, not token count. A 500-character chunk might be 80 tokens or 200 tokens depending on the content. Token-aware chunking gives you predictable sizes.

## How the pipeline works

`vrk uuid` generates a unique document ID. `vrk chunk --size 512 --overlap 64` splits the input into JSONL records, each containing at most 512 tokens with a 64-token overlap between adjacent chunks. The overlap preserves context across chunk boundaries.

Each JSONL record has an `index` and `text` field. The loop reads each record, extracts the fields, and stores the chunk in `vrk kv` with a composite key: `doc:{id}:chunk:{index}`.

## The chunk output

```json
{"index":0,"text":"First section of the document...","tokens":487}
{"index":1,"text":"...overlap text. Second section...","tokens":502}
{"index":2,"text":"...overlap text. Final section...","tokens":341}
```

The `--overlap` flag controls how many tokens from the end of one chunk appear at the start of the next. This matters for retrieval quality - without overlap, a query that spans a chunk boundary might miss relevant content.

