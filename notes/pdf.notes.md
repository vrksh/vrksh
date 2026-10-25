## How it works

`vrk pdf` extracts the text of a PDF file and prints it as Markdown, plain text or one JSONL record per page.

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

Text a viewer never shows is left out: glyphs drawn with render mode 3 or 7 (`Tr`), the usual way to hide instructions for a model in a document that looks harmless, and anything outside the page box. Control characters are dropped too, so a PDF can't send terminal escapes or fake a page break with a form feed of its own.

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

Hostile files stop at a budget instead of hanging the pipeline. A page stops after 1,000,000 glyphs or 10,000 Form XObjects, and the whole document after 20,000,000 glyphs; the page that hits a budget warns and keeps what it read. Input is capped at 512 MiB, and the path must be a regular file, so `/dev/zero` or a FIFO exits 1 instead of reading forever.

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
