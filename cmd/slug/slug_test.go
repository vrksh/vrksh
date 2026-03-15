package slug

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

// runSlug replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runSlug(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"slug"}, args...)
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

// Test 1 & 2: happy path + exit 0.
func TestHappyPath(t *testing.T) {
	stdout, _, code := runSlug(t, nil, "Hello World\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "hello-world" {
		t.Errorf("stdout = %q, want %q", strings.TrimRight(stdout, "\n"), "hello-world")
	}
}

// Test 3: exit 1 on I/O error (plain stderr path).
func TestIOError(t *testing.T) {
	orig := scanLines
	scanLines = func(r io.Reader, fn func(string) error) error {
		return errors.New("simulated read error")
	}
	defer func() { scanLines = orig }()

	_, stderr, code := runSlug(t, nil, "input")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "error") {
		t.Errorf("stderr = %q, want error message", stderr)
	}
}

// Test 4: exit 2 on unknown flag.
func TestUnknownFlag(t *testing.T) {
	_, _, code := runSlug(t, []string{"--bogus"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// Test 5: --help → exit 0, stdout contains "slug".
func TestHelp(t *testing.T) {
	stdout, _, code := runSlug(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "slug") {
		t.Errorf("--help stdout = %q, want it to contain 'slug'", stdout)
	}
}

// Test 6: interactive TTY → exit 2.
func TestInteractiveTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runSlug(t, nil, "")
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

// Test 7: interactive TTY + --json → exit 2, error JSON to stdout, stderr empty.
func TestInteractiveTTYWithJSON(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runSlug(t, []string{"--json"}, "")
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

// Test 8: --json + injected I/O error → error JSON to stdout, stderr empty, exit 1.
func TestJSONErrorToStdout(t *testing.T) {
	orig := scanLines
	scanLines = func(r io.Reader, fn func(string) error) error {
		return errors.New("simulated read error")
	}
	defer func() { scanLines = orig }()

	stdout, stderr, code := runSlug(t, []string{"--json"}, "input")
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

// Test 9: empty stdin (non-TTY) → exit 0, no output.
func TestEmptyStdin(t *testing.T) {
	stdout, _, code := runSlug(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// Test 10: property test — any non-empty slug output matches the slug pattern.
func TestPropertySlugShape(t *testing.T) {
	re := regexp.MustCompile(`^[a-z0-9]+(-[a-z0-9]+)*$`)
	inputs := []string{
		"Hello World\n",
		"Hello, World!\n",
		"Hello World (2026)\n",
		"Ünïcödé Héró\n",
		"hello--world\n",
		"  hello world  \n",
		"A very long title\n",
		"foo_bar_baz\n",
		"123 abc\n",
	}
	for _, input := range inputs {
		stdout, _, code := runSlug(t, nil, input)
		if code != 0 {
			t.Errorf("input %q: exit code = %d, want 0", input, code)
			continue
		}
		line := strings.TrimRight(stdout, "\n")
		if line == "" {
			// empty slug is valid (e.g. all-punctuation input)
			continue
		}
		if !re.MatchString(line) {
			t.Errorf("input %q: output %q does not match slug pattern", input, line)
		}
	}
}

// Test 11: --separator _ override.
func TestSeparatorOverride(t *testing.T) {
	stdout, _, code := runSlug(t, []string{"--separator", "_"}, "Hello World\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "hello_world" {
		t.Errorf("stdout = %q, want %q", strings.TrimRight(stdout, "\n"), "hello_world")
	}
}

// Test 12: --max word boundary truncation.
func TestMaxWordBoundary(t *testing.T) {
	stdout, _, code := runSlug(t, []string{"--max", "12"}, "A very long title\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "a-very-long" {
		t.Errorf("stdout = %q, want %q", strings.TrimRight(stdout, "\n"), "a-very-long")
	}
}

// Test 13: --max with no word boundary in range → empty output, exit 0.
func TestMaxNoBoundary(t *testing.T) {
	stdout, _, code := runSlug(t, []string{"--max", "3"}, "abcdefghij\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty (no word boundary in range)", stdout)
	}
}

// Test 14: unicode normalisation.
func TestUnicodeNormalisation(t *testing.T) {
	stdout, _, code := runSlug(t, nil, "Ünïcödé Héró\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "unicode-hero" {
		t.Errorf("stdout = %q, want %q", strings.TrimRight(stdout, "\n"), "unicode-hero")
	}
}

// Test 15: multiline batch — two input lines → two output slugs.
func TestMultilineBatch(t *testing.T) {
	stdout, _, code := runSlug(t, nil, "Hello World\nFoo Bar\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d output lines, want 2; stdout=%q", len(lines), stdout)
	}
	if lines[0] != "hello-world" {
		t.Errorf("line[0] = %q, want %q", lines[0], "hello-world")
	}
	if lines[1] != "foo-bar" {
		t.Errorf("line[1] = %q, want %q", lines[1], "foo-bar")
	}
}

// Test 16: positional arg form — works without stdin, even with TTY set.
func TestPositionalArg(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runSlug(t, []string{"Hello World"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "hello-world" {
		t.Errorf("stdout = %q, want %q", strings.TrimRight(stdout, "\n"), "hello-world")
	}
}

// Extra: multiple positional args → one slug per arg.
func TestMultiplePositionalArgs(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runSlug(t, []string{"Hello World", "Foo Bar"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2; stdout=%q", len(lines), stdout)
	}
	if lines[0] != "hello-world" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "hello-world")
	}
	if lines[1] != "foo-bar" {
		t.Errorf("lines[1] = %q, want %q", lines[1], "foo-bar")
	}
}

// Extra: --json output shape.
func TestJSONOutput(t *testing.T) {
	stdout, _, code := runSlug(t, []string{"--json"}, "Hello World\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v; got %q", err, stdout)
	}
	if obj["input"] != "Hello World" {
		t.Errorf("input = %q, want %q", obj["input"], "Hello World")
	}
	if obj["output"] != "hello-world" {
		t.Errorf("output = %q, want %q", obj["output"], "hello-world")
	}
}

// Extra: empty input produces no output (empty string slug is suppressed).
func TestAllPunctuationNoOutput(t *testing.T) {
	stdout, _, code := runSlug(t, nil, "!!!\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "" {
		t.Errorf("stdout = %q, want empty for all-punctuation input", stdout)
	}
}

// Extra: consecutive separators collapsed.
func TestConsecutiveSeparatorsCollapsed(t *testing.T) {
	stdout, _, code := runSlug(t, nil, "hello--world\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "hello-world" {
		t.Errorf("stdout = %q, want %q", strings.TrimRight(stdout, "\n"), "hello-world")
	}
}

// Extra: --quiet suppresses stderr on I/O error.
func TestQuietSuppressesStderr(t *testing.T) {
	orig := scanLines
	scanLines = func(r io.Reader, fn func(string) error) error {
		return errors.New("simulated read error")
	}
	defer func() { scanLines = orig }()

	_, stderr, code := runSlug(t, []string{"--quiet"}, "input")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}
