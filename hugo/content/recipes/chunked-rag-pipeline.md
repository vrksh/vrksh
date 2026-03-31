---
title: "Chunked RAG pipeline"
meta_title: "Chunked RAG pipeline - vrk pipeline recipe"
description: "Keeps every chunk within the embedding model's token limit - no silent truncation during indexing. Split a document into chunks and store each for ..."
why: "Keeps every chunk within the embedding model's token limit - no silent truncation during indexing."
body: "Split a document into chunks and store each for retrieval."
slug: "chunked-rag-pipeline"
steps:
  - "DOC_ID=$(vrk uuid)"
  - "cat doc.txt | vrk chunk --size 512 --overlap 64 | while read -r chunk; do idx=$(echo \"$chunk\" | jq -r '.index'); text=$(echo \"$chunk\" | jq -r '.text'); vrk kv set \"doc:${DOC_ID}:chunk:${idx}\" \"$text\"; done"
tags:
  - "uuid"
  - "chunk"
  - "kv"
---
