# chunk - Token-aware text splitter - JSONL chunks within a token budget

When to use: split documents that exceed an LLM's context window into token-counted pieces. Each chunk is a JSONL record with index, text, and token count.
Composes with: grab, tok, prompt, kv, uuid

| Flag        | Short | Type   | Description                                           |
|-------------|-------|--------|-------------------------------------------------------|
| `--size`    |       | int    | Max tokens per chunk (required)                       |
| `--overlap` |       | int    | Token overlap between adjacent chunks (default: 0)    |
| `--by`      |       | string | Chunking strategy: `paragraph` (default: token-level) |

Exit 0: success (including empty input)
Exit 1: I/O error, tokenizer failure
Exit 2: --size missing or < 1, --overlap >= --size, unknown --by mode, unknown flag

Example:

    cat doc.txt | vrk chunk --size 1000 --overlap 100 | jq -r '.text'

Anti-pattern:
- Don't chunk then summarize without --overlap. Entities that span chunk boundaries get missed by both chunks. Use --overlap at 5-10% of --size.
- Don't use chunk for JSONL streams - it's for single documents. For JSONL record-by-record processing, just pipe directly.
