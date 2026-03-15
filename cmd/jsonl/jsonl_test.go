package jsonl

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// runJsonl replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runJsonl(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"jsonl"}, args...)
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

// errorReader is a reader that always returns an error.
type errorReader struct{ err error }

func (e *errorReader) Read(_ []byte) (int, error) { return 0, e.err }

// --- Positional argument ---

func TestPositionalArgSplit(t *testing.T) {
	// Positional arg bypasses stdin entirely — TTY guard must not fire.
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runJsonl(t, []string{`[{"a":1}]`}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != `{"a":1}` {
		t.Errorf("stdout = %q, want %q", strings.TrimSpace(stdout), `{"a":1}`)
	}
}

// TestPositionalArgCollectOutOfScope documents that collect is a streaming mode
// and positional-arg collect is intentionally not supported.
// (Collect reads line-by-line; a positional arg would be a single JSON value
// on one line, which works trivially but adds no value over stdin.)

// --- Split mode (JSON array → JSONL) ---

func TestHappyPathSplitObjects(t *testing.T) {
	stdout, _, code := runJsonl(t, nil, `[{"a":1},{"b":2},{"c":3}]`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3; stdout=%q", len(lines), stdout)
	}
	for i, line := range lines {
		var v map[string]any
		if err := json.Unmarshal([]byte(line), &v); err != nil {
			t.Errorf("line %d is not valid JSON: %v", i+1, err)
		}
	}
}

func TestHappyPathSplitPrimitives(t *testing.T) {
	stdout, _, code := runJsonl(t, nil, "[1,2,3]\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3; stdout=%q", len(lines), stdout)
	}
	if lines[0] != "1" || lines[1] != "2" || lines[2] != "3" {
		t.Errorf("unexpected output lines: %v", lines)
	}
}

func TestHappyPathEmptyArray(t *testing.T) {
	stdout, _, code := runJsonl(t, nil, "[]\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty for empty array", stdout)
	}
}

// --- Collect mode (JSONL → JSON array) ---

