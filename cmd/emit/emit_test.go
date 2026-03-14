package emit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// runEmit replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and exit code. Restores all globals via t.Cleanup.
// Do not call t.Parallel() — tests share os.Stdin/Stdout/Stderr global state.
//
// isTerminal and stdinReader are NOT saved here — tests that need to mock them
// must do so before calling runEmit and register their own t.Cleanup.
func runEmit(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"emit"}, args...)
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

// parseJSONL parses newline-delimited JSON from s and returns each non-empty
// line as a map. Fails the test if any line is not valid JSON.
func parseJSONL(t *testing.T, s string) []map[string]interface{} {
	t.Helper()
	var records []map[string]interface{}
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		if line == "" {
			continue
		}
		var m map[string]interface{}
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("output line is not valid JSON: %v\nline: %q", err, line)
		}
		records = append(records, m)
	}
	return records
}

// errReader is an io.Reader that always returns an error, used to inject I/O
// failures into the scanner path.
type errReader struct{}

func (e *errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("injected read error")
}

// --- Basic ---

func TestSingleLine(t *testing.T) {
	stdout, _, code := runEmit(t, nil, "Starting job\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	rec := records[0]
	if rec["level"] != "info" {
		t.Errorf("level = %v, want %q", rec["level"], "info")
	}
	if rec["msg"] != "Starting job" {
		t.Errorf("msg = %v, want %q", rec["msg"], "Starting job")
	}
	ts, ok := rec["ts"].(string)
	if !ok || !strings.HasSuffix(ts, "Z") || len(ts) != 24 {
		t.Errorf("ts = %v, want 24-char RFC3339 UTC string ending in Z (e.g. 2006-01-02T15:04:05.000Z)", rec["ts"])
	}
	if _, hasTag := rec["tag"]; hasTag {
		t.Errorf("tag must not be present when --tag is not set, got %v", rec["tag"])
	}
}

func TestStderrEmptyOnSuccess(t *testing.T) {
	_, stderr, code := runEmit(t, nil, "hello\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty on success", stderr)
	}
}

// --- --level / -l ---

func TestLevelFlag(t *testing.T) {
	stdout, _, code := runEmit(t, []string{"--level", "error"}, "Job failed\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["level"] != "error" {
		t.Errorf("level = %v, want %q", records[0]["level"], "error")
	}
}

func TestLevelFlagShort(t *testing.T) {
	stdout, _, code := runEmit(t, []string{"-l", "warn"}, "low memory\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["level"] != "warn" {
		t.Errorf("level = %v, want %q", records[0]["level"], "warn")
	}
}

func TestLevelDebug(t *testing.T) {
	stdout, _, code := runEmit(t, []string{"--level", "debug"}, "verbose\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["level"] != "debug" {
		t.Errorf("level = %v, want %q", records[0]["level"], "debug")
	}
}

