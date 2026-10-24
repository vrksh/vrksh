## How it works

### Plain text (default)

```bash
$ vrk pdf --pages 1 report.pdf
Quarterly report, page one.
```

Without `--pages` you get every page, with a form feed (`\f`) between pages. A PDF with three pages has two form feeds, even if a page is empty.

```bash
$ vrk pdf report.pdf | tr '\f' '-'
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

Page numbers are the real ones, so `--json --pages 2-3` gives records for pages 2 and 3. The last record is the trailer; use `jq 'select(.page)'` to skip it.

### Page ranges (--pages)

`--pages 3` is one page, `--pages 2-5` is a range, and `--pages 4-` runs to the last page. Pages are 1-indexed.

```bash
$ vrk pdf --pages 2- report.pdf | tr '\f' '-'
Revenue grew in every region.
-Outlook for next year.
```

## Pipeline integration

### Count tokens before sending a PDF to a model

```bash
$ vrk pdf report.pdf | vrk tok
21
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
