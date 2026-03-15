package recase

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// runRecase replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runRecase(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"recase"}, args...)
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

func TestHappyPath(t *testing.T) {
	cases := []struct {
		input string
		to    string
		want  string
	}{
		// snake as input
		{"hello_world", "camel", "helloWorld"},
		{"hello_world", "pascal", "HelloWorld"},
		{"hello_world", "kebab", "hello-world"},
		{"hello_world", "screaming", "HELLO_WORLD"},
		{"hello_world", "title", "Hello World"},
		{"hello_world", "lower", "hello world"},
		{"hello_world", "upper", "HELLO WORLD"},
		{"hello_world", "snake", "hello_world"},
		// camel as input
		{"helloWorld", "snake", "hello_world"},
		{"helloWorld", "kebab", "hello-world"},
		// pascal as input
		{"HelloWorld", "snake", "hello_world"},
		// kebab as input
		{"hello-world", "camel", "helloWorld"},
		// title as input
		{"Hello World", "snake", "hello_world"},
		// screaming as input
		{"HELLO_WORLD", "camel", "helloWorld"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input+"→"+tc.to, func(t *testing.T) {
			stdout, _, code := runRecase(t, []string{"--to", tc.to}, tc.input+"\n")
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			got := strings.TrimRight(stdout, "\n")
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestExitCodes(t *testing.T) {
	// exit 0: success
	_, _, code := runRecase(t, []string{"--to", "camel"}, "hello_world\n")
	if code != 0 {
		t.Errorf("success: exit code = %d, want 0", code)
	}

	// exit 2: --to missing
	_, _, code = runRecase(t, nil, "hello_world\n")
	if code != 2 {
		t.Errorf("missing --to: exit code = %d, want 2", code)
	}

	// exit 2: unknown --to value
	_, _, code = runRecase(t, []string{"--to", "bogus"}, "hello_world\n")
	if code != 2 {
		t.Errorf("unknown --to: exit code = %d, want 2", code)
	}

	// exit 2: unknown flag
	_, _, code = runRecase(t, []string{"--bogus"}, "hello_world\n")
	if code != 2 {
		t.Errorf("unknown flag: exit code = %d, want 2", code)
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runRecase(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("--help exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "recase") {
		t.Errorf("--help stdout = %q, want it to contain 'recase'", stdout)
	}
}

func TestTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runRecase(t, []string{"--to", "camel"}, "")
	if code != 2 {
		t.Fatalf("TTY exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("TTY stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "no input") {
		t.Errorf("TTY stderr = %q, want message containing 'no input'", stderr)
	}
}

func TestTTYWithJSON(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runRecase(t, []string{"--to", "camel", "--json"}, "")
	if code != 2 {
		t.Fatalf("TTY+json exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("TTY+json stderr = %q, want empty when --json active", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v; got %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	if obj["code"] != float64(2) {
		t.Errorf("JSON code = %v, want 2", obj["code"])
	}
}

// TestJSONErrorToStdout verifies that --json + I/O error sends the error JSON to
// stdout (not stderr), stderr stays empty, and exit code is 1.
func TestJSONErrorToStdout(t *testing.T) {
	origScanLines := scanLines
	scanLines = func(r io.Reader, fn func(string) error) error {
		return errors.New("simulated read error")
	}
	defer func() { scanLines = origScanLines }()

	stdout, stderr, code := runRecase(t, []string{"--to", "camel", "--json"}, "hello\n")
	if code != 1 {
		t.Fatalf("I/O error+json exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("I/O error+json stderr = %q, want empty when --json active", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v; got %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	if obj["code"] != float64(1) {
		t.Errorf("JSON code = %v, want 1", obj["code"])
	}
}

func TestEmptyStdin(t *testing.T) {
	stdout, _, code := runRecase(t, []string{"--to", "camel"}, "")
	if code != 0 {
		t.Fatalf("empty stdin exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("empty stdin stdout = %q, want empty", stdout)
	}
}

// TestEmptyLinePreserved verifies that a blank line in produces a blank line out —
// silent skipping would corrupt line counts in batch pipelines.
func TestEmptyLinePreserved(t *testing.T) {
	stdout, _, code := runRecase(t, []string{"--to", "camel"}, "\n")
	if code != 0 {
		t.Fatalf("empty line exit code = %d, want 0", code)
	}
	if stdout != "\n" {
		t.Errorf("empty line stdout = %q, want %q (one blank line)", stdout, "\n")
	}
}

func TestAcronyms(t *testing.T) {
	cases := []struct {
		input, to, want string
	}{
		{"userID", "snake", "user_id"},
		{"parseHTML", "snake", "parse_html"},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.input+"→"+tc.to, func(t *testing.T) {
			stdout, _, code := runRecase(t, []string{"--to", tc.to}, tc.input+"\n")
			if code != 0 {
				t.Fatalf("exit code = %d, want 0", code)
			}
			got := strings.TrimRight(stdout, "\n")
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestSplitWordsConsecutiveAcronyms documents the known limitation: two consecutive
// acronyms with no separator cannot be split — the entire uppercase run becomes one word.
// "getHTTPSURL" → "get_httpsurl", not "get_https_url".
// Workaround: use "getHTTPS_URL" or "get-https-url" as input.
func TestSplitWordsConsecutiveAcronyms(t *testing.T) {
	stdout, _, code := runRecase(t, []string{"--to", "snake"}, "getHTTPSURL\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := strings.TrimRight(stdout, "\n")
	if got != "get_httpsurl" {
		t.Errorf("getHTTPSURL→snake: got %q, want %q (documented limitation)", got, "get_httpsurl")
	}
}

func TestMultilineBatch(t *testing.T) {
	stdout, _, code := runRecase(t, []string{"--to", "camel"}, "hello_world\nfoo_bar\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2; stdout=%q", len(lines), stdout)
	}
	if lines[0] != "helloWorld" {
		t.Errorf("line[0] = %q, want %q", lines[0], "helloWorld")
	}
	if lines[1] != "fooBar" {
		t.Errorf("line[1] = %q, want %q", lines[1], "fooBar")
	}
}

func TestJSONOutput(t *testing.T) {
	stdout, _, code := runRecase(t, []string{"--to", "camel", "--json"}, "hello_world\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v; got %q", err, stdout)
	}
	if obj["input"] != "hello_world" {
		t.Errorf("input = %q, want %q", obj["input"], "hello_world")
	}
	if obj["output"] != "helloWorld" {
		t.Errorf("output = %q, want %q", obj["output"], "helloWorld")
	}
	if obj["from"] != "snake" {
		t.Errorf("from = %q, want %q", obj["from"], "snake")
	}
	if obj["to"] != "camel" {
		t.Errorf("to = %q, want %q", obj["to"], "camel")
	}
}

func TestQuiet(t *testing.T) {
	// --quiet on success: stderr stays empty, exit 0.
	_, stderr, code := runRecase(t, []string{"--to", "camel", "--quiet"}, "hello_world\n")
	if code != 0 {
		t.Fatalf("success+quiet: exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("success+quiet stderr = %q, want empty", stderr)
	}

	// --quiet on usage error: exit 2, stderr suppressed.
	_, stderr, code = runRecase(t, []string{"--quiet"}, "hello_world\n")
	if code != 2 {
		t.Fatalf("missing --to+quiet: exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("missing --to+quiet stderr = %q, want empty", stderr)
	}
}

// TestPositionalArgSingle verifies that a single positional arg is accepted
// without piped stdin, even when stdin is a TTY.
func TestPositionalArgSingle(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runRecase(t, []string{"--to", "kebab", "hello_world"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := strings.TrimRight(stdout, "\n")
	if got != "hello-world" {
		t.Errorf("got %q, want %q", got, "hello-world")
	}
}

// TestPositionalArgMultiple verifies that multiple positional args are each
// treated as one input line, producing one output line per arg.
func TestPositionalArgMultiple(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runRecase(t, []string{"--to", "camel", "hello_world", "foo_bar"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2; stdout=%q", len(lines), stdout)
	}
	if lines[0] != "helloWorld" {
		t.Errorf("lines[0] = %q, want %q", lines[0], "helloWorld")
	}
	if lines[1] != "fooBar" {
		t.Errorf("lines[1] = %q, want %q", lines[1], "fooBar")
	}
}

// TestProperty verifies the round-trip invariant: for any identifier-friendly
// convention, joining words and splitting back produces the original word list.
func TestProperty(t *testing.T) {
	words := []string{"hello", "world", "foo"}
	conventions := []string{"camel", "pascal", "snake", "kebab", "screaming"}
	for _, conv := range conventions {
		conv := conv
		t.Run(conv, func(t *testing.T) {
			joined, err := joinWords(words, conv)
			if err != nil {
				t.Fatalf("joinWords(%v, %s): %v", words, conv, err)
			}
			got, _ := splitWords(joined)
			if len(got) != len(words) {
				t.Fatalf("splitWords(joinWords(%v, %s)) = %v, want %v", words, conv, got, words)
			}
			for i, w := range words {
				if got[i] != w {
					t.Errorf("word[%d] = %q, want %q (conv=%s, joined=%q)", i, got[i], w, conv, joined)
				}
			}
		})
	}
}
