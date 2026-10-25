package pdf

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// hostile lists the adversarial fixtures from the 2026-10 security review.
// Each one used to crash, hang or eat gigabytes.
var hostile = []string{
	"objstm_cycle.pdf", "objstm_lenself.pdf", "objstm_hugeN.pdf",
	"xs_index_huge.pdf", "xref_start_1000000000.pdf", "xs_size_1099511627776.pdf",
	"contents_arr_20000.pdf", "operands_10.pdf", "array_10.pdf", "litstr_10.pdf",
	"pred_cols_1000000000000.pdf", "pred_cols_-5.pdf", "w_form.pdf", "type0_w_200.pdf",
}

// runHostile runs vrk pdf on a fixture with a 2s deadline and reports how
// many bytes were allocated meanwhile.
func runHostile(t *testing.T, name string, args ...string) (stdout, stderr string, code int, alloc uint64) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "pdf", "fixtures", name)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	done := make(chan struct{})
	go func() {
		stdout, stderr, code = runPdf(t, append(args, path), nil)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s: hung", name)
	}
	runtime.ReadMemStats(&after)
	return stdout, stderr, code, after.TotalAlloc - before.TotalAlloc
}

// Every hostile fixture finishes fast, allocates little and fails cleanly:
// exit 0 or 1, the --json contract kept, no Go runtime text leaked.
func TestHostileFixtures(t *testing.T) {
	for _, name := range hostile {
		t.Run(name, func(t *testing.T) {
			stdout, stderr, code, alloc := runHostile(t, name, "--json")
			if code != 0 && code != 1 {
				t.Errorf("exit %d", code)
			}
			if stderr != "" {
				t.Errorf("stderr not empty with --json: %.200q", stderr)
			}
			for _, l := range strings.Split(strings.TrimSpace(stdout), "\n") {
				if !json.Valid([]byte(l)) {
					t.Errorf("not JSON: %.200q", l)
				}
			}
			if strings.Contains(stdout, "runtime error") {
				t.Errorf("runtime error leaked: %.200q", stdout)
			}
			// Total allocation, not peak: w_form runs 10,000 forms and the
			// reader re-parses each object it resolves, so it churns about
			// 170 MB of short-lived buffers. Before the fixes these were GBs.
			if alloc > 256<<20 {
				t.Errorf("allocated %d MB", alloc>>20)
			}
			t.Logf("exit %d, %d KB allocated, %.120q", code, alloc>>10, stdout)
		})
	}
}

// An object stream that lives in itself overflowed the stack: exit 2 and a
// goroutine dump on stderr.
func TestObjStmCycle(t *testing.T) {
	for _, name := range []string{"objstm_cycle.pdf", "objstm_lenself.pdf"} {
		stdout, stderr, code, _ := runHostile(t, name, "--json")
		assertJSONError(t, stdout, stderr, code, 1, "pdf: ")
	}
}

// Input is capped, and a device or FIFO path is refused before reading:
// /dev/zero used to read until the machine ran out of memory.
func TestInputLimit(t *testing.T) {
	orig := maxInput
	maxInput = 1024
	t.Cleanup(func() { maxInput = orig })
	big := bytes.Repeat([]byte("x"), 2048)

	_, stderr, code := runPdf(t, nil, big)
	if code != 1 || !strings.Contains(stderr, "exceeds") {
		t.Errorf("stdin: exit %d stderr %q", code, stderr)
	}
	_, stderr, code = runPdf(t, []string{writeTemp(t, "big.pdf", big)}, nil)
	if code != 1 || !strings.Contains(stderr, "exceeds") {
		t.Errorf("file: exit %d stderr %q", code, stderr)
	}
	stdout, stderr, code := runPdfDeadline(t, []string{"--json", "/dev/zero"}, nil)
	assertJSONError(t, stdout, stderr, code, 1, "not a regular file")
}

// A /Contents array reads as one stream, even with an operator split across
// two elements. Elements are opened one at a time now.
func TestContentsArray(t *testing.T) {
	data := writePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [4 0 R] /Count 1 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents [5 0 R 6 0 R 7 0 R] >>",
		stream("", "BT /F1 12 Tf 72 720 Td (one) Tj"),
		stream("", "( two) T"),
		stream("", "j ET"),
	}, "")
	stdout, stderr, code := runPdf(t, []string{"--text"}, data)
	if code != 0 || stdout != "one two\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

// The glyph budget covers the whole document, not just each page: 40 pages
// sharing one 1M-glyph stream gave 40 MB of text from an 11 KB file. Past
// the budget, pages keep what they read, warn, and the run still exits 0.
func TestDocGlyphBudget(t *testing.T) {
	orig := maxDocGlyphs
	maxDocGlyphs = 2_500_000
	t.Cleanup(func() { maxDocGlyphs = orig })
	stdout, stderr, code, _ := runHostile(t, "pages_glyphs_10.pdf", "--text")
	if code != 0 {
		t.Fatalf("exit %d stderr %.200q", code, stderr)
	}
	if n := len([]rune(stdout)); n > 2_600_000 || n < 2_000_000 {
		t.Errorf("stdout has %d runes, want about 2.5M", n)
	}
	if strings.Count(stdout, "\f") != 9 {
		t.Errorf("want 9 page separators, got %d", strings.Count(stdout, "\f"))
	}
	if !strings.Contains(stderr, "pdf: page 3: exceeds the document glyph budget") || strings.Contains(stderr, "page 2:") {
		t.Errorf("stderr %.300q", stderr)
	}
}

// Text and warnings from a PDF can't drive the terminal (ESC, BEL, OSC 52) or
// forge a page break or a line: control characters are dropped from glyph
// text and from library errors, and a newline inside a string is a space.
func TestControlChars(t *testing.T) {
	clean := func(s string) bool {
		for _, r := range s {
			if r < 0x20 && r != '\n' && r != '\t' && r != '\f' || r == 0x7f || r >= 0x80 && r <= 0x9f {
				return false
			}
		}
		return true
	}
	for _, name := range []string{"ansi_win.pdf", "hexerr.pdf", "formfeed.pdf"} {
		stdout, stderr, code, _ := runHostile(t, name, "--text")
		if code != 0 || !clean(stdout) || !clean(stderr) {
			t.Errorf("%s: exit %d stdout %q stderr %q", name, code, stdout, stderr)
		}
	}
	stdout, _, _, _ := runHostile(t, "formfeed.pdf", "--text")
	if strings.Contains(stdout, "\f") || strings.Count(stdout, "\n") != 1 {
		t.Errorf("formfeed.pdf: one page, one line, got %q", stdout)
	}
	if got := short("bad \x1b]0;title\a here"); got != "bad ]0;title here" {
		t.Errorf("short: %q", got)
	}
}
