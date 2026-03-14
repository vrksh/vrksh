package mask

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// runMask replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and exit code. Restores all globals via t.Cleanup.
// Do not call t.Parallel() — tests share os.Stdin/Stdout/Stderr global state.
func runMask(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"mask"}, args...)
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

// errReader is an io.Reader that always returns an error on Read.
type errReader struct{ err error }

func (e *errReader) Read(_ []byte) (int, error) { return 0, e.err }

// --- Built-in patterns ---

func TestBuiltinBearer(t *testing.T) {
	stdout, _, code := runMask(t, nil, "Authorization: Bearer sk-ant-abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "Authorization: Bearer [REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBuiltinPassword(t *testing.T) {
	stdout, _, code := runMask(t, nil, "password=hunter2\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "password=[REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBuiltinSecret(t *testing.T) {
	stdout, _, code := runMask(t, nil, "secret=abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "secret=[REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBuiltinApiKey(t *testing.T) {
	stdout, _, code := runMask(t, nil, "api_key=abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "api_key=[REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBuiltinToken(t *testing.T) {
	stdout, _, code := runMask(t, nil, "token=abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "token=[REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// --- Entropy ---

func TestEntropyDefaultThreshold(t *testing.T) {
	// zXpNrT5wYbsH8cDgQmVk — 20 unique chars, H = log2(20) ≈ 4.32 > 4.0.
	// No builtin pattern matches, so entropy detection must fire.
	stdout, _, code := runMask(t, nil, "zXpNrT5wYbsH8cDgQmVk\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "[REDACTED]") {
		t.Errorf("stdout = %q: expected [REDACTED] for high-entropy token", stdout)
	}
}

func TestEntropyFlagLower(t *testing.T) {
	// sk-ant-AAABBBCCC111222333444555 has entropy ≈ 3.6 — redacted at --entropy 3.0
	// but not at default 4.0. No builtin pattern matches this input either.
	stdout, _, code := runMask(t, []string{"--entropy", "3.0"}, "sk-ant-AAABBBCCC111222333444555\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "[REDACTED]") {
		t.Errorf("stdout = %q: expected [REDACTED] at --entropy 3.0", stdout)
	}
}

func TestEntropyFlagHigher(t *testing.T) {
	// abc123 is 6 chars — below the 8-char token floor, never entropy-checked.
	// With --entropy 4.5 and no builtin match, output is unchanged.
	stdout, _, code := runMask(t, []string{"--entropy", "4.5"}, "abc123\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "[REDACTED]") {
		t.Errorf("stdout = %q: expected no redaction for short token below length floor", stdout)
	}
}

// --- Custom patterns ---

func TestCustomPatternSingle(t *testing.T) {
	stdout, _, code := runMask(t, []string{"--pattern", `sk-ant-[A-Za-z0-9]+`}, "key: sk-ant-AAAA\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "key: [REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestCustomPatternRepeated(t *testing.T) {
	// Two distinct --pattern flags both fire; both regex strings appear in patterns_matched.
	stdout, _, code := runMask(t,
		[]string{"--pattern", `PAT1-[A-Za-z]+`, "--pattern", `PAT2-[A-Za-z]+`, "--json"},
		"value: PAT1-AAABBB and PAT2-CCCDDD\n",
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 lines (text + metadata), got %d: %q", len(lines), stdout)
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &meta); err != nil {
		t.Fatalf("last line not valid JSON: %v\nline: %q", err, lines[len(lines)-1])
	}
	pm, ok := meta["patterns_matched"].([]interface{})
	if !ok {
		t.Fatalf("patterns_matched is not array: %T %v", meta["patterns_matched"], meta["patterns_matched"])
	}
	var names []string
	for _, v := range pm {
		if s, ok := v.(string); ok {
			names = append(names, s)
		}
	}
	hasPAT1, hasPAT2 := false, false
	for _, n := range names {
		if n == `PAT1-[A-Za-z]+` {
			hasPAT1 = true
		}
		if n == `PAT2-[A-Za-z]+` {
			hasPAT2 = true
		}
	}
	if !hasPAT1 || !hasPAT2 {
		t.Errorf("patterns_matched = %v, want both PAT1-[A-Za-z]+ and PAT2-[A-Za-z]+", names)
	}
}

func TestInvalidPatternExitsBeforeStdin(t *testing.T) {
	// An invalid regex must produce exit 2 before any stdin is consumed.
	_, stderr, code := runMask(t, []string{"--pattern", "[bad"}, "should not be read\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage error message, got empty")
	}
}

// --- Multi-line input ---

func TestMultiLine(t *testing.T) {
	// Only the line with a secret should change; clean lines pass through unchanged.
	input := "line1\nBearer abc123xyz\nline3\n"
	stdout, _, code := runMask(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	parts := strings.Split(stdout, "\n")
	// parts[0]="line1", parts[1]="Bearer [REDACTED]", parts[2]="line3", parts[3]=""
	if len(parts) < 4 {
		t.Fatalf("expected 4 output parts (3 lines + trailing empty), got %d: %q", len(parts), stdout)
	}
	if parts[0] != "line1" {
		t.Errorf("line 0 = %q, want %q", parts[0], "line1")
	}
	if parts[1] != "Bearer [REDACTED]" {
		t.Errorf("line 1 = %q, want %q", parts[1], "Bearer [REDACTED]")
	}
	if parts[2] != "line3" {
		t.Errorf("line 2 = %q, want %q", parts[2], "line3")
	}
}

func TestUnchangedInput(t *testing.T) {
	// Text with no secrets passes through byte-for-byte identical, exit 0.
	input := "no secrets here\n"
	stdout, _, code := runMask(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != input {
		t.Errorf("stdout = %q, want %q (unchanged)", stdout, input)
	}
}

// --- Empty line ---

func TestEmptyLine(t *testing.T) {
	// echo '' outputs a single newline; mask should echo it back unchanged.
	stdout, _, code := runMask(t, nil, "\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "\n" {
		t.Errorf("stdout = %q, want %q (empty line preserved)", stdout, "\n")
	}
}

// --- --json flag ---

func TestJSONNormalShape(t *testing.T) {
	stdout, _, code := runMask(t, []string{"--json"}, "Authorization: Bearer sk-ant-abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 lines (text + metadata), got %d: %q", len(lines), stdout)
	}
	// First line: redacted text output.
	if !strings.Contains(lines[0], "[REDACTED]") {
		t.Errorf("first line = %q: expected [REDACTED]", lines[0])
	}
	// Last line: JSON metadata envelope.
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &meta); err != nil {
		t.Fatalf("last line not valid JSON: %v\nline: %q", err, lines[len(lines)-1])
	}
	if meta["_vrk"] != "mask" {
		t.Errorf("_vrk = %v, want %q", meta["_vrk"], "mask")
	}
	if meta["lines"] != float64(1) {
		t.Errorf("lines = %v, want 1", meta["lines"])
	}
	if meta["redacted"] != float64(1) {
		t.Errorf("redacted = %v, want 1", meta["redacted"])
	}
	if _, ok := meta["patterns_matched"]; !ok {
		t.Error("metadata missing patterns_matched field")
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	// Inject an I/O error via stdinReader. With --json:
	//   stdout: {"error":"...","code":1}
	//   stderr: empty
	//   Run() returns 1
	origReader := stdinReader
	stdinReader = &errReader{err: errors.New("injected read error")}
	t.Cleanup(func() { stdinReader = origReader })

	stdout, stderr, code := runMask(t, []string{"--json"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty (errors go to stdout with --json active)", stderr)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Fatal("stdout is empty, want JSON error envelope")
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
		t.Fatalf("stdout not valid JSON: %v\nstdout: %q", err, stdout)
	}
	if m["error"] == nil {
		t.Error("JSON envelope missing 'error' field")
	}
	if m["code"] != float64(1) {
		t.Errorf("JSON envelope code = %v, want 1", m["code"])
	}
}

// --- Usage errors ---

func TestInteractiveTTY(t *testing.T) {
	// Simulate TTY by overriding the isTerminal var to always return true.
	origTTY := isTerminal
	isTerminal = func(_ int) bool { return true }
	t.Cleanup(func() { isTerminal = origTTY })

	_, stderr, code := runMask(t, nil, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (TTY → usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage error message, got empty")
	}
}

func TestUnknownFlag(t *testing.T) {
	_, stderr, code := runMask(t, []string{"--unknown-flag"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage error message, got empty")
	}
}

// --- Property tests ---

func TestPropertyExitCodesOnly(t *testing.T) {
	// Any valid invocation must produce exit code 0, 1, or 2 — never anything else.
	cases := []struct {
		args  []string
		input string
	}{
		{nil, "no secrets here\n"},
		{nil, "Bearer abc123xyz\n"},
		{nil, ""},
		{[]string{"--json"}, "hello world\n"},
		{[]string{"--entropy", "2.0"}, "zXpNrT5wYbsH8cDgQmVk\n"},
	}
	for _, tc := range cases {
		_, _, code := runMask(t, tc.args, tc.input)
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("args=%v input=%q: exit code = %d, want 0, 1, or 2", tc.args, tc.input, code)
		}
	}
}

func TestPropertyRedactedNeverLeaksOriginal(t *testing.T) {
	// After redaction, the original secret value must not appear anywhere in stdout.
	secret := "sk-ant-SuperSecretValue123"
	input := "Authorization: Bearer " + secret + "\n"
	stdout, _, _ := runMask(t, nil, input)
	if strings.Contains(stdout, secret) {
		t.Errorf("stdout contains original secret %q: %q", secret, stdout)
	}
}
