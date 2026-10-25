---
title: "vrk pdf"
description: "Extract the text layer from a PDF. Markdown or plain text with form feeds between pages, or one JSONL record per page. No OCR, no Python."
meta_lead: "vrk pdf reads a PDF from a file path or stdin and writes its text layer to stdout, pages separated by form feeds or as one JSONL record per page."
og_title: "vrk pdf - extract text from PDFs for AI pipelines"
tool: pdf
group: v1
mcp_callable: false
noindex: false
---

<!-- generated - do not edit below this line -->

vrk pdf reads a PDF from a file path or stdin and writes its text layer to stdout, pages separated by form feeds or as one JSONL record per page.

## The problem

Getting text out of a PDF usually means installing poppler for `pdftotext` or a Python package with its own dependencies. Neither tells you which page the text came from in a form a pipeline can use, and a scanned PDF quietly produces an empty string that goes straight to the LLM.

## The solution

`vrk pdf` extracts the text layer of a PDF. Markdown by default (headings from font size and weight, bullet lists, tables, paragraphs; running headers, page numbers and line-end hyphens cleaned up), `--text` for plain text, with a form feed (`\f`) between pages. `--json` emits one `{"page":N,"text":"..."}` record per page and a `{"_vrk":"pdf","count":N}` trailer. `--pages` selects pages like `3`, `2-5`, `4-` or `1,3,5-7`. Scanned PDFs with no text layer exit 1 so a pipeline stops before sending empty text to a model.

## Before and after

**Before**

```bash
pip install pypdf
python -c 'import sys, pypdf; print("".join(p.extract_text() for p in pypdf.PdfReader(sys.argv[1]).pages))' report.pdf
# no page numbers, scanned PDFs print nothing and exit 0
```

**After**

```bash
vrk pdf --json report.pdf
```

## Example

```bash
vrk pdf --pages 1-3 report.pdf | vrk tok
```

## Exit codes

| Code | Meaning                                                                                   |
|------|-------------------------------------------------------------------------------------------|
| 0    | Success, including empty stdin                                                            |
| 1    | File not found, not a PDF, encrypted, corrupt, page out of range, or no extractable text  |
| 2    | Invalid --pages range, more than one file, unknown flag, or interactive TTY with no input |

## Flags

| Flag           | Short | Type   | Description                                                                    |
|----------------|-------|--------|--------------------------------------------------------------------------------|
| `--text`       | -t    | bool   | Plain text instead of Markdown                                                 |
| `--pages`      | -p    | string | Pages: N, N-M, N- or a comma list like 1,3,5-7 (1-indexed)                     |
| `--json`       | -j    | bool   | One record per page, then a metadata trailer                                   |
| `--as-printed` |       | bool   | Keep running headers, footers, page numbers and end-of-line hyphens as printed |
| `--quiet`      | -q    | bool   | Suppress stderr output                                                         |


<!-- notes - edit in notes/pdf.notes.md -->

## How it works

### Markdown (default)

```bash
$ vrk pdf --pages 1 report.pdf
Quarterly report, page one.
```

Output is Markdown. Lines set noticeably bigger than the body text, and followed by smaller text, become headings: the largest size is `#`, the next `##`, the rest `###`. A short bold line at body size, like `3.2.1 Scaled Attention`, followed by a full line of plain text, is a heading one level below those. Bullet glyphs like `•` become `- ` list items, and a bigger vertical gap starts a new paragraph. Ligatures like `ﬁ` come out as plain `fi`.

Rows whose cells are split by wide gaps and line up with the row above become a Markdown table, first row as header. It's a guess from spacing: a merged cell splits a table in two, and aligned code comments can come out as a table.

### Cleanup

Running headers, footers and page numbers are dropped: a line near the top or bottom of a page that repeats on three or more of the pages you extract, ignoring digits, keeps only its first copy, and bare page numbers go everywhere. Only extracted pages count, so `--pages 5` keeps that page's header.

A word split across lines, like `trans-` and `former`, is joined back into `transformer`. The hyphen stays when the document spells the compound with one elsewhere, as in `self-attention`.

`--as-printed` turns off both: headers, footers, page numbers and hyphenated line breaks come through as they are on the page. Use it when you need the printed page numbers, say for citations, or when a real line got dropped.

Most two-column papers already read in order. When a page is written row by row across two columns and no text crosses the gutter, the left column comes out first.

Without `--pages` you get every page, with a form feed (`\f`) on its own line between pages. A PDF with three pages has two form feeds, even if a page is empty.

```bash
$ vrk pdf report.pdf | tr '\f' '-'
Quarterly report, page one.

-
Revenue grew in every region.

-
Outlook for next year.
```

### Plain text (--text)

`--text` drops the Markdown: no `#`, no list markers, and the form feed comes straight after each page.

```bash
$ vrk pdf --text report.pdf | tr '\f' '-'
Quarterly report, page one.
-Revenue grew in every region.
-Outlook for next year.
```

### One record per page (--json)

```bash
$ vrk pdf --json report.pdf
{"page":1,"text":"Quarterly report, page one."}
{"page":2,"text":"Revenue grew in every region."}
{"page":3,"text":"Outlook for next year."}
{"_vrk":"pdf","count":3}
```

Each `text` is Markdown unless you add `--text`. Page numbers are the real ones, so `--json --pages 2-3` gives records for pages 2 and 3. The last record is the trailer; use `jq 'select(.page)'` to skip it.

### Page ranges (--pages)

`--pages 3` is one page, `--pages 2-5` is a range, and `--pages 4-` runs to the last page. Pages are 1-indexed.

A comma list mixes them: `--pages 1,3,5-7`. Pages come out in the order you list them, and a page listed twice comes out once.

```bash
$ vrk pdf --text --pages 2- report.pdf | tr '\f' '-'
Revenue grew in every region.
-Outlook for next year.
```

```bash
$ vrk pdf --text --pages 3,1 report.pdf | tr '\f' '-'
Outlook for next year.
-Quarterly report, page one.
```

## Pipeline integration

### Count tokens before sending a PDF to a model

```bash
$ vrk pdf report.pdf | vrk tok
23
```

### Per-page text for RAG

```bash
$ vrk pdf --json report.pdf | jq -r 'select(.page) | "\(.page): \(.text)"'
1: Quarterly report, page one.
2: Revenue grew in every region.
3: Outlook for next year.
```

### Chunk a long PDF

```bash
vrk pdf manual.pdf | vrk chunk --size 1000 | vrk prompt --field text --system 'Summarise this section.'
```

## When it fails

A scanned PDF has pictures of pages and no text layer. `vrk pdf` exits 1 instead of printing nothing, so the pipeline stops before an empty prompt reaches the model:

```bash
$ vrk pdf scan.pdf
error: pdf: no extractable text found; document may be scanned (OCR required)
$ echo $?
1
```

A page the reader chokes on doesn't sink the file. It warns on stderr and keeps whatever text it read before the error; the other pages come out as usual. With `--json` that page's record gets an `"error"` field. Only when no page has any text does it exit 1.

A page past the end:

```bash
$ vrk pdf --pages 9 report.pdf
error: pdf: page 9 does not exist (PDF has 3 pages)
```

A malformed range is a usage error:

```bash
$ vrk pdf --pages 5-2 report.pdf
usage error: pdf: invalid page range: 5-2
$ echo $?
2
```

The argument is a file path, not PDF content. A missing file exits 1:

```bash
$ vrk pdf missing.pdf
error: pdf: file not found: missing.pdf
```
