package pdf

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	lpdf "github.com/ledongthuc/pdf"
)

// runPdf replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe - tests share
// global OS state.
func runPdf(t *testing.T, args []string, stdin []byte) (stdout, stderr string, code int) {
	t.Helper()

	origStdin, origStdout, origStderr, origArgs := os.Stdin, os.Stdout, os.Stderr, os.Args
	t.Cleanup(func() {
		os.Stdin, os.Stdout, os.Stderr, os.Args = origStdin, origStdout, origStderr, origArgs
	})

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	// Write in a goroutine so inputs bigger than the pipe buffer don't block.
	go func() {
		_, _ = stdinW.Write(stdin)
		_ = stdinW.Close()
	}()
	os.Stdin = stdinR

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stdout, os.Stderr = stdoutW, stderrW

	var outBuf, errBuf bytes.Buffer
	outDone, errDone := make(chan struct{}), make(chan struct{})
	go func() { _, _ = io.Copy(&outBuf, stdoutR); close(outDone) }()
	go func() { _, _ = io.Copy(&errBuf, stderrR); close(errDone) }()

	os.Args = append([]string{"pdf"}, args...)
	code = Run()

	_ = stdoutW.Close()
	_ = stderrW.Close()
	<-outDone
	<-errDone
	_ = stdinR.Close()
	return outBuf.String(), errBuf.String(), code
}

// assemblePDF writes a PDF 1.4 file with one content stream per page, a
// Helvetica font as /F1, and a correct xref table. kids overrides the /Kids
// array of the page tree when non-empty; trailer is extra text for the
// trailer dictionary.
func assemblePDF(kids, trailer string, contents []string) []byte {
	if kids == "" {
		refs := make([]string, len(contents))
		for i := range contents {
			refs[i] = fmt.Sprintf("%d 0 R", 4+2*i)
		}
		kids = strings.Join(refs, " ")
	}
	objs := []string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		fmt.Sprintf("<< /Type /Pages /Kids [%s] /Count %d >>", kids, len(contents)),
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
	}
	for i, content := range contents {
		objs = append(objs,
			fmt.Sprintf("<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> >> /Contents %d 0 R >>", 5+2*i),
			stream("", content))
	}
	return writePDF(objs, trailer)
}

// stream is a stream object with dict entries extra and the given data.
func stream(extra, data string) string {
	return fmt.Sprintf("<< %s/Length %d >>\nstream\n%s\nendstream", extra, len(data), data)
}

// writePDF numbers objs from 1 and writes them with an xref table. Object 1
// must be the catalog.
func writePDF(objs []string, trailer string) []byte {
	var b bytes.Buffer
	var offs []int
	b.WriteString("%PDF-1.4\n")
	for _, body := range objs {
		offs = append(offs, b.Len())
		fmt.Fprintf(&b, "%d 0 obj\n%s\nendobj\n", len(offs), body)
	}

	xref := b.Len()
	fmt.Fprintf(&b, "xref\n0 %d\n0000000000 65535 f \n", len(offs)+1)
	for _, o := range offs {
		fmt.Fprintf(&b, "%010d 00000 n \n", o)
	}
	fmt.Fprintf(&b, "trailer\n<< /Size %d /Root 1 0 R %s>>\nstartxref\n%d\n%%%%EOF\n", len(offs)+1, trailer, xref)
	return b.Bytes()
}

