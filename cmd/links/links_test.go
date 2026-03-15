package links

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// runLinks replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runLinks(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
	t.Helper()

	origStdin := os.Stdin
	origStdout := os.Stdout
	origStderr := os.Stderr
	origArgs := os.Args

	t.Cleanup(func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
		os.Stderr = origStderr
		os.Args = origArgs
	})

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if _, err := io.WriteString(stdinW, stdinContent); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := stdinW.Close(); err != nil {
		t.Fatalf("close stdin write end: %v", err)
	}
	os.Stdin = stdinR

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	os.Stdout = stdoutW

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stderr = stderrW

	os.Args = append([]string{"links"}, args...)
	code = Run()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, stdoutR); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if _, err := io.Copy(&errBuf, stderrR); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	_ = stdoutR.Close()
	_ = stderrR.Close()

	return outBuf.String(), errBuf.String(), code
}

// parseRecords parses JSONL from stdout, skipping blank lines.
func parseRecords(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("invalid JSON line %q: %v", line, err)
		}
		records = append(records, m)
	}
	return records
}

func TestPositionalArg(t *testing.T) {
	// Positional arg bypasses stdin entirely — TTY guard must not fire.
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runLinks(t, []string{"See [link](https://example.com)"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1; stdout=%q", len(recs), stdout)
	}
	if recs[0]["url"] != "https://example.com" {
		t.Errorf("url = %q, want %q", recs[0]["url"], "https://example.com")
	}
	if recs[0]["text"] != "link" {
		t.Errorf("text = %q, want %q", recs[0]["text"], "link")
	}
}

func TestMarkdownInlineLink(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "See [Homebrew](https://brew.sh) for install.\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["text"] != "Homebrew" {
		t.Errorf("text = %q, want %q", recs[0]["text"], "Homebrew")
	}
	if recs[0]["url"] != "https://brew.sh" {
		t.Errorf("url = %q, want %q", recs[0]["url"], "https://brew.sh")
	}
	if recs[0]["line"] != float64(1) {
		t.Errorf("line = %v, want 1", recs[0]["line"])
	}
}

func TestHTMLAnchorTag(t *testing.T) {
	stdout, _, code := runLinks(t, nil, `<a href="https://example.com">Example</a>`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["text"] != "Example" {
		t.Errorf("text = %q, want %q", recs[0]["text"], "Example")
	}
	if recs[0]["url"] != "https://example.com" {
		t.Errorf("url = %q, want %q", recs[0]["url"], "https://example.com")
	}
}

func TestBareURL(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "Visit https://example.com for more.\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["text"] != recs[0]["url"] {
		t.Errorf("bare URL: text %q != url %q", recs[0]["text"], recs[0]["url"])
	}
	if recs[0]["url"] != "https://example.com" {
		t.Errorf("url = %q, want %q", recs[0]["url"], "https://example.com")
	}
}

func TestMarkdownRefLink(t *testing.T) {
	input := "Check [Homebrew][brew].\n\n[brew]: https://brew.sh\n"
	stdout, _, code := runLinks(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1; stdout=%q", len(recs), stdout)
	}
	if recs[0]["text"] != "Homebrew" {
		t.Errorf("text = %q, want %q", recs[0]["text"], "Homebrew")
	}
	if recs[0]["url"] != "https://brew.sh" {
		t.Errorf("url = %q, want %q", recs[0]["url"], "https://brew.sh")
	}
	if recs[0]["line"] != float64(1) {
		t.Errorf("line = %v, want 1 (line of usage, not definition)", recs[0]["line"])
	}
}

func TestMarkdownRefUsageNoDefinition(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "[text][missing]\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty (unresolved ref skipped)", stdout)
	}
}

func TestOrphanRefDefinitionNotEmitted(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "[label]: https://example.com\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty (orphan ref def not emitted)", stdout)
	}
}

