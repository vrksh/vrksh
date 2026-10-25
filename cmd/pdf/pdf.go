// Package pdf implements vrk pdf - a PDF text extractor.
// Reads a PDF from a file path or stdin and writes the text layer to stdout as
// Markdown (or plain text with --text): pages separated by form feeds, or one
// JSONL record per page with --json.
//
// Text layer only. No OCR, so scanned PDFs exit 1. Text comes out in
// content-stream order, with spaces and line breaks rebuilt from glyph
// positions and headings guessed from font size. The underlying library (internal/pdfread, vendored from github.com/ledongthuc/pdf)
// reads PDF 1.0-1.7; 2.0 files exit 1.
package pdf

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/spf13/pflag"
	lpdf "github.com/vrksh/vrksh/internal/pdfread"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// readAll is a var so tests can inject I/O errors.
var readAll = io.ReadAll

// maxTreeDepth caps how deep the page-tree walk goes. Real trees are a few
// levels deep; a cycle in /Kids would otherwise loop forever.
const maxTreeDepth = 64

// pageRange is 1-indexed and inclusive. end == 0 means "to the last page".
type pageRange struct{ start, end int }

type pageRecord struct {
	Page  int    `json:"page"`
	Text  string `json:"text"`
	Error string `json:"error,omitempty"`
}