// textPages turns each page's text into a one-line content stream.
func textPages(pages []string) []string {
	esc := strings.NewReplacer(`\`, `\\`, `(`, `\(`, `)`, `\)`)
	contents := make([]string, len(pages))
	for i, p := range pages {
		if p != "" {
			contents[i] = fmt.Sprintf("BT /F1 12 Tf 72 720 Td (%s) Tj ET", esc.Replace(p))
		}
	}
	return contents
}

// kernedPage spaces words with TJ kerning instead of space characters, the
// way LaTeX does, and moves lines with Td. Text rise makes a superscript.
const kernedPage = "BT /F1 12 Tf 72 720 Td [(Attention)-333(Is)-333(All)] TJ " +
	"0 -14 Td [(Y)40(ou)] TJ 5 Ts (2) Tj 0 Ts " +
	"0 -40 Td (Next para) Tj ET"

const kernedText = "Attention Is All\nYou2\n\nNext para"

// buildPDF returns a valid PDF with one page per argument.
func buildPDF(pages ...string) []byte {
	return assemblePDF("", "", textPages(pages))
}

// buildEncryptedPDF returns a PDF with a Standard security handler whose U
// entry doesn't match the empty password, so opening it needs a password.
func buildEncryptedPDF(pages ...string) []byte {
	enc := "/Encrypt << /Filter /Standard /V 1 /R 2 /O <" + strings.Repeat("ab", 32) +
		"> /U <" + strings.Repeat("cd", 32) + "> /P -4 >> /ID [<" + strings.Repeat("01", 16) +
		"> <" + strings.Repeat("01", 16) + ">] "
	return assemblePDF("", enc, textPages(pages))
}

// buildLoopPDF returns a PDF whose page tree lists itself as its own kid.
// The library's Reader.Page walks this forever.
func buildLoopPDF() []byte {
	return assemblePDF("2 0 R", "", textPages([]string{"p1"}))
}

// buildMissingKidPDF returns a PDF whose page tree points at an object that
// doesn't exist.
func buildMissingKidPDF() []byte {
	return assemblePDF("99 0 R", "", textPages([]string{"p1", "p2"}))
}

func writeTemp(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

type jsonRec struct {
	Page  int    `json:"page"`
	Text  string `json:"text"`
	VRK   string `json:"_vrk"`
	Count int    `json:"count"`
}

func parseJSONL(t *testing.T, s string) []jsonRec {
	t.Helper()
	var recs []jsonRec
	for _, line := range strings.Split(strings.TrimSuffix(s, "\n"), "\n") {
		if line == "" {
			continue
		}
		var r jsonRec
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("bad JSONL line %q: %v", line, err)
		}
		recs = append(recs, r)
	}
	return recs
}

func assertJSONError(t *testing.T, stdout, stderr string, code, wantCode int, wantMsg string) {
	t.Helper()
	if code != wantCode {
		t.Errorf("exit = %d, want %d", code, wantCode)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty with --json, got %q", stderr)
	}
	var e struct {
		Error string `json:"error"`
		Code  int    `json:"code"`
	}
	if err := json.Unmarshal([]byte(stdout), &e); err != nil {
		t.Fatalf("stdout not error JSON: %q (%v)", stdout, err)
	}
	if e.Code != wantCode || !strings.Contains(e.Error, wantMsg) {
		t.Errorf("error JSON = %+v, want code %d and message containing %q", e, wantCode, wantMsg)
	}
}

// --- sanity ---

func TestBuildPDFSanity(t *testing.T) {
	data := buildPDF("p1", "p2", "p3")
	r, err := lpdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("generated PDF doesn't open: %v", err)
	}
	if r.NumPage() != 3 {
		t.Fatalf("NumPage = %d, want 3", r.NumPage())
	}
	enc := buildEncryptedPDF("secret")
	if _, err := lpdf.NewReader(bytes.NewReader(enc), int64(len(enc))); !errors.Is(err, lpdf.ErrInvalidPassword) {
		t.Fatalf("encrypted PDF opened with empty password: err = %v", err)
	}
}

// --- happy paths ---

func TestStdinMultiPage(t *testing.T) {
	stdout, stderr, code := runPdf(t, nil, buildPDF("p1", "p2", "p3"))
	if code != 0 {
		t.Fatalf("exit = %d, stderr = %q", code, stderr)
	}
	// Markdown pages keep blank lines around the form feed.
	if stdout != "p1\n\n\f\np2\n\n\f\np3\n" {
		t.Errorf("stdout = %q", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr = %q", stderr)
	}
	stdout, _, _ = runPdf(t, []string{"--text"}, buildPDF("p1", "p2", "p3"))
	if stdout != "p1\n\fp2\n\fp3\n" {
		t.Errorf("--text stdout = %q", stdout)
	}
}

func TestFileArgMatchesStdin(t *testing.T) {
	data := buildPDF("p1", "p2", "p3")
	path := writeTemp(t, "doc.pdf", data)
	fromFile, _, code := runPdf(t, []string{path}, nil)
	if code != 0 {
		t.Fatalf("exit = %d", code)
	}
	fromStdin, _, _ := runPdf(t, nil, data)
	if fromFile != fromStdin {
		t.Errorf("file %q != stdin %q", fromFile, fromStdin)
	}
}

func TestKerningBecomesSpaces(t *testing.T) {
	stdout, stderr, code := runPdf(t, nil, assemblePDF("", "", []string{kernedPage}))
	if code != 0 || stdout != kernedText+"\n" {
		t.Errorf("exit %d stdout %q stderr %q, want %q", code, stdout, stderr, kernedText+"\n")
	}
}

func TestMarkdown(t *testing.T) {
	page := "BT /F1 24 Tf 72 720 Td (Big Title) Tj ET " +
		"BT /F1 12 Tf 72 680 Td (Body line one) Tj 0 -14 Td (# not a heading) Tj " +
		"0 -14 Td (\\225 item) Tj 0 -14 Td (---) Tj ET"
	data := assemblePDF("", "", []string{page})
	stdout, stderr, code := runPdf(t, nil, data)
	want := "# Big Title\n\nBody line one\n\\# not a heading\n- item\n\\---\n"
	if code != 0 || stdout != want {
		t.Errorf("exit %d stdout %q stderr %q, want %q", code, stdout, stderr, want)
	}
	stdout, _, _ = runPdf(t, []string{"--text"}, data)
	want = "Big Title\nBody line one\n# not a heading\n\u2022 item\n---\n"
	if stdout != want {
		t.Errorf("--text stdout %q, want %q", stdout, want)
	}
}

// Same-size bold lines are headings when a full line of plain text follows,
// and not when a short one does, like a bold name over an affiliation.
func TestBoldHeadings(t *testing.T) {
	body := "Plain text that runs long enough to be a paragraph."
	page := "BT /F1 12 Tf 72 720 Td (Report) Tj ET " +
		"BT /F2 12 Tf 72 680 Td (Ana Rao) Tj /F1 12 Tf 0 -14 Td (Google) Tj ET " +
		"BT /F2 12 Tf 72 620 Td (3.2.1 Scaled Attention) Tj /F1 12 Tf 0 -14 Td (" + body + ") Tj " +
		"/F2 12 Tf 0 -14 Td (Bold but carries on in the) Tj /F1 12 Tf 0 -14 Td (" + body + ") Tj ET"
	data := writePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [5 0 R] /Count 1 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /ABCDEF+Helvetica-Bold /Encoding /WinAnsiEncoding >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R /F2 4 0 R >> >> /Contents 6 0 R >>",
		stream("", page),
	}, "")
	stdout, stderr, code := runPdf(t, nil, data)
	want := "Report\n\nAna Rao\nGoogle\n\n# 3.2.1 Scaled Attention\n\n" + body + "\nBold but carries on in the\n" + body + "\n"
	if code != 0 || stdout != want {
		t.Errorf("exit %d stderr %q\nstdout %q\nwant   %q", code, stderr, stdout, want)
	}
}

// TestRowOrderedColumns: some generators write two columns a row at a time,
// left fragment then right. The left column should still read first.
func TestRowOrderedColumns(t *testing.T) {
	var page strings.Builder
	var want []string
	for i := 1; i <= 6; i++ {
		y := 720 - 14*i
		fmt.Fprintf(&page, "BT /F1 12 Tf 72 %d Td (Left %d) Tj ET BT /F1 12 Tf 320 %d Td (Right %d) Tj ET ", y, i, y, i)
		want = append(want, fmt.Sprintf("Left %d", i))
	}
	for i := 1; i <= 6; i++ {
		want = append(want, fmt.Sprintf("Right %d", i))
	}
	stdout, stderr, code := runPdf(t, []string{"--text"}, assemblePDF("", "", []string{page.String()}))
	if code != 0 || stdout != strings.Join(want, "\n")+"\n" {
		t.Errorf("exit %d stderr %q\nstdout %q", code, stderr, stdout)
	}
}

// TestTables: rows whose cells line up past wide gaps become a Markdown
// table. A lone gapped line stays text, and --text keeps plain lines.
func TestTables(t *testing.T) {
	row := func(y int, a, b, c string) string {
		return fmt.Sprintf("BT /F1 12 Tf 72 %d Td (%s) Tj ET BT /F1 12 Tf 200 %d Td (%s) Tj ET BT /F1 12 Tf 300 %d Td (%s) Tj ET ", y, a, y, b, y, c)
	}
	page := row(720, "model", "err", "a|b") + row(706, "VGG", "7.3", "x") + row(692, "ResNet", "3.5", "y") +
		"BT /F1 12 Tf 72 640 Td (Prose after the table.) Tj ET " + row(600, "Lone", "row", "here")
	data := assemblePDF("", "", []string{page})
	stdout, stderr, code := runPdf(t, nil, data)
	want := "| model | err | a\\|b |\n| --- | --- | --- |\n| VGG | 7.3 | x |\n| ResNet | 3.5 | y |\n\n" +
		"Prose after the table.\n\nLone row here\n"
	if code != 0 || stdout != want {
		t.Errorf("exit %d stderr %q\nstdout %q\nwant   %q", code, stderr, stdout, want)
	}
	stdout, _, _ = runPdf(t, []string{"--text"}, data)
	if !strings.HasPrefix(stdout, "model err a|b\nVGG 7.3 x\n") {
		t.Errorf("--text stdout %q", stdout)
	}
}

func TestFormXObject(t *testing.T) {
	data := writePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [4 0 R] /Count 1 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> /XObject << /X1 6 0 R >> >> /Contents 5 0 R >>",
		stream("", "BT /F1 12 Tf 72 720 Td (Body) Tj ET q 1 0 0 1 72 720 cm /X1 Do Q"),
		stream("/Type /XObject /Subtype /Form /BBox [0 0 200 50] /Matrix [1 0 0 1 0 -14] ", "BT /F1 12 Tf 0 0 Td (Inside form) Tj ET"),
	}, "")
	stdout, stderr, code := runPdf(t, []string{"--text"}, data)
	if code != 0 || stdout != "Body\nInside form\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

// A form that draws itself stops at the depth cap instead of recursing forever.
func TestFormXObjectCycle(t *testing.T) {
	data := writePDF([]string{
		"<< /Type /Catalog /Pages 2 0 R >>",
		"<< /Type /Pages /Kids [4 0 R] /Count 1 >>",
		"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica /Encoding /WinAnsiEncoding >>",
		"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Resources << /Font << /F1 3 0 R >> /XObject << /X1 5 0 R >> >> /Contents 6 0 R >>",
		stream("/Type /XObject /Subtype /Form /BBox [0 0 200 50] ", "/X1 Do"),
		stream("", "BT /F1 12 Tf 72 720 Td (Body) Tj ET /X1 Do"),
	}, "")
	if stdout, stderr, code := runPdf(t, []string{"--text"}, data); code != 0 || stdout != "Body\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestClipsToPageBox(t *testing.T) {
	page := "BT /F1 12 Tf 72 720 Td (Inside) Tj ET BT /F1 12 Tf 700 400 Td (Outside) Tj ET"
	stdout, stderr, code := runPdf(t, []string{"--text"}, assemblePDF("", "", []string{page}))
	if code != 0 || stdout != "Inside\n" {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestBrokenPageKeepsText(t *testing.T) {
	data := assemblePDF("", "", []string{"BT /F1 12 Tf 72 720 Td (Good) Tj ET end (Lost) Tj", "BT /F1 12 Tf 72 720 Td (p2) Tj ET"})
	stdout, stderr, code := runPdf(t, []string{"--text"}, data)
	if code != 0 || stdout != "Good\n\fp2\n" || !strings.Contains(stderr, "pdf: page 1: ") {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = runPdf(t, []string{"--json"}, data)
	if code != 0 || stderr != "" || !strings.Contains(stdout, `"text":"Good","error":"pdf: page 1: `) {
		t.Errorf("--json: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestStandardFontWidths(t *testing.T) {
	// No /Widths on the font, so the gaps only make sense with Helvetica's
	// real widths: M is 833/1000 em, i is 222.
	cases := map[string]string{
		"BT /F1 10 Tf 72 720 Td (MMMM) Tj ET BT /F1 10 Tf 105.32 720 Td (x) Tj ET": "MMMMx",
		"BT /F1 10 Tf 72 720 Td (iiii) Tj ET BT /F1 10 Tf 80.88 720 Td (x) Tj ET":  "iiiix",
		"BT /F1 10 Tf 72 720 Td (iiii) Tj ET BT /F1 10 Tf 83.88 720 Td (x) Tj ET":  "iiii x",
	}
	for page, want := range cases {
		stdout, _, _ := runPdf(t, []string{"--text"}, assemblePDF("", "", []string{page}))
		if stdout != want+"\n" {
			t.Errorf("%s: got %q, want %q", page, stdout, want)
		}
	}
}

func TestShortError(t *testing.T) {
	if got := short("first\nsecond"); got != "first..." {
		t.Errorf("short = %q", got)
	}
	long := strings.Repeat("é", 100)
	if got := short(long); len(got) > 123 || !strings.HasSuffix(got, "...") || !utf8.ValidString(got) {
		t.Errorf("short(long) = %q", got)
	}
}

func TestEmptyPages(t *testing.T) {
	cases := []struct {
		pages []string
		want  string
	}{
		{[]string{"p1", "", "p3"}, "p1\n\f\fp3\n"},
		{[]string{"p1", ""}, "p1\n\f\n"},
	}
	for _, tc := range cases {
		stdout, stderr, code := runPdf(t, []string{"-t"}, buildPDF(tc.pages...))
		if code != 0 || stdout != tc.want {
			t.Errorf("pages %q: exit %d stdout %q stderr %q, want %q", tc.pages, code, stdout, stderr, tc.want)
		}
	}
}

func TestJSONOutput(t *testing.T) {
	stdout, stderr, code := runPdf(t, []string{"--json"}, buildPDF("p1", "p2", "p3"))
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	recs := parseJSONL(t, stdout)
	if len(recs) != 4 {
		t.Fatalf("got %d records, want 4: %q", len(recs), stdout)
	}
	for i := 0; i < 3; i++ {
		if recs[i].Page != i+1 || recs[i].Text != fmt.Sprintf("p%d", i+1) {
			t.Errorf("record %d = %+v", i, recs[i])
		}
	}
	if recs[3].VRK != "pdf" || recs[3].Count != 3 {
		t.Errorf("trailer = %+v", recs[3])
	}
	if !strings.HasSuffix(stdout, `{"_vrk":"pdf","count":3}`+"\n") {
		t.Errorf("trailer bytes wrong: %q", stdout)
	}
}

func TestJSONPagesKeepRealNumbers(t *testing.T) {
	stdout, _, code := runPdf(t, []string{"--json", "--pages", "2-3"}, buildPDF("p1", "p2", "p3"))
	if code != 0 {
		t.Fatalf("exit %d", code)
	}
	recs := parseJSONL(t, stdout)
	if len(recs) != 3 || recs[0].Page != 2 || recs[1].Page != 3 || recs[2].Count != 2 {
		t.Errorf("records = %+v", recs)
	}
}

func TestPagesSelection(t *testing.T) {
	data := buildPDF("p1", "p2", "p3", "p4", "p5")
	cases := map[string]string{
		"3":         "p3\n",
		"2-5":       "p2\n\fp3\n\fp4\n\fp5\n",
		"3-":        "p3\n\fp4\n\fp5\n",
		" 3 ":       "p3\n",
		"1,3":       "p1\n\fp3\n",
		"4-,1":      "p4\n\fp5\n\fp1\n",
		"1-3, 2, 5": "p1\n\fp2\n\fp3\n\fp5\n",
	}
	for arg, want := range cases {
		stdout, stderr, code := runPdf(t, []string{"--text", "--pages", arg}, data)
		if code != 0 || stdout != want {
			t.Errorf("--pages %q: exit %d stdout %q stderr %q, want %q", arg, code, stdout, stderr, want)
		}
	}
	// -p shorthand
	if stdout, _, _ := runPdf(t, []string{"-p", "1"}, data); stdout != "p1\n" {
		t.Errorf("-p 1 = %q", stdout)
	}
}

// --- usage errors ---

func TestPagesInvalid(t *testing.T) {
	data := buildPDF("p1", "p2", "p3")
	for _, arg := range []string{"0", "5-2", "abc", "-3", "1-2-3", "+3", " ", "0-2", "2-0", "1,", ",1", "1,,2", "1;2"} {
		stdout, stderr, code := runPdf(t, []string{"--pages=" + arg}, data)
		if code != 2 {
			t.Errorf("--pages %q: exit %d, want 2", arg, code)
		}
		if stdout != "" {
			t.Errorf("--pages %q: stdout %q", arg, stdout)
		}
		if !strings.Contains(stderr, "pdf: invalid page range: "+arg) {
			t.Errorf("--pages %q: stderr %q", arg, stderr)
		}
	}
}

func TestTwoPositionalArgs(t *testing.T) {
	_, stderr, code := runPdf(t, []string{"a.pdf", "b.pdf"}, nil)
	if code != 2 || !strings.Contains(stderr, "pdf:") {
		t.Errorf("exit %d stderr %q", code, stderr)
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runPdf(t, []string{"--help"}, nil)
	if code != 0 || !strings.Contains(stdout, "pdf") || !strings.Contains(stdout, "--pages") {
		t.Errorf("exit %d stdout %q", code, stdout)
	}
}

func TestUnknownFlag(t *testing.T) {
	stdout, _, code := runPdf(t, []string{"--nope"}, nil)
	if code != 2 || stdout != "" {
		t.Errorf("exit %d stdout %q", code, stdout)
	}
}

func TestTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	stdout, stderr, code := runPdf(t, nil, nil)
	if code != 2 || stdout != "" || !strings.Contains(stderr, "pdf: no input: provide a file path or pipe a PDF to stdin") {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}

	stdout, stderr, code = runPdf(t, []string{"--json"}, nil)
	assertJSONError(t, stdout, stderr, code, 2, "pdf: no input")

	path := writeTemp(t, "doc.pdf", buildPDF("p1"))
	stdout, _, code = runPdf(t, []string{path}, nil)
	if code != 0 || stdout != "p1\n" {
		t.Errorf("TTY + file: exit %d stdout %q", code, stdout)
	}
}

// --- runtime errors ---

func TestPageOutOfRange(t *testing.T) {
	data := buildPDF("p1", "p2", "p3")
	for _, arg := range []string{"9", "2-9", "9-", "1,9"} {
		stdout, stderr, code := runPdf(t, []string{"--pages", arg}, data)
		if code != 1 || stdout != "" || !strings.Contains(stderr, "pdf: page 9 does not exist (PDF has 3 pages)") {
			t.Errorf("--pages %s: exit %d stdout %q stderr %q", arg, code, stdout, stderr)
		}
	}
}

const ocrMsg = "pdf: no extractable text found; document may be scanned (OCR required)"

func TestNoText(t *testing.T) {
	stdout, stderr, code := runPdf(t, nil, buildPDF("", ""))
	if code != 1 || stdout != "" || !strings.Contains(stderr, ocrMsg) {
		t.Errorf("blank: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	// A blank page in a PDF that has text elsewhere is not a scan.
	stdout, stderr, code = runPdf(t, []string{"--pages", "2"}, buildPDF("p1", ""))
	if code != 1 || stdout != "" || !strings.Contains(stderr, "pdf: no extractable text on the selected pages") {
		t.Errorf("blank page 2: exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = runPdf(t, []string{"--json"}, buildPDF(""))
	assertJSONError(t, stdout, stderr, code, 1, ocrMsg)
}

func TestEncrypted(t *testing.T) {
	stdout, stderr, code := runPdf(t, nil, buildEncryptedPDF("secret"))
	if code != 1 || stdout != "" || !strings.Contains(stderr, "pdf: PDF is encrypted: cannot extract text") {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestNotAPDF(t *testing.T) {
	stdout, stderr, code := runPdf(t, nil, []byte("just some text, not a pdf\n"))
	if code != 1 || stdout != "" || !strings.Contains(stderr, "pdf: not a valid PDF: ") {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
}

func TestPDF20Rejected(t *testing.T) {
	data := buildPDF("p1")
	data = append([]byte("%PDF-2.0"), data[len("%PDF-1.4"):]...)
	_, stderr, code := runPdf(t, nil, data)
	if code != 1 || !strings.Contains(stderr, "pdf: PDF 2.0 is not supported") {
		t.Errorf("exit %d stderr %q", code, stderr)
	}
}

func TestJSONKeepsHTMLCharacters(t *testing.T) {
	stdout, _, code := runPdf(t, []string{"--json"}, buildPDF("a < b && c > d"))
	if code != 0 || !strings.Contains(stdout, `"text":"a < b && c > d"`) {
		t.Errorf("exit %d stdout %q", code, stdout)
	}
}

func TestBrokenPageTree(t *testing.T) {
	for name, data := range map[string][]byte{"loop": buildLoopPDF(), "missing kid": buildMissingKidPDF()} {
		done := make(chan struct{})
		var stdout, stderr string
		var code int
		go func() {
			stdout, stderr, code = runPdf(t, nil, data)
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: hung", name)
		}
		if code != 1 || stdout != "" || !strings.Contains(stderr, "pdf: corrupt PDF: ") {
			t.Errorf("%s: exit %d stdout %q stderr %q", name, code, stdout, stderr)
		}
	}
}

func TestMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nope.pdf")
	stdout, stderr, code := runPdf(t, []string{path}, nil)
	if code != 1 || stdout != "" || !strings.Contains(stderr, "pdf: file not found: "+path) {
		t.Errorf("exit %d stdout %q stderr %q", code, stdout, stderr)
	}
	stdout, stderr, code = runPdf(t, []string{"--json", path}, nil)
	assertJSONError(t, stdout, stderr, code, 1, "pdf: file not found")
}

func TestJSONReadError(t *testing.T) {
	orig := readAll
	readAll = func(io.Reader) ([]byte, error) { return nil, errors.New("simulated") }
	t.Cleanup(func() { readAll = orig })

	stdout, stderr, code := runPdf(t, []string{"--json"}, nil)
	assertJSONError(t, stdout, stderr, code, 1, "simulated")
}

func TestJSONErrorsKeepStderrEmpty(t *testing.T) {
	stdout, stderr, code := runPdf(t, []string{"--json"}, buildEncryptedPDF("x"))
	assertJSONError(t, stdout, stderr, code, 1, "encrypted")
	stdout, stderr, code = runPdf(t, []string{"--json", "--pages", "abc"}, buildPDF("p1"))
	assertJSONError(t, stdout, stderr, code, 2, "invalid page range")
}

func TestEmptyStdin(t *testing.T) {
	for _, args := range [][]string{nil, {"--json"}} {
		stdout, stderr, code := runPdf(t, args, nil)
		if code != 0 || stdout != "" || stderr != "" {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, code, stdout, stderr)
		}
	}
}

func TestQuiet(t *testing.T) {
	_, stderr, code := runPdf(t, []string{"--quiet"}, []byte("not a pdf"))
	if code != 1 || stderr != "" {
		t.Errorf("exit %d stderr %q", code, stderr)
	}
}

// --- property ---

func TestPropertyJSONRoundTrip(t *testing.T) {
	rng := rand.New(rand.NewSource(1))
	for iter := 0; iter < 50; iter++ {
		pages := make([]string, 1+rng.Intn(5))
		for i := range pages {
			// Printable ASCII with no leading or trailing space: the library
			// can't tell a positioned space from layout at the edges.
			n := 1 + rng.Intn(40)
			b := make([]byte, n)
			for j := range b {
				b[j] = byte(0x20 + rng.Intn(0x7f-0x20))
				if (j == 0 || j == n-1) && b[j] == ' ' {
					b[j] = 'x'
				}
			}
			pages[i] = string(b)
		}
		stdout, stderr, code := runPdf(t, []string{"--json", "--text"}, buildPDF(pages...))
		if code != 0 {
			t.Fatalf("pages %q: exit %d stderr %q", pages, code, stderr)
		}
		recs := parseJSONL(t, stdout)
		if len(recs) != len(pages)+1 || recs[len(pages)].Count != len(pages) {
			t.Fatalf("pages %q: records %+v", pages, recs)
		}
		for i, p := range pages {
			if recs[i].Page != i+1 || recs[i].Text != p {
				t.Errorf("page %d: got %+v, want %q", i+1, recs[i], p)
			}
		}
	}
}

// --- fuzz ---

func FuzzPdf(f *testing.F) {
	f.Add(buildPDF("p1", "p2"))
	f.Add(buildPDF(""))
	f.Add(buildEncryptedPDF("x"))
	f.Add(buildLoopPDF())
	f.Add(buildMissingKidPDF())
	f.Add([]byte("%PDF-1.4\n%%EOF\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		done := make(chan struct{})
		go func() {
			defer close(done)
			r, n, err := open(data)
			if err != nil {
				return
			}
			pages, _ := selectPages([]pageRange{{start: 1, end: min(n, 20)}}, n)
			lines, _, _ := extract(r, pages)
			h := findHeadings(lines)
			for _, ls := range lines {
				_ = markdown(ls, h)
				_ = plainText(ls)
			}
		}()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatalf("hung on %d bytes", len(data))
		}
	})
}

// --- fixtures ---

// TestWriteFixtures regenerates the committed smoke fixtures.
// Run with VRK_UPDATE_FIXTURES=1 make test-tool TOOL=pdf.
func TestWriteFixtures(t *testing.T) {
	if os.Getenv("VRK_UPDATE_FIXTURES") == "" {
		t.Skip("set VRK_UPDATE_FIXTURES=1 to regenerate testdata/pdf/fixtures")
	}
	dir := filepath.Join("..", "..", "testdata", "pdf", "fixtures")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	files := map[string][]byte{
		"text.pdf":      buildPDF("Quarterly report, page one.", "Revenue grew in every region.", "Outlook for next year."),
		"blank.pdf":     buildPDF("", ""),
		"encrypted.pdf": buildEncryptedPDF("secret"),
		"kerned.pdf":    assemblePDF("", "", []string{kernedPage}),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// Repeated edge lines go, page numbers everywhere and headers after their
// first copy. Body text that repeats in the middle of a page stays.
func TestStripRunning(t *testing.T) {
	page := func(n string) []line {
		w := map[string]string{"1": "one", "2": "two", "3": "three"}[n]
		ls := []line{{text: "Journal of Things"}, {text: "intro " + w}, {text: "Same body line"}, {text: "Same body line"}, {text: "end " + w}, {text: n}}
		for i := range ls {
			ls[i].upright, ls[i].y = true, float64(700-i*20)
		}
		return ls
	}
	got := stripRunning([][]line{page("1"), page("2"), page("3")})
	var texts []string
	for _, p := range got {
		var ts []string
		for _, l := range p {
			ts = append(ts, l.text)
		}
		texts = append(texts, strings.Join(ts, "|"))
	}
	want := []string{
		"Journal of Things|intro one|Same body line|Same body line|end one",
		"intro two|Same body line|Same body line|end two",
		"intro three|Same body line|Same body line|end three",
	}
	if !reflect.DeepEqual(texts, want) {
		t.Errorf("got %q\nwant %q", texts, want)
	}
	// Two pages aren't enough to call anything a header.
	if got := stripRunning([][]line{page("1"), page("2")}); len(got[1]) != 6 {
		t.Errorf("two pages: kept %d lines, want 6", len(got[1]))
	}
}

func TestJoinHyphens(t *testing.T) {
	texts := []string{
		"we improve on the exist-", "ing best result, by a non-", "trivial margin with self-",
		"attention, self-attention and the English-", "to-German task. A stop-the-", "world pause.",
		"Next para-", "graph",
	}
	p := make([]line, len(texts))
	for i, s := range texts {
		p[i] = line{text: s, upright: true}
	}
	p[len(p)-1].brk = true // a paragraph break stops the join
	p = append(p, line{text: "non-zero ends here", upright: true})
	joinHyphens([][]line{p})
	var got []string
	for _, l := range p {
		got = append(got, l.text)
	}
	want := []string{
		"we improve on the existing", "best result, by a non-trivial", "margin with self-attention,",
		"self-attention and the English-to-German", "task. A stop-the-world", "pause.",
		"Next para-", "graph", "non-zero ends here",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
