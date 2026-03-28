# chunk - Token-aware text splitter - JSONL chunks within a token budget

When to use: split large documents into token-bounded chunks for RAG or batched LLM calls.
Composes with: grab, tok, prompt, kv, uuid

| Flag | Short | Type | Description |
|------|-------|------|-------------|
| `--size` | | int | Max tokens per chunk (required) |
| `--overlap` | | int | Token overlap between adjacent chunks (default: 0) |
| `--by` | | string | Chunking strategy: `paragraph` (default: token-level) |

Exit 0: success (including empty input)
Exit 1: I/O error, tokenizer failure
Exit 2: --size missing or < 1, --overlap >= --size, unknown --by mode, unknown flag

Example:

    cat doc.txt | vrk chunk --size 1000 --overlap 100 | jq -r '.text'

Anti-pattern:
- Don't assume chunk boundaries align with word boundaries -- default mode splits at token IDs, which can fall mid-word.
