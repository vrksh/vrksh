// Package pdf implements vrk pdf - a PDF text extractor.
// Reads a PDF from a file path or stdin and writes the text layer to stdout:
// pages joined by form feeds, or one JSONL record per page with --json.
//
// Text layer only. No OCR, so scanned PDFs exit 1. No layout reconstruction:
// text comes out in content-stream order. The underlying library
// (github.com/ledongthuc/pdf) reads PDF 1.0-1.7 and rejects 2.0 headers.
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

	lpdf "github.com/ledongthuc/pdf"
	"github.com/spf13/pflag"
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
	Page int    `json:"page"`
	Text string `json:"text"`
}

type metaRecord struct {
	VRK   string `json:"_vrk"`
	Count int    `json:"count"`
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "pdf",
		Short: "PDF text extractor - plain text or JSONL per page, --pages",
		Flags: []shared.FlagMeta{
			{Name: "pages", Shorthand: "p", Usage: "page range: N, N-M, or N- (1-indexed)"},
			{Name: "json", Shorthand: "j", Usage: `one {"page":N,"text":"..."} per page, then {"_vrk":"pdf","count":N}`},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk pdf. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("pdf", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var pagesFlag string
	var jsonFlag, quietFlag bool
	fs.StringVarP(&pagesFlag, "pages", "p", "", "page range: N, N-M, or N- (1-indexed)")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `one {"page":N,"text":"..."} per page, then {"_vrk":"pdf","count":N}`)
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

	pr := pageRange{start: 1}
	if fs.Changed("pages") {
		var ok bool
		if pr, ok = parsePages(pagesFlag); !ok {
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

	r, n, err := open(data)
	if err != nil {
		return fail(1, "%s", errMessage(err))
	}

	if pr.end == 0 {
		pr.end = n
	}
	for _, p := range []int{pr.start, pr.end} {
		if p > n {
			return fail(1, "pdf: page %d does not exist (PDF has %d pages)", p, n)
		}
	}

	texts, err := extract(r, pr)
	if err != nil {
		return fail(1, "pdf: %v", err)
	}

	hasText := false
	for _, t := range texts {
		if strings.TrimSpace(t) != "" {
			hasText = true
			break
		}
	}
	if !hasText {
		return fail(1, "pdf: no extractable text found; document may be scanned (OCR required)")
	}

	w := bufio.NewWriter(os.Stdout)
	if jsonFlag {
		enc := json.NewEncoder(w)
		for i, t := range texts {
			if err := enc.Encode(pageRecord{Page: pr.start + i, Text: t}); err != nil {
				return fail(1, "pdf: writing output: %v", err)
			}
		}
		if err := enc.Encode(metaRecord{VRK: "pdf", Count: len(texts)}); err != nil {
			return fail(1, "pdf: writing output: %v", err)
		}
	} else {
		for i, t := range texts {
			if t != "" {
				texts[i] = t + "\n"
			}
		}
		out := strings.Join(texts, "\f")
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

// parsePages accepts "N", "N-M" and "N-" with N >= 1 and M >= N.
func parsePages(s string) (pageRange, bool) {
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

// open parses a PDF held in memory and returns it with its page count.
// The library panics on some malformed input; that comes back as an error.
func open(data []byte) (r *lpdf.Reader, n int, err error) {
	defer func() {
		if p := recover(); p != nil {
			r, n, err = nil, 0, fmt.Errorf("%v", p)
		}
	}()
	r, err = lpdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, 0, err
	}
	return r, r.NumPage(), nil
}

// extract returns the text of each page in pr, with leading and trailing
// newlines trimmed. pr.end must already be resolved (non-zero).
func extract(r *lpdf.Reader, pr pageRange) (texts []string, err error) {
	defer func() {
		if p := recover(); p != nil {
			texts, err = nil, fmt.Errorf("corrupt PDF: %v", p)
		}
	}()
	for i := pr.start; i <= pr.end; i++ {
		page, ok := pageAt(r, i)
		if !ok {
			return nil, fmt.Errorf("corrupt PDF: page %d missing from page tree", i)
		}
		// nil fonts: each page builds its own cache, since resource names
		// like /F1 can point at different fonts on different pages.
		t, err := page.GetPlainText(nil)
		if err != nil {
			return nil, fmt.Errorf("page %d: %v", i, err)
		}
		texts = append(texts, strings.Trim(t, "\r\n"))
	}
	return texts, nil
}

// pageAt finds 1-indexed page n. Same walk as lpdf.Reader.Page, but with a
// depth cap so a /Kids cycle fails instead of hanging.
func pageAt(r *lpdf.Reader, n int) (lpdf.Page, bool) {
	n--
	node := r.Trailer().Key("Root").Key("Pages")
	for depth := 0; depth < maxTreeDepth; depth++ {
		kids := node.Key("Kids")
		next := lpdf.Value{}
		for i := 0; i < kids.Len() && next.IsNull(); i++ {
			kid := kids.Index(i)
			switch kid.Key("Type").Name() {
			case "Pages":
				if c := int(kid.Key("Count").Int64()); n < c {
					next = kid
				} else {
					n -= c
				}
			case "Page":
				if n == 0 {
					return lpdf.Page{V: kid}, true
				}
				n--
			}
		}
		if next.IsNull() {
			return lpdf.Page{}, false
		}
		node = next
	}
	return lpdf.Page{}, false
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
		return "pdf: corrupt PDF: " + msg
	}
}

// Flags returns flag metadata for schema generation.
// This FlagSet is never used for parsing - Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("pdf", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringP("pages", "p", "", "page range: N, N-M, or N- (1-indexed)")
	fs.BoolP("json", "j", false, `one {"page":N,"text":"..."} per page, then {"_vrk":"pdf","count":N}`)
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk pdf [flags] [file.pdf]",
		"       cat file.pdf | vrk pdf",
		"",
		"PDF text extractor. Writes the text layer, pages separated by form feeds (\\f).",
		"The argument is a file path, not PDF content. No OCR: scanned PDFs exit 1.",
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