type metaRecord struct {
	VRK   string `json:"_vrk"`
	Count int    `json:"count"`
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "pdf",
		Short: "PDF text extractor - Markdown, --text, JSONL per page",
		Flags: []shared.FlagMeta{
			{Name: "pages", Shorthand: "p", Usage: "pages: N, N-M, N- or a comma list like 1,3,5-7 (1-indexed)"},
			{Name: "text", Shorthand: "t", Usage: "plain text instead of Markdown"},
			{Name: "json", Shorthand: "j", Usage: `one {"page":N,"text":"..."} per page, then {"_vrk":"pdf","count":N}`},
			{Name: "as-printed", Usage: "keep running headers, footers, page numbers and end-of-line hyphens as printed"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk pdf. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("pdf", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var pagesFlag string
	var textFlag, jsonFlag, asPrinted, quietFlag bool
	fs.StringVarP(&pagesFlag, "pages", "p", "", "pages: N, N-M, N- or a comma list like 1,3,5-7 (1-indexed)")
	fs.BoolVarP(&textFlag, "text", "t", false, "plain text instead of Markdown")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `one {"page":N,"text":"..."} per page, then {"_vrk":"pdf","count":N}`)
	fs.BoolVar(&asPrinted, "as-printed", false, "keep running headers, footers, page numbers and end-of-line hyphens as printed")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) - callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	fail := func(code int, format string, args ...any) int {
		msg := fmt.Sprintf(format, args...)
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": msg, "code": code})
		}
		if code == 2 {
			return shared.UsageErrorf("%s", msg)
		}
		return shared.Errorf("%s", msg)
	}

	ranges := []pageRange{{start: 1}}
	if fs.Changed("pages") {
		var ok bool
		if ranges, ok = parsePages(pagesFlag); !ok {
			return fail(2, "pdf: invalid page range: %s", pagesFlag)
		}
	}

	args := fs.Args()
	if len(args) > 1 {
		return fail(2, "pdf: expected one file path, got %d arguments", len(args))
	}

	// ponytail: whole PDF is held in memory (the library needs random access);
	// switch file paths to os.Open + ReaderAt if multi-GB PDFs show up.
	var data []byte
	if len(args) == 1 {
		var err error
		data, err = os.ReadFile(args[0])
		if errors.Is(err, os.ErrNotExist) {
			return fail(1, "pdf: file not found: %s", args[0])
		}
		if err != nil {
			return fail(1, "pdf: reading %s: %v", args[0], err)
		}
	} else {
		// TTY guard: interactive terminal with no piped input -> usage error.
		if isTerminal(int(os.Stdin.Fd())) {
			return fail(2, "pdf: no input: provide a file path or pipe a PDF to stdin")
		}
		var err error
		data, err = readAll(os.Stdin)
		if err != nil {
			return fail(1, "pdf: reading stdin: %v", err)
		}
		// Empty stdin is valid input with nothing in it.
		if len(data) == 0 {
			return 0
		}
	}

	if bytes.HasPrefix(data, []byte("%PDF-2.")) {
		return fail(1, "pdf: PDF 2.0 is not supported (PDF 1.0 to 1.7 only)")
	}
	all, broken, err := open(data)
	if err != nil {
		return fail(1, "%s", errMessage(err))
	}
	n := len(all)
	if n == 0 {
		return fail(1, "pdf: corrupt PDF: page tree has no readable pages")
	}
	if broken > 0 && !jsonFlag {
		shared.Warn("pdf: page tree: skipped %d unreadable entries", broken)
	}

	pages, bad := selectPages(ranges, n)
	if bad > 0 {
		return fail(1, "pdf: page %d does not exist (PDF has %d pages)", bad, n)
	}

	lines, pageErrs := extract(all, pages, !asPrinted)

	texts := make([]string, len(lines))
	var h headings
	if !textFlag {
		h = findHeadings(lines)
	}
	hasText := false
	for i, ls := range lines {
		if textFlag {
			texts[i] = plainText(ls)
		} else {
			texts[i] = markdown(ls, h)
		}
		hasText = hasText || strings.TrimSpace(texts[i]) != ""
	}
	if !hasText {
		// A page that failed to parse says more than "no text".
		for i, e := range pageErrs {
			if e != nil {
				return fail(1, "pdf: page %d: %s", pages[i], short(e.Error()))
			}
		}
		if len(pages) < n {
			return fail(1, "pdf: no extractable text on the selected pages")
		}
		return fail(1, "pdf: no extractable text found; document may be scanned (OCR required)")
	}

	w := bufio.NewWriter(os.Stdout)
	if jsonFlag {
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		for i, t := range texts {
			rec := pageRecord{Page: pages[i], Text: t}
			if pageErrs[i] != nil {
				rec.Error = "pdf: page " + strconv.Itoa(pages[i]) + ": " + short(pageErrs[i].Error())
			}
			if err := enc.Encode(rec); err != nil {
				return fail(1, "pdf: writing output: %v", err)
			}
		}
		if err := enc.Encode(metaRecord{VRK: "pdf", Count: len(texts)}); err != nil {
			return fail(1, "pdf: writing output: %v", err)
		}
	} else {
		for i, e := range pageErrs {
			if e != nil {
				shared.Warn("pdf: page %d: %s", pages[i], short(e.Error()))
			}
		}
		// Markdown pages keep blank lines around the form feed so a heading
		// at the top of a page still parses.
		sep := "\n\n\f\n"
		if textFlag {
			sep = "\f"
			for i, t := range texts {
				if t != "" {
					texts[i] = t + "\n"
				}
			}
		}
		out := strings.Join(texts, sep)
		if !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		if _, err := w.WriteString(out); err != nil {
			return shared.Errorf("pdf: writing output: %v", err)
		}
	}
	if err := w.Flush(); err != nil {
		return shared.Errorf("pdf: writing output: %v", err)
	}
	return 0
}

// parsePages accepts a comma-separated list of "N", "N-M" and "N-", with
// N >= 1 and M >= N.
func parsePages(s string) ([]pageRange, bool) {
	var out []pageRange
	for _, part := range strings.Split(s, ",") {
		pr, ok := parseRange(part)
		if !ok {
			return nil, false
		}
		out = append(out, pr)
	}
	return out, true
}

func parseRange(s string) (pageRange, bool) {
	s = strings.TrimSpace(s)
	startStr, endStr, hasDash := strings.Cut(s, "-")
	start, ok := pageNum(startStr)
	if !ok {
		return pageRange{}, false
	}
	if !hasDash {
		return pageRange{start, start}, true
	}
	if endStr == "" {
		return pageRange{start, 0}, true
	}
	end, ok := pageNum(endStr)
	if !ok || end < start {
		return pageRange{}, false
	}
	return pageRange{start, end}, true
}

