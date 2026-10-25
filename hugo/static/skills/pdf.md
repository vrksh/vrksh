# pdf - PDF text extractor - plain text or JSONL per page, --pages

When to use: get the text layer out of a PDF before counting, chunking or prompting. The argument is a file path; PDF bytes also work on stdin.
Composes with: tok, chunk, prompt, jq

| Flag      | Short | Type   | Description                                                             |
|-----------|-------|--------|-------------------------------------------------------------------------|
| `--pages` | `-p`  | string | Page range: `N`, `N-M`, or `N-` (1-indexed)                             |
| `--json`  | `-j`  | bool   | One `{"page":N,"text":"..."}` per page, then `{"_vrk":"pdf","count":N}` |
| `--quiet` | `-q`  | bool   | Suppress stderr                                                         |

Exit 0: success (including empty stdin)
Exit 1: file not found, not a PDF, encrypted, corrupt, page out of range, no extractable text (scanned)
Exit 2: invalid --pages, more than one file, unknown flag, interactive terminal with no input

Example:

    vrk pdf --json report.pdf | jq -r 'select(.page) | .text' | vrk tok

Anti-pattern:
- Don't pass PDF content as the argument. `vrk pdf "$text"` opens a file named after the text.
- Don't expect OCR or layout. Scanned PDFs exit 1, and text comes out in content-stream order, so columns and tables may interleave.
- Don't forget the trailer under --json. Use `select(.page)` to skip `{"_vrk":"pdf","count":N}`.