func TestLevelInvalid(t *testing.T) {
	_, stderr, code := runEmit(t, []string{"--level", "bad"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage error, got empty")
	}
}

func TestLevelCaseInsensitive(t *testing.T) {
	// --level accepts uppercase input and normalises to lowercase
	stdout, _, code := runEmit(t, []string{"--level", "ERROR"}, "boom\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if records[0]["level"] != "error" {
		t.Errorf("level = %v, want %q", records[0]["level"], "error")
	}
}

// --- --tag ---

func TestTagFlag(t *testing.T) {
	stdout, _, code := runEmit(t, []string{"--tag", "agent-run"}, "Job failed\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	rec := records[0]
	if rec["tag"] != "agent-run" {
		t.Errorf("tag = %v, want %q", rec["tag"], "agent-run")
	}
	// Verify field order: ts < level < tag < msg in the raw output line.
	line := strings.TrimRight(stdout, "\n")
	tsIdx := strings.Index(line, `"ts"`)
	levelIdx := strings.Index(line, `"level"`)
	tagIdx := strings.Index(line, `"tag"`)
	msgIdx := strings.Index(line, `"msg"`)
	if tsIdx >= levelIdx || levelIdx >= tagIdx || tagIdx >= msgIdx {
		t.Errorf("field order wrong in: %s", line)
	}
}

func TestTagEmpty(t *testing.T) {
	// --tag "" must omit the tag field entirely.
	stdout, _, code := runEmit(t, []string{"--tag", ""}, "hello\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if _, hasTag := records[0]["tag"]; hasTag {
		t.Errorf("tag must be omitted when --tag is empty, got %v", records[0]["tag"])
	}
}

// --- --msg ---

func TestMsgOverridePlainStdin(t *testing.T) {
	// Plain-text stdin with --msg: message overridden, no extra fields added.
	stdout, _, code := runEmit(t, []string{"--msg", "Job failed"}, "plain text line\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	rec := records[0]
	if rec["msg"] != "Job failed" {
		t.Errorf("msg = %v, want %q", rec["msg"], "Job failed")
	}
	// Only core fields: no extra keys from the plain-text stdin line.
	for k := range rec {
		if k != "ts" && k != "level" && k != "msg" {
			t.Errorf("unexpected key %q in record (plain stdin should not merge)", k)
		}
	}
}

func TestMsgOverrideJSONStdin(t *testing.T) {
	// JSON stdin with --msg: message overridden, JSON fields merged into record.
	stdout, _, code := runEmit(t, []string{"--level", "error", "--msg", "Job failed"}, `{"job_id":"abc"}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	rec := records[0]
	if rec["msg"] != "Job failed" {
		t.Errorf("msg = %v, want %q", rec["msg"], "Job failed")
	}
	if rec["level"] != "error" {
		t.Errorf("level = %v, want %q", rec["level"], "error")
	}
	if rec["job_id"] != "abc" {
		t.Errorf("job_id = %v, want %q", rec["job_id"], "abc")
	}
}

func TestMsgJSONFieldOrder(t *testing.T) {
	// Extra merged fields must be emitted alphabetically after msg.
	stdout, _, code := runEmit(t, []string{"--msg", "done"}, `{"zoo":"z","alpha":"a","middle":"m"}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	line := strings.TrimRight(stdout, "\n")
	alphaIdx := strings.Index(line, `"alpha"`)
	middleIdx := strings.Index(line, `"middle"`)
	zooIdx := strings.Index(line, `"zoo"`)
	msgIdx := strings.Index(line, `"msg"`)
	if alphaIdx < 0 || middleIdx < 0 || zooIdx < 0 {
		t.Fatalf("expected extra fields in output, got: %s", line)
	}
	if alphaIdx >= middleIdx || middleIdx >= zooIdx {
		t.Errorf("extra fields not in alphabetical order: %s", line)
	}
	if msgIdx >= alphaIdx {
		t.Errorf("extra fields should appear after msg: %s", line)
	}
}

func TestMsgCoreFieldsWin(t *testing.T) {
	// Core field names in stdin JSON must not override the flag-set values.
	stdin := `{"msg":"overridden","level":"debug","ts":"2020","tag":"x","extra":"val"}` + "\n"
	stdout, _, code := runEmit(t, []string{"--msg", "mine", "--level", "info"}, stdin)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	rec := records[0]
	if rec["msg"] != "mine" {
		t.Errorf("msg = %v, want %q (--msg flag wins over stdin JSON)", rec["msg"], "mine")
	}
	if rec["level"] != "info" {
		t.Errorf("level = %v, want %q (--level flag wins over stdin JSON)", rec["level"], "info")
	}
	if rec["extra"] != "val" {
		t.Errorf("extra = %v, want %q (non-core field must be merged)", rec["extra"], "val")
	}
	ts, _ := rec["ts"].(string)
	if ts == "2020" {
		t.Errorf("ts must not be overwritten by stdin JSON, got %q", ts)
	}
}

// --- --parse-level ---

func TestParseLevelKnownPrefixes(t *testing.T) {
	cases := []struct {
		input string
		level string
		msg   string
	}{
		{"ERROR: disk full", "error", "disk full"},
		{"error: disk full", "error", "disk full"},
		{"WARN: low memory", "warn", "low memory"},
		{"warn: low memory", "warn", "low memory"},
		{"WARNING: out of space", "warn", "out of space"},
		{"INFO: started", "info", "started"},
		{"DEBUG: connecting", "debug", "connecting"},
	}
	for _, tc := range cases {
		stdout, _, code := runEmit(t, []string{"--parse-level"}, tc.input+"\n")
		if code != 0 {
			t.Errorf("input %q: exit code = %d, want 0", tc.input, code)
			continue
		}
		records := parseJSONL(t, stdout)
		if len(records) != 1 {
			t.Errorf("input %q: got %d records, want 1", tc.input, len(records))
			continue
		}
		if records[0]["level"] != tc.level {
			t.Errorf("input %q: level = %v, want %q", tc.input, records[0]["level"], tc.level)
		}
		if records[0]["msg"] != tc.msg {
			t.Errorf("input %q: msg = %v, want %q", tc.input, records[0]["msg"], tc.msg)
		}
	}
}

func TestParseLevelStrip(t *testing.T) {
	// Prefix + optional colon + leading whitespace are stripped from msg.
	cases := []struct {
		input, msg string
	}{
		{"ERROR: colon space", "colon space"},
		{"WARN space only", "space only"},
		{"INFO:no_space", "no_space"},
		{"DEBUG  double_space", "double_space"},
		{"ERROR", ""},
	}
	for _, tc := range cases {
		stdout, _, code := runEmit(t, []string{"--parse-level"}, tc.input+"\n")
		if code != 0 {
			t.Errorf("input %q: exit code = %d, want 0", tc.input, code)
			continue
		}
		if tc.msg == "" && stdout == "" {
			// Empty msg after stripping is a valid edge; line was non-empty pre-strip.
			continue
		}
		records := parseJSONL(t, stdout)
		if len(records) != 1 {
			t.Errorf("input %q: got %d records, want 1", tc.input, len(records))
			continue
		}
		if records[0]["msg"] != tc.msg {
			t.Errorf("input %q: msg = %v, want %q", tc.input, records[0]["msg"], tc.msg)
		}
	}
}

func TestParseLevelWordBoundary(t *testing.T) {
	// "DEBUGGER:" must NOT match the DEBUG prefix — only word-boundary match.
	stdout, _, code := runEmit(t, []string{"--parse-level"}, "DEBUGGER: something\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	// Should fall back to default level "info", msg unchanged.
	if records[0]["level"] != "info" {
		t.Errorf("level = %v, want %q (DEBUGGER must not match DEBUG prefix)", records[0]["level"], "info")
	}
	if records[0]["msg"] != "DEBUGGER: something" {
		t.Errorf("msg = %v, want %q (msg must be unchanged for non-matching prefix)", records[0]["msg"], "DEBUGGER: something")
	}
}

func TestParseLevelUnknownPrefix(t *testing.T) {
	// Unknown prefix falls back to --level value (default "info").
	stdout, _, code := runEmit(t, []string{"--parse-level"}, "[ERROR] crash\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["level"] != "info" {
		t.Errorf("level = %v, want %q (unknown prefix → default level)", records[0]["level"], "info")
	}
	if records[0]["msg"] != "[ERROR] crash" {
		t.Errorf("msg = %v, want %q (msg unchanged for unknown prefix)", records[0]["msg"], "[ERROR] crash")
	}
}

func TestParseLevelWithLevelFlag(t *testing.T) {
	// Known prefix overrides --level value.
	stdout, _, code := runEmit(t, []string{"--parse-level", "--level", "warn"}, "ERROR: boom\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["level"] != "error" {
		t.Errorf("level = %v, want %q (ERROR prefix overrides --level warn)", records[0]["level"], "error")
	}
	if records[0]["msg"] != "boom" {
		t.Errorf("msg = %v, want %q", records[0]["msg"], "boom")
	}

	// Unknown prefix falls back to --level warn (not hardcoded "info").
	stdout, _, code = runEmit(t, []string{"--parse-level", "--level", "warn"}, "something\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records = parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["level"] != "warn" {
		t.Errorf("level = %v, want %q (unknown prefix → --level warn, not hardcoded info)", records[0]["level"], "warn")
	}
}

func TestParseLevelWithMsg(t *testing.T) {
	// --parse-level detects level; --msg overrides the message.
	stdout, _, code := runEmit(t, []string{"--parse-level", "--msg", "override"}, "ERROR: actual message\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["level"] != "error" {
		t.Errorf("level = %v, want %q (parse-level still detects from raw line)", records[0]["level"], "error")
	}
	if records[0]["msg"] != "override" {
		t.Errorf("msg = %v, want %q (--msg takes over)", records[0]["msg"], "override")
	}
}

// --- empty lines ---

func TestEmptyLinesSkipped(t *testing.T) {
	// Multiple empty lines produce no output.
	stdout, _, code := runEmit(t, nil, "\n\n\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (all empty lines must be skipped)", stdout)
	}
}

func TestEmptyStdin(t *testing.T) {
	// Empty stdin (zero bytes) → exit 0, no output.
	stdout, _, code := runEmit(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestMixedEmptyAndNonEmpty(t *testing.T) {
	// Empty lines interspersed with real lines: only real lines produce records.
	stdout, _, code := runEmit(t, nil, "\nfirst\n\nsecond\n\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2 (empty lines skipped)", len(records))
	}
	if records[0]["msg"] != "first" {
		t.Errorf("records[0].msg = %v, want %q", records[0]["msg"], "first")
	}
	if records[1]["msg"] != "second" {
		t.Errorf("records[1].msg = %v, want %q", records[1]["msg"], "second")
	}
}

// --- multi-line ---

func TestMultiLine(t *testing.T) {
	stdout, _, code := runEmit(t, nil, "first\nsecond\nthird\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}
	if records[0]["msg"] != "first" {
		t.Errorf("records[0].msg = %v, want %q", records[0]["msg"], "first")
	}
	if records[2]["msg"] != "third" {
		t.Errorf("records[2].msg = %v, want %q", records[2]["msg"], "third")
	}
}

// --- interactive TTY ---

func TestInteractiveTTY(t *testing.T) {
	// When stdin appears to be a terminal (no positional arg), emit must exit 2.
	orig := isTerminal
	isTerminal = func(_ int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	_, stderr, code := runEmit(t, nil, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error for interactive stdin)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain error message, got empty")
	}
}

// --- positional arg ---

func TestPositionalArg(t *testing.T) {
	// Positional arg is treated as a single input line; piped stdin is ignored.
	stdout, _, code := runEmit(t, []string{"Starting job"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["msg"] != "Starting job" {
		t.Errorf("msg = %v, want %q", records[0]["msg"], "Starting job")
	}
}

func TestPositionalArgWithFlags(t *testing.T) {
	// Positional arg works with other flags.
	stdout, _, code := runEmit(t, []string{"--level", "error", "--tag", "myapp", "crash"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["level"] != "error" {
		t.Errorf("level = %v, want %q", records[0]["level"], "error")
	}
	if records[0]["tag"] != "myapp" {
		t.Errorf("tag = %v, want %q", records[0]["tag"], "myapp")
	}
	if records[0]["msg"] != "crash" {
		t.Errorf("msg = %v, want %q", records[0]["msg"], "crash")
	}
}

// --- usage errors ---

func TestUnknownFlag(t *testing.T) {
	_, stderr, code := runEmit(t, []string{"--unknown-flag"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage error, got empty")
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runEmit(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "emit") {
		t.Errorf("--help stdout = %q: missing tool name", stdout)
	}
	if !strings.Contains(stdout, "--level") {
		t.Errorf("--help stdout missing --level flag")
	}
}

// --- I/O error ---

func TestIOError(t *testing.T) {
	// Injected scanner error → exit 1, error message on stderr.
	orig := stdinReader
	stdinReader = &errReader{}
	t.Cleanup(func() { stdinReader = orig })

	_, stderr, code := runEmit(t, nil, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (I/O error)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain error message on I/O error, got empty")
	}
}

// --- property tests ---

func TestPropertyValidJSON(t *testing.T) {
	// For any input, every non-empty stdout line must be valid JSON.
	inputs := []string{
		"hello world\n",
		"line1\nline2\nline3\n",
		"\n\nreal line\n\n",
		`{"already":"json"}` + "\n",
		"unicode: 日本語\n",
		"ERROR: disk full\n",
	}
	for _, input := range inputs {
		stdout, _, _ := runEmit(t, nil, input)
		for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
			if line == "" {
				continue
			}
			var m interface{}
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				t.Errorf("input %q: output line not valid JSON: %v\nline: %q", input, err, line)
			}
		}
	}
}

func TestPropertyExitCodesOnly(t *testing.T) {
	// Any valid invocation must produce exit code 0, 1, or 2 — never anything else.
	inputs := []string{
		"hello\n",
		"",
		"\n\n",
		"ERROR: something\n",
		`{"key":"val"}` + "\n",
	}
	for _, input := range inputs {
		_, _, code := runEmit(t, nil, input)
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("input %q: exit code = %d, want 0, 1, or 2", input, code)
		}
	}
}

// --- large integer precision ---

func TestLargeIntegerMerge(t *testing.T) {
	// Large integers in merged JSON must not lose precision via float64 conversion.
	stdout, _, code := runEmit(t, []string{"--msg", "done"}, `{"id":9007199254740993}`+"\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "9007199254740993") {
		t.Errorf("large integer was not preserved in output: %q", stdout)
	}
}