// pageNum parses a digits-only page number >= 1. strconv.Atoi alone would
// accept "+3" and "-3".
func pageNum(s string) (int, bool) {
	if s == "" || strings.Trim(s, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.Atoi(s)
	return n, err == nil && n >= 1
}

// selectPages expands ranges against an n-page PDF into page numbers, in the
// order given and without repeats. bad is the first page past the end, or 0.
func selectPages(ranges []pageRange, n int) (pages []int, bad int) {
	seen := map[int]bool{}
	for _, pr := range ranges {
		if pr.end == 0 {
			pr.end = n
		}
		for _, p := range []int{pr.start, pr.end} {
			if p > n {
				return nil, p
			}
		}
		for p := pr.start; p <= pr.end; p++ {
			if !seen[p] {
				seen[p] = true
				pages = append(pages, p)
			}
		}
	}
	return pages, 0
}

// open parses a PDF held in memory and returns its pages, from one walk of
// the page tree (/Count is never trusted). broken is how many page-tree
// entries couldn't be read. The library panics on some malformed input; that
// comes back as an error.
func open(data []byte) (pages []lpdf.Page, broken int, err error) {
	defer func() {
		if p := recover(); p != nil {
			pages, broken, err = nil, 0, fmt.Errorf("%v", p)
		}
	}()
	r, err := lpdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, 0, err
	}
	pages, broken = r.Pages(maxTreeDepth)
	return pages, broken, nil
}

// extract lays out each selected page (1-indexed) into lines. A page whose
// content stream breaks partway keeps the text read before the break, and its
// error comes back in pageErrs. clean drops running headers, footers and
// page numbers and re-joins hyphenated words.
func extract(all []lpdf.Page, pages []int, clean bool) (lines [][]line, pageErrs []error) {
	for _, i := range pages {
		gs, perr := pageGlyphs(all[i-1])
		lines = append(lines, layout(columns(gs)))
		pageErrs = append(pageErrs, perr)
	}
	if clean {
		lines = stripRunning(lines)
		joinHyphens(lines)
	}
	return lines, pageErrs
}

// short keeps library error text to its first line and about 120 bytes; some
// panics carry a dump of the object that broke.
func short(s string) string {
	s, _, cut := strings.Cut(s, "\n")
	if len(s) > 120 {
		i := 120
		for i > 0 && !utf8.RuneStart(s[i]) {
			i--
		}
		s, cut = s[:i], true
	}
	if cut {
		s += "..."
	}
	return s
}

// errMessage turns a library error into the user-facing message.
func errMessage(err error) string {
	msg := err.Error()
	switch {
	case errors.Is(err, lpdf.ErrInvalidPassword), strings.HasPrefix(msg, "unsupported PDF: encryption"):
		return "pdf: PDF is encrypted: cannot extract text"
	case strings.HasPrefix(msg, "not a PDF file: "):
		return "pdf: not a valid PDF: " + strings.TrimPrefix(msg, "not a PDF file: ")
	default:
		return "pdf: corrupt PDF: " + short(msg)
	}
}

// Flags returns flag metadata for schema generation.
// This FlagSet is never used for parsing - Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("pdf", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringP("pages", "p", "", "pages: N, N-M, N- or a comma list like 1,3,5-7 (1-indexed)")
	fs.BoolP("text", "t", false, "plain text instead of Markdown")
	fs.BoolP("json", "j", false, `one {"page":N,"text":"..."} per page, then {"_vrk":"pdf","count":N}`)
	fs.Bool("as-printed", false, "keep running headers, footers, page numbers and end-of-line hyphens as printed")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk pdf [flags] [file.pdf]",
		"       cat file.pdf | vrk pdf",
		"",
		"PDF text extractor. Writes the text layer as Markdown (headings guessed from",
		"font size), or plain text with --text. Pages are separated by form feeds (\\f).",
		"The argument is a file path, not PDF content. No OCR: scanned PDFs exit 1.",
		"Running headers, footers and page numbers are dropped and words hyphenated",
		"across lines are re-joined; --as-printed keeps them.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("pdf: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
