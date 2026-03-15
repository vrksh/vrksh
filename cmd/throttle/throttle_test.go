package throttle

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// runThrottle replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and
// returns captured stdout, stderr, and exit code. Restores all globals via
// t.Cleanup. Sets sleepFn to a no-op so tests never block on rate delays.
// Do not call t.Parallel() — tests share OS-global state.
func runThrottle(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
	t.Helper()

	// No-op sleep so tests don't block on rate delays.
	origSleep := sleepFn
	sleepFn = func(time.Duration) {}
	t.Cleanup(func() { sleepFn = origSleep })

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

	os.Args = append([]string{"throttle"}, args...)
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

// errReader is an io.Reader that immediately returns an error, used to inject
// I/O failures into the scanner path.
type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("injected read error")
}

// lines splits stdout into non-empty lines.
func lines(s string) []string {
	var out []string
	for _, l := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if l != "" {
			out = append(out, l)
		}
	}
	return out
}

// --- Happy path ---

func TestHappyPath(t *testing.T) {
	stdout, stderr, code := runThrottle(t, []string{"--rate", "10/s"}, "a\nb\nc\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	got := lines(stdout)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("got %d lines, want %d; stdout=%q", len(got), len(want), stdout)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestExitCodeZero(t *testing.T) {
	_, _, code := runThrottle(t, []string{"--rate", "1/s"}, "hello\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestOrderPreserved(t *testing.T) {
	input := "line1\nline2\nline3\nline4\nline5\n"
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := lines(stdout)
	for i, want := range []string{"line1", "line2", "line3", "line4", "line5"} {
		if i >= len(got) || got[i] != want {
			t.Errorf("line[%d] = %q, want %q", i, func() string {
				if i < len(got) {
					return got[i]
				}
				return "<missing>"
			}(), want)
		}
	}
}

// --- Empty / blank input ---

func TestEmptyStdin(t *testing.T) {
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestEmptyLine(t *testing.T) {
	// echo '' sends a single newline → scanner produces one empty string → skipped.
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s"}, "\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (empty line is skipped)", stdout)
	}
}

func TestWhitespaceOnlyLine(t *testing.T) {
	// A line of spaces is content — it passes through unchanged.
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s"}, "   \n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "   " {
		t.Errorf("stdout = %q, want %q (whitespace-only line is content)", stdout, "   \n")
	}
}

// --- Usage errors (exit 2) ---

func TestMissingRate(t *testing.T) {
	_, stderr, code := runThrottle(t, nil, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--rate") {
		t.Errorf("stderr = %q, want mention of --rate", stderr)
	}
}

func TestRateZero(t *testing.T) {
	_, stderr, code := runThrottle(t, []string{"--rate", "0/s"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "must be > 0") {
		t.Errorf("stderr = %q, want 'must be > 0'", stderr)
	}
}

func TestRateZeroMin(t *testing.T) {
	_, _, code := runThrottle(t, []string{"--rate", "0/m"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestRateInvalidFormat(t *testing.T) {
	_, stderr, code := runThrottle(t, []string{"--rate", "abc"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "invalid rate format") {
		t.Errorf("stderr = %q, want 'invalid rate format'", stderr)
	}
}

func TestRateDecimalRejected(t *testing.T) {
	// 0.5/s — N is not a positive integer.
	_, stderr, code := runThrottle(t, []string{"--rate", "0.5/s"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "positive integer") {
		t.Errorf("stderr = %q, want 'positive integer'", stderr)
	}
}

func TestRateDecimalNotSameAsFormat(t *testing.T) {
	// Distinct message: 0.5/s is "not an integer", not "wrong format".
	_, stderr0, _ := runThrottle(t, []string{"--rate", "0.5/s"}, "")
	_, stderrAbc, _ := runThrottle(t, []string{"--rate", "abc"}, "")
	if stderr0 == stderrAbc {
		t.Errorf("decimal error and format error should produce distinct messages, both gave: %q", stderr0)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, code := runThrottle(t, []string{"--bogus"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// --- --help ---

func TestHelp(t *testing.T) {
	stdout, _, code := runThrottle(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "throttle") {
		t.Errorf("--help stdout = %q, want it to contain 'throttle'", stdout)
	}
}

// --- TTY guard ---

func TestInteractiveTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	stdout, stderr, code := runThrottle(t, []string{"--rate", "10/s"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on TTY error", stdout)
	}
	if !strings.Contains(stderr, "no input") {
		t.Errorf("stderr = %q, want 'no input'", stderr)
	}
}

func TestInteractiveTTYWithJSON(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	stdout, stderr, code := runThrottle(t, []string{"--rate", "10/s", "--json"}, "")
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

// --- TestJSONErrorToStdout: I/O error + --json → error JSON on stdout ---

func TestJSONErrorToStdout(t *testing.T) {
	origReader := stdinReader
	stdinReader = &errReader{}
	t.Cleanup(func() { stdinReader = origReader })

	stdout, stderr, code := runThrottle(t, []string{"--rate", "10/s", "--json"}, "")
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

func TestIOError(t *testing.T) {
	origReader := stdinReader
	stdinReader = &errReader{}
	t.Cleanup(func() { stdinReader = origReader })

	stdout, stderr, code := runThrottle(t, []string{"--rate", "10/s"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on error without --json", stdout)
	}
	if !strings.Contains(stderr, "error") {
		t.Errorf("stderr = %q, want error message", stderr)
	}
}

// --- --json metadata trailer ---

func TestJSONMetadata(t *testing.T) {
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s", "--json"}, "a\nb\nc\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := lines(stdout)
	// 3 data lines + 1 metadata = 4 lines total.
	if len(got) != 4 {
		t.Fatalf("got %d lines, want 4 (3 data + 1 metadata); stdout=%q", len(got), stdout)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(got[3]), &meta); err != nil {
		t.Fatalf("last line is not valid JSON: %v\ngot: %q", err, got[3])
	}
	if meta["_vrk"] != "throttle" {
		t.Errorf("_vrk = %v, want %q", meta["_vrk"], "throttle")
	}
	if meta["rate"] != "10/s" {
		t.Errorf("rate = %v, want %q", meta["rate"], "10/s")
	}
	if meta["lines"] != float64(3) {
		t.Errorf("lines = %v, want 3", meta["lines"])
	}
	if _, ok := meta["elapsed_ms"]; !ok {
		t.Error("metadata missing 'elapsed_ms' field")
	}
}

func TestJSONMetadataEmptyStdin(t *testing.T) {
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s", "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := lines(stdout)
	if len(got) != 1 {
		t.Fatalf("got %d lines, want 1 (only metadata); stdout=%q", len(got), stdout)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(got[0]), &meta); err != nil {
		t.Fatalf("not valid JSON: %v\ngot: %q", err, got[0])
	}
	if meta["lines"] != float64(0) {
		t.Errorf("lines = %v, want 0", meta["lines"])
	}
}

// --- --burst ---

func TestBurst(t *testing.T) {
	// With --burst 3 and 5 lines, all 5 lines should pass through.
	stdout, _, code := runThrottle(t, []string{"--rate", "2/s", "--burst", "3"}, "a\nb\nc\nd\ne\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := lines(stdout)
	if len(got) != 5 {
		t.Fatalf("got %d lines, want 5; stdout=%q", len(got), stdout)
	}
}

func TestBurstExceedsInput(t *testing.T) {
	// Burst larger than line count — all lines emit immediately, exit 0.
	stdout, _, code := runThrottle(t, []string{"--rate", "1/s", "--burst", "100"}, "a\nb\nc\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := lines(stdout)
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3; stdout=%q", len(got), stdout)
	}
}

func TestBurstZero(t *testing.T) {
	// --burst 0 is the same as no burst — all lines go through the rate limiter.
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s", "--burst", "0"}, "a\nb\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := lines(stdout)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2; stdout=%q", len(got), stdout)
	}
}

// --- --tokens-field ---

func TestTokensField(t *testing.T) {
	input := "{\"prompt\":\"hi\"}\n{\"prompt\":\"hello world\"}\n"
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s", "--tokens-field", "prompt"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := lines(stdout)
	if len(got) != 2 {
		t.Fatalf("got %d lines, want 2; stdout=%q", len(got), stdout)
	}
	// Lines are passed through verbatim.
	if got[0] != `{"prompt":"hi"}` {
		t.Errorf("line[0] = %q, want %q", got[0], `{"prompt":"hi"}`)
	}
}

func TestTokensFieldInvalidJSON(t *testing.T) {
	_, _, code := runThrottle(t, []string{"--rate", "10/s", "--tokens-field", "prompt"}, "not json\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (invalid JSON)", code)
	}
}

func TestTokensFieldMissing(t *testing.T) {
	_, _, code := runThrottle(t, []string{"--rate", "10/s", "--tokens-field", "prompt"}, "{\"other\":\"val\"}\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (field not found)", code)
	}
}

func TestTokensFieldBurstCountsLines(t *testing.T) {
	// --burst counts lines, not tokens.
	input := "{\"p\":\"hi\"}\n{\"p\":\"hello\"}\n{\"p\":\"world\"}\n"
	stdout, _, code := runThrottle(t, []string{"--rate", "1/s", "--tokens-field", "p", "--burst", "3"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := lines(stdout)
	if len(got) != 3 {
		t.Fatalf("got %d lines, want 3; stdout=%q", len(got), stdout)
	}
}

func TestTokensFieldEmptyValue(t *testing.T) {
	// Empty field value → 0 tokens → no delay, still emits the line.
	stdout, _, code := runThrottle(t, []string{"--rate", "10/s", "--tokens-field", "p"}, "{\"p\":\"\"}\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(lines(stdout)) != 1 {
		t.Errorf("stdout = %q, want 1 line", stdout)
	}
}

// --- parseRate table tests ---

func TestParseRate(t *testing.T) {
	cases := []struct {
		input   string
		wantErr bool
		wantVal float64
		wantMsg string // substring expected in error
	}{
		{"1/s", false, 1.0, ""},
		{"10/s", false, 10.0, ""},
		{"60/m", false, 1.0, ""},  // 60/60 = 1.0
		{"120/m", false, 2.0, ""}, // 120/60 = 2.0
		{"abc", true, 0, "invalid rate format"},
		{"1/hour", true, 0, "invalid rate format"},
		{"1/min", true, 0, "invalid rate format"},
		{"/s", true, 0, ""},
		{"0.5/s", true, 0, "positive integer"},
		{"1.0/s", true, 0, "positive integer"},
		{"0/s", true, 0, "must be > 0"},
		{"0/m", true, 0, "must be > 0"},
		{"-1/s", true, 0, ""},
	}
	for _, tc := range cases {
		t.Run(tc.input, func(t *testing.T) {
			val, err := parseRate(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("parseRate(%q) = %v, want error", tc.input, val)
				}
				if tc.wantMsg != "" && !strings.Contains(err.Error(), tc.wantMsg) {
					t.Errorf("parseRate(%q) error = %q, want to contain %q", tc.input, err.Error(), tc.wantMsg)
				}
			} else {
				if err != nil {
					t.Fatalf("parseRate(%q) unexpected error: %v", tc.input, err)
				}
				if val != tc.wantVal {
					t.Errorf("parseRate(%q) = %v, want %v", tc.input, val, tc.wantVal)
				}
			}
		})
	}
}

// --- Property test ---

func TestPropertyLinesPassThrough(t *testing.T) {
	inputs := []string{
		"hello\n",
		"a\nb\nc\n",
		"line with spaces\n",
		"  whitespace  \n",
		"\x00binary\n",
	}
	for _, input := range inputs {
		stdout, _, code := runThrottle(t, []string{"--rate", "100/s"}, input)
		if code != 0 {
			t.Errorf("input %q: exit code = %d, want 0", input, code)
			continue
		}
		// Every non-empty input line must appear verbatim in output.
		scanner := strings.NewReader(input)
		_ = scanner
		for _, want := range strings.Split(strings.TrimRight(input, "\n"), "\n") {
			if want == "" {
				continue
			}
			if !strings.Contains(stdout, want) {
				t.Errorf("input %q: output %q does not contain line %q", input, stdout, want)
			}
		}
	}
}

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on I/O error.
// Exit code is unaffected.
func TestQuietSuppressesStderr(t *testing.T) {
	orig := stdinReader
	stdinReader = &errReader{}
	t.Cleanup(func() { stdinReader = orig })

	_, stderr, code := runThrottle(t, []string{"--rate", "1000/s", "--quiet"}, "")
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
	stdout, stderr, code := runThrottle(t, []string{"--rate", "1000/s", "--quiet"}, "hello\nworld\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	if strings.Count(stdout, "\n") != 2 {
		t.Errorf("stdout = %q, want 2 lines", stdout)
	}
}