func TestHappyPathCollect(t *testing.T) {
	input := "{\"a\":1}\n{\"b\":2}\n{\"c\":3}\n"
	stdout, _, code := runJsonl(t, []string{"--collect"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var arr []map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &arr); err != nil {
		t.Fatalf("output is not a valid JSON array: %v\ngot: %q", err, stdout)
	}
	if len(arr) != 3 {
		t.Fatalf("got %d elements, want 3", len(arr))
	}
}

func TestCollectEmpty(t *testing.T) {
	stdout, _, code := runJsonl(t, []string{"--collect"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "[]" {
		t.Errorf("stdout = %q, want []", strings.TrimSpace(stdout))
	}
}

func TestCollectShortFlag(t *testing.T) {
	stdout, _, code := runJsonl(t, []string{"-c"}, "{\"x\":1}\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "[") {
		t.Errorf("stdout = %q, want a JSON array", stdout)
	}
}

// --- Exit codes ---

func TestExitZeroOnSuccess(t *testing.T) {
	_, _, code := runJsonl(t, nil, "[1]\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestExitOneInvalidJSON(t *testing.T) {
	_, stderr, code := runJsonl(t, nil, "not json\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid JSON") {
		t.Errorf("stderr = %q, want 'invalid JSON'", stderr)
	}
}

func TestExitOneNonArray(t *testing.T) {
	_, stderr, code := runJsonl(t, nil, `{"not":"array"}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "not a JSON array") {
		t.Errorf("stderr = %q, want 'not a JSON array'", stderr)
	}
	if !strings.Contains(stderr, "--collect") {
		t.Errorf("stderr = %q, want '--collect' hint", stderr)
	}
}

func TestExitOneInvalidLineCollect(t *testing.T) {
	input := "{\"a\":1}\nnot-json\n{\"b\":2}\n"
	_, stderr, code := runJsonl(t, []string{"--collect"}, input)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "line 2") {
		t.Errorf("stderr = %q, want 'line 2'", stderr)
	}
}

func TestExitTwoUnknownFlag(t *testing.T) {
	_, _, code := runJsonl(t, []string{"--bogus"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// --- TTY guard ---

func TestExitTwoInteractiveTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runJsonl(t, nil, "")
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

func TestExitTwoTTYWithJSON(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runJsonl(t, []string{"--json"}, "")
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

// TestJSONErrorToStdout checks that when --json is active and a read error
// occurs, the error goes to stdout as JSON, stderr stays empty, and exit code
// is 1.
func TestJSONErrorToStdout(t *testing.T) {
	origReader := stdinReader
	stdinReader = &errorReader{err: errors.New("simulated read error")}
	defer func() { stdinReader = origReader }()

	stdout, stderr, code := runJsonl(t, []string{"--json"}, "some input")
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

// --- --help ---

func TestHelp(t *testing.T) {
	stdout, _, code := runJsonl(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "jsonl") {
		t.Errorf("--help stdout = %q, want it to contain 'jsonl'", stdout)
	}
}

// --- Empty stdin ---

func TestEmptyStdinSplit(t *testing.T) {
	// echo '' produces \n — decoder skips whitespace and hits EOF, same as zero bytes.
	stdout, _, code := runJsonl(t, nil, "\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 for newline-only stdin", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}

	// printf '' produces zero bytes.
	stdout2, _, code2 := runJsonl(t, nil, "")
	if code2 != 0 {
		t.Fatalf("exit code = %d, want 0 for empty stdin", code2)
	}
	if stdout2 != "" {
		t.Errorf("stdout = %q, want empty", stdout2)
	}
}

// --- --json flag ---

func TestJSONTrailerSplit(t *testing.T) {
	stdout, _, code := runJsonl(t, []string{"--json"}, `[{"a":1},{"b":2}]`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	// 2 records + 1 metadata = 3 lines.
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3 (2 records + metadata); stdout=%q", len(lines), stdout)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(lines[2]), &meta); err != nil {
		t.Fatalf("last line is not valid JSON: %v\ngot: %q", err, lines[2])
	}
	if meta["_vrk"] != "jsonl" {
		t.Errorf("_vrk = %q, want %q", meta["_vrk"], "jsonl")
	}
	if meta["count"] != float64(2) {
		t.Errorf("count = %v, want 2", meta["count"])
	}
}

func TestJSONEmptyArrayTrailer(t *testing.T) {
	stdout, _, code := runJsonl(t, []string{"--json"}, "[]\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// Only the metadata line — no records.
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1 (metadata only); stdout=%q", len(lines), stdout)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &meta); err != nil {
		t.Fatalf("line is not valid JSON: %v\ngot: %q", err, lines[0])
	}
	if meta["count"] != float64(0) {
		t.Errorf("count = %v, want 0", meta["count"])
	}
}

// TestJSONNoOpCollect verifies that --json is a no-op in collect mode.
// Appending a trailer after ] would produce invalid JSON.
func TestJSONNoOpCollect(t *testing.T) {
	input := "{\"a\":1}\n{\"b\":2}\n"
	stdout1, _, code1 := runJsonl(t, []string{"--collect", "--json"}, input)
	stdout2, _, code2 := runJsonl(t, []string{"--collect"}, input)
	if code1 != 0 {
		t.Fatalf("--collect --json exit code = %d, want 0", code1)
	}
	if code2 != 0 {
		t.Fatalf("--collect exit code = %d, want 0", code2)
	}
	if stdout1 != stdout2 {
		t.Errorf("--json is not a no-op in collect mode:\n  with --json: %q\n  without: %q", stdout1, stdout2)
	}
}

// --- Property tests ---

// TestPropertyAllLinesValidJSON checks that every line emitted by split mode
// is valid JSON, across a variety of input types.
func TestPropertyAllLinesValidJSON(t *testing.T) {
	inputs := []string{
		`[{"a":1},{"b":"hello"}]`,
		`[1,2,3]`,
		`["x","y","z"]`,
		`[true,false,null]`,
		`[{"nested":{"key":"val"}}]`,
	}
	for _, input := range inputs {
		stdout, _, code := runJsonl(t, nil, input+"\n")
		if code != 0 {
			t.Errorf("input %q: exit code = %d, want 0", input, code)
			continue
		}
		for i, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
			if line == "" {
				continue
			}
			var v any
			if err := json.Unmarshal([]byte(line), &v); err != nil {
				t.Errorf("input %q line %d: invalid JSON %q: %v", input, i+1, line, err)
			}
		}
	}
}

// TestPropertyRoundTrip verifies that split then collect yields a structurally
// equal result. Uses re-parsed comparison — not byte equality — because
// json.Marshal sorts object keys alphabetically.
func TestPropertyRoundTrip(t *testing.T) {
	inputs := []string{
		`[{"a":1},{"b":2}]`,
		`[1,2,3]`,
		`["x","y"]`,
	}
	for _, input := range inputs {
		// Split pass.
		split, _, code1 := runJsonl(t, nil, input+"\n")
		if code1 != 0 {
			t.Errorf("input %q: split exit code = %d, want 0", input, code1)
			continue
		}
		// Collect pass.
		collected, _, code2 := runJsonl(t, []string{"--collect"}, split)
		if code2 != 0 {
			t.Errorf("input %q: collect exit code = %d, want 0", input, code2)
			continue
		}
		// Re-parse and re-marshal both to normalise representation.
		var orig, roundtrip any
		if err := json.Unmarshal([]byte(input), &orig); err != nil {
			t.Fatalf("could not parse original %q: %v", input, err)
		}
		if err := json.Unmarshal([]byte(strings.TrimSpace(collected)), &roundtrip); err != nil {
			t.Fatalf("could not parse round-trip output %q: %v", strings.TrimSpace(collected), err)
		}
		origBytes, _ := json.Marshal(orig)
		rtBytes, _ := json.Marshal(roundtrip)
		if string(origBytes) != string(rtBytes) {
			t.Errorf("input %q: round-trip mismatch\n  orig: %s\n  got:  %s", input, origBytes, rtBytes)
		}
	}
}
