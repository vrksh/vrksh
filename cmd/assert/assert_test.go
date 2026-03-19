package assert

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// runAssert replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runAssert(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
	t.Helper()

	origStdin := os.Stdin
	origStdout := os.Stdout
	origStderr := os.Stderr
	origArgs := os.Args
	origIsTerminal := isTerminal
	origReadAll := readAll

	t.Cleanup(func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
		os.Stderr = origStderr
		os.Args = origArgs
		isTerminal = origIsTerminal
		readAll = origReadAll
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

	os.Args = append([]string{"assert"}, args...)
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

// --- 1. Happy path ---

func TestJSONConditionPass(t *testing.T) {
	stdout, stderr, code := runAssert(t, []string{`.status == "ok"`}, `{"status":"ok"}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if strings.TrimSpace(stdout) != `{"status":"ok"}` {
		t.Errorf("stdout = %q, want input passed through", stdout)
	}
}

// --- 2. Exit 0 on success ---

func TestExitZeroOnSuccess(t *testing.T) {
	_, _, code := runAssert(t, []string{".x == 1"}, `{"x":1}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

// --- 3. Exit 1 on assertion failure ---

func TestExitOneOnFailure(t *testing.T) {
	stdout, stderr, code := runAssert(t, []string{`.status == "ok"`}, `{"status":"fail"}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on failure", stdout)
	}
	if !strings.Contains(stderr, "assertion failed") {
		t.Errorf("stderr = %q, want 'assertion failed'", stderr)
	}
}

// --- 4. Exit 1 on invalid JSON input ---

func TestExitOneOnInvalidJSON(t *testing.T) {
	_, stderr, code := runAssert(t, []string{`.field == "value"`}, "not json\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "not valid JSON") {
		t.Errorf("stderr = %q, want 'not valid JSON'", stderr)
	}
}

// --- 5. Exit 2 on no condition ---

func TestExitTwoNoCondition(t *testing.T) {
	_, stderr, code := runAssert(t, nil, `{"x":1}`+"\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "condition expression is required") {
		t.Errorf("stderr = %q, want 'condition expression is required'", stderr)
	}
}

// --- 6. Exit 2 on no stdin (TTY) ---

func TestExitTwoTTYNoStdin(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })
	_, stderr, code := runAssert(t, []string{".x == 1"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "no input") {
		t.Errorf("stderr = %q, want 'no input'", stderr)
	}
}

// --- 7. --help → exit 0 ---

func TestHelp(t *testing.T) {
	stdout, _, code := runAssert(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "assert") {
		t.Errorf("--help stdout = %q, want it to contain 'assert'", stdout)
	}
}

// --- 8. TTY + --json → exit 2 with JSON error on stdout ---

func TestTTYWithJSONFlag(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })
	stdout, stderr, code := runAssert(t, []string{"--json", ".x == 1"}, "")
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

// --- 9. --json + I/O error → JSON error on stdout, stderr empty ---

func TestJSONIOError(t *testing.T) {
	orig := readAll
	readAll = func(r io.Reader) ([]byte, error) {
		return nil, errors.New("simulated read error")
	}
	t.Cleanup(func() { readAll = orig })
	stdout, stderr, code := runAssert(t, []string{"--json", "--contains", "x"}, "")
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
}

// --- 10. Multiple conditions (AND logic) ---

func TestMultipleConditionsAllPass(t *testing.T) {
	stdout, _, code := runAssert(t, []string{".count > 0", ".errors == []"}, `{"count":5,"errors":[]}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != `{"count":5,"errors":[]}` {
		t.Errorf("stdout = %q, want passthrough", stdout)
	}
}

func TestMultipleConditionsFirstFails(t *testing.T) {
	_, stderr, code := runAssert(t, []string{".count > 0", ".errors == []"}, `{"count":0,"errors":[]}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, ".count > 0") {
		t.Errorf("stderr = %q, want failing condition shown", stderr)
	}
}

// --- 11. Numeric comparisons ---

func TestNumericGreaterThanPass(t *testing.T) {
	_, _, code := runAssert(t, []string{".score > 0.8"}, `{"score":0.91}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestNumericGreaterThanFail(t *testing.T) {
	_, _, code := runAssert(t, []string{".score > 0.8"}, `{"score":0.71}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestNumericLessThan(t *testing.T) {
	_, _, code := runAssert(t, []string{".x < 10"}, `{"x":5}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestNumericGreaterEqual(t *testing.T) {
	_, _, code := runAssert(t, []string{".x >= 5"}, `{"x":5}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestNumericLessEqual(t *testing.T) {
	_, _, code := runAssert(t, []string{".x <= 5"}, `{"x":5}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

// --- 12. Missing field == null ---

func TestMissingFieldIsNull(t *testing.T) {
	_, _, code := runAssert(t, []string{".missing == null"}, `{}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestMissingFieldNotNull(t *testing.T) {
	_, _, code := runAssert(t, []string{".missing != null"}, `{}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// --- 13. Array length via pipe operator ---

func TestArrayLengthPass(t *testing.T) {
	_, _, code := runAssert(t, []string{".items | length > 0"}, `{"items":[1,2,3]}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

func TestArrayLengthFail(t *testing.T) {
	_, _, code := runAssert(t, []string{".items | length > 0"}, `{"items":[]}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// --- 14. --contains pass and fail ---

func TestContainsPass(t *testing.T) {
	stdout, _, code := runAssert(t, []string{"--contains", "passed"}, "All tests passed\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "All tests passed" {
		t.Errorf("stdout = %q, want passthrough", stdout)
	}
}

func TestContainsFail(t *testing.T) {
	stdout, stderr, code := runAssert(t, []string{"--contains", "passed"}, "Some tests failed\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on failure", stdout)
	}
	if !strings.Contains(stderr, `does not contain`) {
		t.Errorf("stderr = %q, want 'does not contain'", stderr)
	}
}

// --- 15. --matches pass and fail ---

func TestMatchesPass(t *testing.T) {
	stdout, _, code := runAssert(t, []string{"--matches", "^OK:"}, "OK: all systems nominal\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "OK: all systems nominal" {
		t.Errorf("stdout = %q, want passthrough", stdout)
	}
}

func TestMatchesFail(t *testing.T) {
	_, stderr, code := runAssert(t, []string{"--matches", "^OK:"}, "ERROR: disk full\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, `does not match`) {
		t.Errorf("stderr = %q, want 'does not match'", stderr)
	}
}

// --- 16. --contains + --matches combined ---

func TestContainsPlusMatchesPass(t *testing.T) {
	stdout, _, code := runAssert(t, []string{"--contains", "passed", "--matches", "^OK:"}, "OK: all tests passed\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "OK: all tests passed" {
		t.Errorf("stdout = %q, want passthrough", stdout)
	}
}

func TestContainsPlusMatchesFail(t *testing.T) {
	_, _, code := runAssert(t, []string{"--contains", "passed", "--matches", "^OK:"}, "OK: some tests failed\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// --- 17. --message custom failure message format ---

func TestMessageFlag(t *testing.T) {
	_, stderr, code := runAssert(t, []string{`.status == "ok"`, "--message", "Bad API response"}, `{"status":"fail"}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "Bad API response") {
		t.Errorf("stderr = %q, want custom message", stderr)
	}
	if !strings.Contains(stderr, `.status == "ok"`) {
		t.Errorf("stderr = %q, want condition shown alongside message", stderr)
	}
}

// --- 18. --quiet suppresses stderr ---

func TestQuietSuppressesStderr(t *testing.T) {
	_, stderr, code := runAssert(t, []string{"--quiet", ".x == 2"}, `{"x":1}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// --- 19. JSONL: all lines pass → all pass through ---

func TestJSONLAllPass(t *testing.T) {
	input := "{\"ok\":true}\n{\"ok\":true}\n"
	stdout, _, code := runAssert(t, []string{".ok == true"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2; stdout=%q", len(lines), stdout)
	}
}

// --- 20. JSONL: failure on second line → first line passed, exit 1 ---

func TestJSONLFailSecondLine(t *testing.T) {
	input := "{\"ok\":true}\n{\"ok\":false}\n"
	stdout, _, code := runAssert(t, []string{".ok == true"}, input)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	// First line should have been written before failure.
	if !strings.Contains(stdout, `{"ok":true}`) {
		t.Errorf("stdout = %q, want first line passed through", stdout)
	}
	// Second line (the failing one) should not be in stdout.
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1 {
		t.Errorf("got %d lines, want 1 (only first line); stdout=%q", len(lines), stdout)
	}
}

// --- 21. Byte-for-byte transparency ---

func TestByteForByteTransparency(t *testing.T) {
	// Intentionally use non-canonical JSON formatting (extra spaces).
	input := `{"a":1,  "b":2}` + "\n"
	stdout, _, code := runAssert(t, []string{".a == 1"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// stdout must be the exact original bytes, not re-serialized JSON.
	if stdout != input {
		t.Errorf("stdout = %q, want exact input %q (byte-for-byte)", stdout, input)
	}
}

// --- 22. Positional args + --contains → exit 2 ---

func TestModeConflictContains(t *testing.T) {
	_, stderr, code := runAssert(t, []string{".x == 1", "--contains", "text"}, "text\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "cannot combine") || !strings.Contains(stderr, "--contains") {
		t.Errorf("stderr = %q, want mode conflict message", stderr)
	}
}

// --- 23. Positional args + --matches → exit 2 ---

func TestModeConflictMatches(t *testing.T) {
	_, stderr, code := runAssert(t, []string{".x == 1", "--matches", "^OK"}, "OK\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "cannot combine") || !strings.Contains(stderr, "--matches") {
		t.Errorf("stderr = %q, want mode conflict message", stderr)
	}
}

// --- 24. --json output shape for JSON mode pass, fail, and error ---

func TestJSONOutputPass(t *testing.T) {
	stdout, stderr, code := runAssert(t, []string{"--json", `.status == "ok"`}, `{"status":"ok"}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
	if obj["passed"] != true {
		t.Errorf("passed = %v, want true", obj["passed"])
	}
	if obj["condition"] != `.status == "ok"` {
		t.Errorf("condition = %v, want .status == \"ok\"", obj["condition"])
	}
	if _, ok := obj["input"]; !ok {
		t.Error("JSON mode pass should include 'input' field")
	}
}

func TestJSONOutputFail(t *testing.T) {
	stdout, stderr, code := runAssert(t, []string{"--json", `.status == "ok"`}, `{"status":"fail"}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
	if obj["passed"] != false {
		t.Errorf("passed = %v, want false", obj["passed"])
	}
	// "message" should be absent when --message not set.
	if _, ok := obj["message"]; ok {
		t.Errorf("'message' should be absent when --message not set, got %v", obj["message"])
	}
}

func TestJSONOutputFailWithMessage(t *testing.T) {
	stdout, _, code := runAssert(t, []string{"--json", `.status == "ok"`, "-m", "Bad status"}, `{"status":"fail"}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
	if obj["message"] != "Bad status" {
		t.Errorf("message = %v, want 'Bad status'", obj["message"])
	}
}

// --- 25. --json output shape for plain text mode ---

func TestJSONPlainTextContainsPass(t *testing.T) {
	stdout, _, code := runAssert(t, []string{"--json", "--contains", "passed"}, "All tests passed\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
	if obj["passed"] != true {
		t.Errorf("passed = %v, want true", obj["passed"])
	}
	if obj["condition"] != "--contains: passed" {
		t.Errorf("condition = %v, want '--contains: passed'", obj["condition"])
	}
	// No "input" field in plain text mode.
	if _, ok := obj["input"]; ok {
		t.Error("plain text mode should not include 'input' field")
	}
}

func TestJSONPlainTextContainsFail(t *testing.T) {
	stdout, _, code := runAssert(t, []string{"--json", "--contains", "passed"}, "Some tests failed\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
	if obj["passed"] != false {
		t.Errorf("passed = %v, want false", obj["passed"])
	}
	if _, ok := obj["message"]; !ok {
		t.Error("plain text fail should include 'message' field")
	}
}

func TestJSONPlainTextMatchesPass(t *testing.T) {
	stdout, _, code := runAssert(t, []string{"--json", "--matches", "^OK:"}, "OK: nominal\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
	if obj["condition"] != "--matches: ^OK:" {
		t.Errorf("condition = %v, want '--matches: ^OK:'", obj["condition"])
	}
}

// --- 26. --json with JSONL: one result per line ---

func TestJSONWithJSONL(t *testing.T) {
	input := "{\"ok\":true}\n{\"ok\":false}\n"
	stdout, stderr, code := runAssert(t, []string{"--json", ".ok == true"}, input)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json", stderr)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2; stdout=%q", len(lines), stdout)
	}
	// First line: passed
	var first map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &first); err != nil {
		t.Fatalf("line 0 not JSON: %v", err)
	}
	if first["passed"] != true {
		t.Errorf("line 0 passed = %v, want true", first["passed"])
	}
	// Second line: failed
	var second map[string]any
	if err := json.Unmarshal([]byte(lines[1]), &second); err != nil {
		t.Fatalf("line 1 not JSON: %v", err)
	}
	if second["passed"] != false {
		t.Errorf("line 1 passed = %v, want false", second["passed"])
	}
}

// --- 27. Property test ---

func TestPropertySelfEquality(t *testing.T) {
	inputs := []string{
		`{"x":1}`,
		`{"x":"hello"}`,
		`{"x":null}`,
		`{"x":true}`,
		`{"x":false}`,
		`{"x":[1,2,3]}`,
		`{"x":{"nested":true}}`,
	}
	for _, input := range inputs {
		_, _, code := runAssert(t, []string{".x == .x"}, input+"\n")
		if code != 0 {
			t.Errorf("input %q: .x == .x should always pass, got exit %d", input, code)
		}
	}
}

// --- 28. Non-boolean gojq truthiness ---

func TestTruthyNonBoolean(t *testing.T) {
	// String is truthy.
	_, _, code := runAssert(t, []string{".status"}, `{"status":"ok"}`+"\n")
	if code != 0 {
		t.Fatalf("string should be truthy, got exit %d", code)
	}
}

func TestFalsyNull(t *testing.T) {
	_, _, code := runAssert(t, []string{".missing"}, `{}`+"\n")
	if code != 1 {
		t.Fatalf("null should be falsy, got exit %d", code)
	}
}

func TestFalsyFalse(t *testing.T) {
	_, _, code := runAssert(t, []string{".x"}, `{"x":false}`+"\n")
	if code != 1 {
		t.Fatalf("false should be falsy, got exit %d", code)
	}
}

func TestTruthyNumber(t *testing.T) {
	_, _, code := runAssert(t, []string{".x"}, `{"x":42}`+"\n")
	if code != 0 {
		t.Fatalf("number should be truthy, got exit %d", code)
	}
}

func TestTruthyZero(t *testing.T) {
	// In jq, 0 is truthy (unlike most languages).
	_, _, code := runAssert(t, []string{".x"}, `{"x":0}`+"\n")
	if code != 0 {
		t.Fatalf("0 should be truthy in jq, got exit %d", code)
	}
}

// --- Additional edge cases ---

func TestUnknownFlag(t *testing.T) {
	_, _, code := runAssert(t, []string{"--bogus"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestInvalidRegex(t *testing.T) {
	_, stderr, code := runAssert(t, []string{"--matches", "[invalid"}, "test\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (bad regex)", code)
	}
	if !strings.Contains(stderr, "invalid") {
		t.Errorf("stderr = %q, want invalid regex message", stderr)
	}
}

func TestStderrMessageFormat(t *testing.T) {
	_, stderr, code := runAssert(t, []string{`.status == "ok"`}, `{"status":"fail"}`+"\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	// Stderr must start with "assert: assertion failed:"
	if !strings.HasPrefix(strings.TrimSpace(stderr), "assert: assertion failed:") {
		t.Errorf("stderr = %q, want prefix 'assert: assertion failed:'", stderr)
	}
}

func TestMessageFormatWithCustomMessage(t *testing.T) {
	_, stderr, _ := runAssert(t, []string{`.status == "ok"`, "-m", "Bad API response"}, `{"status":"fail"}`+"\n")
	want := `assert: assertion failed: Bad API response (.status == "ok")`
	if !strings.Contains(stderr, want) {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestContainsStderrFormat(t *testing.T) {
	_, stderr, code := runAssert(t, []string{"--contains", "passed"}, "Some tests failed\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, `does not contain "passed"`) {
		t.Errorf("stderr = %q, want contains failure message", stderr)
	}
}

func TestMatchesStderrFormat(t *testing.T) {
	_, stderr, code := runAssert(t, []string{"--matches", "^OK:"}, "ERROR: disk full\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, `does not match`) {
		t.Errorf("stderr = %q, want matches failure message", stderr)
	}
}
