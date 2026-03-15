package pct

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// runPct replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runPct(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"pct"}, args...)
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

// parseJSON unmarshals a single JSON object from a string.
func parseJSON(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(s)), &m); err != nil {
		t.Fatalf("invalid JSON %q: %v", s, err)
	}
	return m
}

// --- Happy path ---

func TestHappyPathEncode(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--encode"}, "hello world & more\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "hello%20world%20%26%20more\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestHappyPathDecode(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--decode"}, "hello%20world%20%26%20more\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "hello world & more\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestRoundTrip(t *testing.T) {
	original := "hello world & more\n"
	encoded, _, code := runPct(t, []string{"--encode"}, original)
	if code != 0 {
		t.Fatalf("encode exit code = %d, want 0", code)
	}
	decoded, _, code := runPct(t, []string{"--decode"}, encoded)
	if code != 0 {
		t.Fatalf("decode exit code = %d, want 0", code)
	}
	if decoded != original {
		t.Errorf("round-trip: got %q, want %q", decoded, original)
	}
}

// --- Form encoding ---

func TestFormEncode(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--encode", "--form"}, "hello world\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "hello+world\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestFormDecode(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--decode", "--form"}, "hello+world\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "hello world\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// --- Edge cases ---

func TestDoubleEncode(t *testing.T) {
	// Encoding an already-encoded string encodes the % — correct behaviour.
	stdout, _, code := runPct(t, []string{"--encode"}, "hello%20world\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "hello%2520world\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestPercentTwoBDecode(t *testing.T) {
	// %2B decodes to + in non-form mode.
	stdout, _, code := runPct(t, []string{"--decode"}, "hello%2Bworld\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "hello+world\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestPlusLiteralNonForm(t *testing.T) {
	// + is a literal character in non-form decode — not converted to space.
	stdout, _, code := runPct(t, []string{"--decode"}, "hello+world\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "hello+world\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q (+ must stay literal in non-form mode)", stdout, want)
	}
}

func TestEmptyStdin(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--encode"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestMultilineInput(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--encode"}, "a b\nc d\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "a%20b\nc%20d\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestPositionalArg(t *testing.T) {
	// Positional arg bypasses stdin — TTY guard must not fire.
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runPct(t, []string{"--encode", "hello world"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "hello%20world\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// --- Usage errors (exit 2) ---

func TestNoModeFlag(t *testing.T) {
	_, stderr, code := runPct(t, nil, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--encode") || !strings.Contains(stderr, "--decode") {
		t.Errorf("stderr = %q, want mention of --encode and --decode", stderr)
	}
}

func TestBothFlags(t *testing.T) {
	_, _, code := runPct(t, []string{"--encode", "--decode"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, code := runPct(t, []string{"--encode", "--bogus"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "pct") {
		t.Errorf("--help stdout = %q, want it to contain 'pct'", stdout)
	}
}

func TestInteractiveTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runPct(t, []string{"--encode"}, "")
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

func TestInteractiveTTYWithJSONFlag(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runPct(t, []string{"--encode", "--json"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json active", stderr)
	}
	obj := parseJSON(t, stdout)
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	if obj["code"] != float64(2) {
		t.Errorf("JSON code = %v, want 2", obj["code"])
	}
}

// --- Runtime errors (exit 1) ---

func TestInvalidPercentSequence(t *testing.T) {
	_, _, code := runPct(t, []string{"--decode"}, "%ZZ\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestTruncatedSequence(t *testing.T) {
	_, _, code := runPct(t, []string{"--decode"}, "%\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	origReadAll := readAll
	readAll = func(r io.Reader) ([]byte, error) {
		return nil, errors.New("simulated read error")
	}
	defer func() { readAll = origReadAll }()

	stdout, stderr, code := runPct(t, []string{"--decode", "--json"}, "any input")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json active", stderr)
	}
	obj := parseJSON(t, stdout)
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	if obj["code"] != float64(1) {
		t.Errorf("JSON code = %v, want 1", obj["code"])
	}
}

// --- JSON output ---

func TestJSONEncode(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--encode", "--json"}, "hello world\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	obj := parseJSON(t, stdout)
	if obj["input"] != "hello world" {
		t.Errorf("input = %q, want %q", obj["input"], "hello world")
	}
	if obj["output"] != "hello%20world" {
		t.Errorf("output = %q, want %q", obj["output"], "hello%20world")
	}
	if obj["op"] != "encode" {
		t.Errorf("op = %q, want %q", obj["op"], "encode")
	}
	if obj["mode"] != "percent" {
		t.Errorf("mode = %q, want %q", obj["mode"], "percent")
	}
}

func TestJSONFormDecode(t *testing.T) {
	stdout, _, code := runPct(t, []string{"--decode", "--form", "--json"}, "hello+world\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	obj := parseJSON(t, stdout)
	if obj["mode"] != "form" {
		t.Errorf("mode = %q, want %q", obj["mode"], "form")
	}
	if obj["op"] != "decode" {
		t.Errorf("op = %q, want %q", obj["op"], "decode")
	}
	if obj["output"] != "hello world" {
		t.Errorf("output = %q, want %q", obj["output"], "hello world")
	}
}

func TestJSONMultiline(t *testing.T) {
	// Multiline with --json emits one JSON object per line (JSONL).
	stdout, _, code := runPct(t, []string{"--encode", "--json"}, "a b\nc d\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d JSON lines, want 2; stdout=%q", len(lines), stdout)
	}
	var first, second map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 1 not valid JSON: %v", err)
	}
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("line 2 not valid JSON: %v", err)
	}
	if first["input"] != "a b" {
		t.Errorf("line 1 input = %q, want %q", first["input"], "a b")
	}
	if second["input"] != "c d" {
		t.Errorf("line 2 input = %q, want %q", second["input"], "c d")
	}
}

// --- Unicode ---

func TestUnicodeRoundTrip(t *testing.T) {
	input := "é 你好\n"
	encoded, _, code := runPct(t, []string{"--encode"}, input)
	if code != 0 {
		t.Fatalf("encode exit code = %d, want 0", code)
	}
	decoded, _, code := runPct(t, []string{"--decode"}, encoded)
	if code != 0 {
		t.Fatalf("decode exit code = %d, want 0", code)
	}
	if decoded != input {
		t.Errorf("unicode round-trip: got %q, want %q", decoded, input)
	}
}

// --- Property test ---

// TestPropertyRoundTrip verifies decode(encode(s)) == s for a variety of inputs.
func TestPropertyRoundTrip(t *testing.T) {
	cases := []string{
		"hello world",
		"& = ? # / + %",
		"https://example.com/path?q=1&r=2#fragment",
		"colon:slash/at@bang!dollar$amp&",
		"tilde~dot.dash-underscore_",
	}
	for _, c := range cases {
		encoded, _, code := runPct(t, []string{"--encode"}, c+"\n")
		if code != 0 {
			t.Fatalf("encode %q: exit code = %d, want 0", c, code)
		}
		decoded, _, code := runPct(t, []string{"--decode"}, encoded)
		if code != 0 {
			t.Fatalf("decode %q: exit code = %d, want 0", c, code)
		}
		got := strings.TrimSuffix(decoded, "\n")
		if got != c {
			t.Errorf("round-trip %q: got %q", c, got)
		}
	}
}