func TestBareFlag(t *testing.T) {
	stdout, _, code := runLinks(t, []string{"--bare"}, "See [Homebrew](https://brew.sh) for install.\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "https://brew.sh" {
		t.Errorf("--bare stdout = %q, want %q", strings.TrimSpace(stdout), "https://brew.sh")
	}
}

func TestJSONTrailingMetadata(t *testing.T) {
	stdout, _, code := runLinks(t, []string{"--json"}, "[link](https://example.com)\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (link + metadata)", len(recs))
	}
	meta := recs[len(recs)-1]
	if meta["_vrk"] != "links" {
		t.Errorf("metadata _vrk = %q, want %q", meta["_vrk"], "links")
	}
	if meta["count"] != float64(1) {
		t.Errorf("metadata count = %v, want 1", meta["count"])
	}
}

func TestJSONNoLinksMetadataOnly(t *testing.T) {
	stdout, _, code := runLinks(t, []string{"--json"}, "no links here\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1 (only metadata); stdout=%q", len(recs), stdout)
	}
	if recs[0]["_vrk"] != "links" {
		t.Errorf("metadata _vrk = %q, want %q", recs[0]["_vrk"], "links")
	}
	if recs[0]["count"] != float64(0) {
		t.Errorf("metadata count = %v, want 0", recs[0]["count"])
	}
}

func TestBarePlusJSON(t *testing.T) {
	stdout, _, code := runLinks(t, []string{"--bare", "--json"}, "See [A](https://a.com) and [B](https://b.com).\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 lines, got %d; stdout=%q", len(lines), stdout)
	}
	// Last line must be the JSON metadata record.
	last := lines[len(lines)-1]
	var meta map[string]any
	if err := json.Unmarshal([]byte(last), &meta); err != nil {
		t.Fatalf("last line is not JSON: %v (got %q)", err, last)
	}
	if meta["_vrk"] != "links" {
		t.Errorf("metadata _vrk = %q, want %q", meta["_vrk"], "links")
	}
	// Preceding lines are bare URLs.
	for _, l := range lines[:len(lines)-1] {
		if l != "https://a.com" && l != "https://b.com" {
			t.Errorf("unexpected bare line: %q", l)
		}
	}
}

func TestEmptyStdin(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestNoLinksInInput(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "no links here\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestInteractiveTTYNoArg(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runLinks(t, nil, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on usage error", stdout)
	}
	if !strings.Contains(stderr, "no input") {
		t.Errorf("stderr = %q, want 'no input'", stderr)
	}
}

func TestMultipleLinksOnOneLine(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "[A](https://a.com) and [B](https://b.com)\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
	if recs[0]["url"] != "https://a.com" {
		t.Errorf("recs[0].url = %q, want %q", recs[0]["url"], "https://a.com")
	}
	if recs[1]["url"] != "https://b.com" {
		t.Errorf("recs[1].url = %q, want %q", recs[1]["url"], "https://b.com")
	}
}

func TestDuplicateURLsBothEmitted(t *testing.T) {
	input := "[A](https://example.com)\n[B](https://example.com)\n"
	stdout, _, code := runLinks(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (no dedup)", len(recs))
	}
}

func TestBareURLInsideMarkdownNotDoubleCounted(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "[Homebrew](https://brew.sh)\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1 (bare URL inside Markdown not double-counted); stdout=%q", len(recs), stdout)
	}
	if recs[0]["text"] != "Homebrew" {
		t.Errorf("text = %q, want %q (should be Markdown text, not bare URL)", recs[0]["text"], "Homebrew")
	}
}

func TestRelativeURLEmittedAsIs(t *testing.T) {
	stdout, _, code := runLinks(t, nil, `<a href="/about">About</a>`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["url"] != "/about" {
		t.Errorf("url = %q, want %q", recs[0]["url"], "/about")
	}
}

func TestHTMLCaseInsensitive(t *testing.T) {
	stdout, _, code := runLinks(t, nil, `<A HREF="https://example.com">Example</A>`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["url"] != "https://example.com" {
		t.Errorf("url = %q, want %q", recs[0]["url"], "https://example.com")
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runLinks(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "links") {
		t.Errorf("--help stdout = %q, want it to contain 'links'", stdout)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, code := runLinks(t, []string{"--bogus"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	origReadAll := readAll
	readAll = func(r io.Reader) ([]byte, error) {
		return nil, errors.New("simulated read error")
	}
	defer func() { readAll = origReadAll }()

	stdout, stderr, code := runLinks(t, []string{"--json"}, "any input")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json active", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	if obj["code"] != float64(1) {
		t.Errorf("JSON code = %v, want 1", obj["code"])
	}
}

// TestBareEmptyStdin ensures --bare exits 0 with no output on empty stdin.
func TestBareEmptyStdin(t *testing.T) {
	stdout, _, code := runLinks(t, []string{"--bare"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// TestBareJSONEmptyStdin ensures --bare --json on empty stdin emits only the
// metadata record (count:0) and exits 0.
func TestBareJSONEmptyStdin(t *testing.T) {
	stdout, _, code := runLinks(t, []string{"--bare", "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1 (only metadata)", len(recs))
	}
	if recs[0]["_vrk"] != "links" {
		t.Errorf("metadata _vrk = %q, want %q", recs[0]["_vrk"], "links")
	}
	if recs[0]["count"] != float64(0) {
		t.Errorf("metadata count = %v, want 0", recs[0]["count"])
	}
}

// TestInteractiveTTYWithJSONFlag ensures that when stdin is a TTY and --json
// is active, the error goes to stdout (not stderr) and exit code is 2.
func TestInteractiveTTYWithJSONFlag(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runLinks(t, []string{"--json"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json active", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	if obj["code"] != float64(2) {
		t.Errorf("JSON code = %v, want 2", obj["code"])
	}
}

// TestEmptyTextInlineLink verifies that [](url) is emitted with text="" and
// the correct URL. The spec does not prohibit empty anchor text, so we emit
// it rather than skip it — callers can filter on text == "" if needed.
func TestEmptyTextInlineLink(t *testing.T) {
	stdout, _, code := runLinks(t, nil, "[](https://example.com)\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseRecords(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["text"] != "" {
		t.Errorf("text = %q, want empty string for [](url)", recs[0]["text"])
	}
	if recs[0]["url"] != "https://example.com" {
		t.Errorf("url = %q, want %q", recs[0]["url"], "https://example.com")
	}
}

// TestPropertyRecordFields verifies that every emitted record has a non-empty
// "url" and "line" >= 1, across a variety of input types.
func TestPropertyRecordFields(t *testing.T) {
	inputs := []string{
		"[A](https://a.com)\n",
		`<a href="https://b.com">B</a>` + "\n",
		"https://c.com\n",
		"[A](https://a.com) https://b.com\n",
		"[ref][r]\n\n[r]: https://r.com\n",
	}
	for _, input := range inputs {
		stdout, _, code := runLinks(t, nil, input)
		if code != 0 {
			t.Errorf("input %q: exit code = %d, want 0", input, code)
			continue
		}
		recs := parseRecords(t, stdout)
		for i, rec := range recs {
			url, _ := rec["url"].(string)
			if url == "" {
				t.Errorf("input %q record[%d]: url is empty", input, i)
			}
			line, _ := rec["line"].(float64)
			if line < 1 {
				t.Errorf("input %q record[%d]: line = %v, want >= 1", input, i, line)
			}
		}
	}
}

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on I/O error.
// Exit code is unaffected.
func TestQuietSuppressesStderr(t *testing.T) {
	orig := readAll
	readAll = func(r io.Reader) ([]byte, error) { return nil, errors.New("simulated read error") }
	t.Cleanup(func() { readAll = orig })

	_, stderr, code := runLinks(t, []string{"--quiet"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (I/O error)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	stdout, stderr, code := runLinks(t, []string{"--quiet"}, "[example](https://example.com)")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	if !strings.Contains(stdout, "example.com") {
		t.Errorf("stdout = %q, want link record containing example.com", stdout)
	}
}
