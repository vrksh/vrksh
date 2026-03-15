package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// runValidate replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and
// returns captured stdout, stderr, and exit code. Restores all globals via
// t.Cleanup. Do not call t.Parallel() — tests share os.Stdin/Stdout/Stderr
// and the fixFn/newStdinReader package-level vars.
func runValidate(t *testing.T, stdinContent string, args []string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"validate"}, args...)
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

// errorReader always returns the given error on Read — used to simulate I/O
// errors in the scanner path without a real file descriptor.
type errorReader struct{ err error }

func (r *errorReader) Read([]byte) (int, error) { return 0, r.err }

// parseJSONLines splits s into non-empty lines and unmarshals each as JSON.
func parseJSONLines(t *testing.T, s string) []map[string]interface{} {
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

// --- Usage errors ---

func TestNoSchema(t *testing.T) {
	_, stderr, code := runValidate(t, "", nil)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if !strings.Contains(stderr, "--schema") {
		t.Errorf("stderr should mention --schema, got: %q", stderr)
	}
}

func TestInvalidSchemaJSON(t *testing.T) {
	_, stderr, code := runValidate(t, "", []string{"--schema", `{"bad json`})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "invalid schema JSON") {
		t.Errorf("stderr should mention 'invalid schema JSON', got: %q", stderr)
	}
}

func TestInvalidSchemaType(t *testing.T) {
	_, stderr, code := runValidate(t, "", []string{"--schema", `{"name":"colour"}`})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "invalid schema type") {
		t.Errorf("stderr should mention 'invalid schema type', got: %q", stderr)
	}
}

func TestFileSchema_NotFound(t *testing.T) {
	_, stderr, code := runValidate(t, "", []string{"--schema", "/no/such/schema.json"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "cannot read schema file") {
		t.Errorf("stderr should mention 'cannot read schema file', got: %q", stderr)
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runValidate(t, "", []string{"--help"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "usage:") {
		t.Errorf("--help stdout should contain 'usage:', got: %q", stdout)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, code := runValidate(t, "", []string{"--schema", `{"a":"string"}`, "--bogus"})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// --- Empty input ---

func TestEmptyStdin(t *testing.T) {
	stdout, _, code := runValidate(t, "", []string{"--schema", `{"name":"string"}`})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty, got: %q", stdout)
	}
}

func TestEmptyStdin_JSON(t *testing.T) {
	// Empty input + --json still emits the metadata record with all zeros.
	stdout, _, code := runValidate(t, "", []string{"--schema", `{"name":"string"}`, "--json"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONLines(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1 (metadata only)", len(recs))
	}
	meta := recs[0]
	if meta["_vrk"] != "validate" {
		t.Errorf("_vrk = %v, want %q", meta["_vrk"], "validate")
	}
	if meta["total"] != float64(0) {
		t.Errorf("total = %v, want 0", meta["total"])
	}
	if meta["passed"] != float64(0) {
		t.Errorf("passed = %v, want 0", meta["passed"])
	}
	if meta["failed"] != float64(0) {
		t.Errorf("failed = %v, want 0", meta["failed"])
	}
}

func TestNewlineOnlyStdin(t *testing.T) {
	// A single newline is like empty stdin — no records processed.
	stdout, _, code := runValidate(t, "\n", []string{"--schema", `{"name":"string"}`})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout should be empty, got: %q", stdout)
	}
}

// --- Valid records ---

func TestValidRecord_String(t *testing.T) {
	in := `{"name":"alice"}`
	stdout, _, code := runValidate(t, in, []string{"--schema", `{"name":"string"}`})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != in {
		t.Errorf("stdout = %q, want %q", stdout, in)
	}
}

func TestValidRecord_AllTypes(t *testing.T) {
	// One field of each type — all should pass.
	cases := []struct {
		schema string
		record string
	}{
		{`{"x":"string"}`, `{"x":"hello"}`},
		{`{"x":"number"}`, `{"x":42}`},
		{`{"x":"boolean"}`, `{"x":true}`},
		{`{"x":"array"}`, `{"x":[1,2,3]}`},
		{`{"x":"object"}`, `{"x":{"a":1}}`},
	}
	for _, tc := range cases {
		stdout, _, code := runValidate(t, tc.record, []string{"--schema", tc.schema})
		if code != 0 {
			t.Errorf("schema=%q record=%q: exit code = %d, want 0", tc.schema, tc.record, code)
		}
		if strings.TrimRight(stdout, "\n") != tc.record {
			t.Errorf("schema=%q: stdout = %q, want %q", tc.schema, stdout, tc.record)
		}
	}
}

func TestValidRecord_ExtraKeys(t *testing.T) {
	// Record has extra keys not in schema — still valid, emitted to stdout.
	in := `{"name":"alice","extra":"ignored"}`
	stdout, _, code := runValidate(t, in, []string{"--schema", `{"name":"string"}`})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != in {
		t.Errorf("stdout = %q, want %q", stdout, in)
	}
}

func TestValidRecord_LargeInteger(t *testing.T) {
	// Large integers must not lose precision — UseNumber() is required.
	in := `{"id":9007199254740993}`
	stdout, _, code := runValidate(t, in, []string{"--schema", `{"id":"number"}`})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "9007199254740993") {
		t.Errorf("large integer lost precision — UseNumber() missing? stdout: %q", stdout)
	}
}

// --- Invalid records ---

func TestInvalidRecord_WrongType(t *testing.T) {
	// age is a string but schema expects number.
	stdout, stderr, code := runValidate(t,
		`{"name":"alice","age":"wrong"}`,
		[]string{"--schema", `{"name":"string","age":"number"}`},
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (default warn mode)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty for invalid record, got: %q", stdout)
	}
	if !strings.Contains(stderr, "age") {
		t.Errorf("stderr should mention 'age', got: %q", stderr)
	}
	if !strings.Contains(stderr, "number") {
		t.Errorf("stderr should mention 'number', got: %q", stderr)
	}
}

func TestInvalidRecord_MissingKey(t *testing.T) {
	// Record is missing a key declared in the schema — invalid.
	stdout, stderr, code := runValidate(t,
		`{"name":"alice"}`,
		[]string{"--schema", `{"name":"string","age":"number"}`},
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty, got: %q", stdout)
	}
	if !strings.Contains(stderr, "age") {
		t.Errorf("stderr should mention missing key 'age', got: %q", stderr)
	}
}

func TestNullValue(t *testing.T) {
	// {"name": null} with schema {"name":"string"} — null is a distinct JSON type.
	// Must warn and skip (type mismatch), never crash.
	stdout, stderr, code := runValidate(t,
		`{"name":null}`,
		[]string{"--schema", `{"name":"string"}`},
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (null is a type mismatch, not a crash)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty for invalid record, got: %q", stdout)
	}
	if stderr == "" {
		t.Error("stderr must contain a warning for null value")
	}
	if !strings.Contains(stderr, "name") {
		t.Errorf("stderr should mention field 'name', got: %q", stderr)
	}
}

func TestLineNotJSON(t *testing.T) {
	stdout, stderr, code := runValidate(t, "not valid json", []string{"--schema", `{"name":"string"}`})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty for non-JSON line, got: %q", stdout)
	}
	if stderr == "" {
		t.Error("stderr must contain a warning for non-JSON line")
	}
}

// --- --strict mode ---

func TestStrictMode(t *testing.T) {
	// First invalid line → exit 1, processing stops.
	input := `{"name":"alice","age":"wrong"}` + "\n" + `{"name":"bob","age":30}`
	stdout, _, code := runValidate(t, input,
		[]string{"--schema", `{"name":"string","age":"number"}`, "--strict"},
	)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (strict mode hit invalid)", code)
	}
	// The second (valid) record must not appear on stdout.
	if strings.Contains(stdout, "bob") {
		t.Errorf("strict mode should stop at first invalid; found 'bob' in stdout: %q", stdout)
	}
}

func TestStrictMode_JSON_EmitsMetadata(t *testing.T) {
	// --strict + --json must emit the metadata record even when exiting early.
	// First line is invalid → strict fires → metadata emitted before exit 1.
	input := `{"name":123}` + "\n" + `{"name":"alice"}`
	stdout, _, code := runValidate(t, input,
		[]string{"--schema", `{"name":"string"}`, "--strict", "--json"},
	)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	recs := parseJSONLines(t, stdout)
	// First line is invalid → strict exits before the second line → only metadata.
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1 (metadata only — strict exited on first line)", len(recs))
	}
	meta := recs[0]
	if meta["_vrk"] != "validate" {
		t.Errorf("_vrk = %v, want %q", meta["_vrk"], "validate")
	}
	if meta["total"] != float64(1) {
		t.Errorf("total = %v, want 1", meta["total"])
	}
	if meta["passed"] != float64(0) {
		t.Errorf("passed = %v, want 0", meta["passed"])
	}
	if meta["failed"] != float64(1) {
		t.Errorf("failed = %v, want 1", meta["failed"])
	}
}

func TestStrictMode_AllValid(t *testing.T) {
	// --strict with all valid records → exit 0.
	input := `{"name":"alice"}` + "\n" + `{"name":"bob"}`
	stdout, _, code := runValidate(t, input,
		[]string{"--schema", `{"name":"string"}`, "--strict"},
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Errorf("got %d lines, want 2", len(lines))
	}
}

// --- --json metadata ---

func TestJSONMetadata(t *testing.T) {
	// Three lines: two valid, one invalid. --json appends metadata last.
	input := `{"name":"alice"}` + "\n" +
		`{"name":123}` + "\n" +
		`{"name":"bob"}`
	stdout, _, code := runValidate(t, input,
		[]string{"--schema", `{"name":"string"}`, "--json"},
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONLines(t, stdout)
	// 2 valid data records + 1 metadata record.
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3 (2 data + 1 meta)", len(recs))
	}
	meta := recs[len(recs)-1]
	if meta["_vrk"] != "validate" {
		t.Errorf("last record _vrk = %v, want %q", meta["_vrk"], "validate")
	}
	if meta["total"] != float64(3) {
		t.Errorf("total = %v, want 3", meta["total"])
	}
	if meta["passed"] != float64(2) {
		t.Errorf("passed = %v, want 2", meta["passed"])
	}
	if meta["failed"] != float64(1) {
		t.Errorf("failed = %v, want 1", meta["failed"])
	}
}

func TestJSONMetadata_IsLastLine(t *testing.T) {
	// The metadata record must appear after all data records.
	input := `{"name":"alice"}` + "\n" + `{"name":"bob"}`
	stdout, _, _ := runValidate(t, input,
		[]string{"--schema", `{"name":"string"}`, "--json"},
	)
	recs := parseJSONLines(t, stdout)
	last := recs[len(recs)-1]
	if _, ok := last["_vrk"]; !ok {
		t.Errorf("last record is not the metadata record: %v", last)
	}
}

// TestJSONErrorToStdout verifies the universal contract: --json + I/O error
// emits {"error":"...","code":1} to stdout (not stderr), and exits 1.
func TestJSONErrorToStdout(t *testing.T) {
	origNewStdinReader := newStdinReader
	newStdinReader = func() io.Reader {
		return &errorReader{err: fmt.Errorf("injected read error")}
	}
	t.Cleanup(func() { newStdinReader = origNewStdinReader })

	// runValidate manages os.Stdin/Stdout/Stderr/Args; our injected reader
	// overrides what Run() actually scans (not os.Stdin).
	stdout, stderr, code := runValidate(t, "",
		[]string{"--schema", `{"name":"string"}`, "--json"},
	)

	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty with --json on error, got: %q", stderr)
	}
	var m map[string]interface{}
	trimmed := strings.TrimSpace(stdout)
	if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nstdout: %q", err, stdout)
	}
	if _, ok := m["error"]; !ok {
		t.Errorf("stdout JSON missing 'error' field: %v", m)
	}
	if m["code"] != float64(1) {
		t.Errorf("stdout JSON code = %v, want 1", m["code"])
	}
}

// --- File-based schema ---

func TestFileSchema(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "schema*.json")
	if err != nil {
		t.Fatalf("create temp schema file: %v", err)
	}
	if _, err := f.WriteString(`{"name":"string","age":"number"}`); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	_ = f.Close()

	in := `{"name":"alice","age":30}`
	stdout, _, code := runValidate(t, in, []string{"--schema", f.Name()})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != in {
		t.Errorf("stdout = %q, want %q", stdout, in)
	}
}

func TestFileSchema_Invalid(t *testing.T) {
	// File exists but contains invalid JSON.
	f, err := os.CreateTemp(t.TempDir(), "schema*.json")
	if err != nil {
		t.Fatalf("create temp schema file: %v", err)
	}
	if _, err := f.WriteString(`{bad json`); err != nil {
		t.Fatalf("write schema: %v", err)
	}
	_ = f.Close()

	_, stderr, code := runValidate(t, "", []string{"--schema", f.Name()})
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "invalid schema JSON") {
		t.Errorf("stderr should mention 'invalid schema JSON', got: %q", stderr)
	}
}

// --- Mixed stream ---

func TestMixedStream(t *testing.T) {
	input := `{"name":"alice"}` + "\n" +
		`{"name":123}` + "\n" +
		`{"name":"bob"}`
	stdout, stderr, code := runValidate(t, input, []string{"--schema", `{"name":"string"}`})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// Only valid lines appear on stdout.
	if !strings.Contains(stdout, "alice") {
		t.Errorf("stdout should contain 'alice', got: %q", stdout)
	}
	if !strings.Contains(stdout, "bob") {
		t.Errorf("stdout should contain 'bob', got: %q", stdout)
	}
	if strings.Contains(stdout, "123") {
		t.Errorf("stdout must not contain the invalid record (123), got: %q", stdout)
	}
	// Invalid line warned to stderr.
	if stderr == "" {
		t.Error("stderr must contain a warning for the invalid line")
	}
}

func TestMixedStream_StdoutOrdering(t *testing.T) {
	// Valid records appear in their original order.
	input := `{"name":"first"}` + "\n" +
		`{"name":999}` + "\n" +
		`{"name":"second"}`
	stdout, _, _ := runValidate(t, input, []string{"--schema", `{"name":"string"}`})
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if !strings.Contains(lines[0], "first") || !strings.Contains(lines[1], "second") {
		t.Errorf("order wrong: lines = %v", lines)
	}
}

// --- --fix flag ---

func TestFixFlag(t *testing.T) {
	// Injected fixFn returns a valid repair — repaired line should appear on stdout.
	orig := fixFn
	fixFn = func(line, _ string) (string, bool) {
		return `{"name":"alice","age":30}`, true
	}
	t.Cleanup(func() { fixFn = orig })

	stdout, stderr, code := runValidate(t,
		`{"name":"alice","age":"wrong"}`,
		[]string{"--schema", `{"name":"string","age":"number"}`, "--fix"},
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, `"age":30`) {
		t.Errorf("repaired record should be on stdout, got: %q", stdout)
	}
	// No warnings when fix succeeds.
	if strings.Contains(stderr, "validation failed") {
		t.Errorf("stderr should not warn when fix succeeds, got: %q", stderr)
	}
}

func TestFixFlag_CountedAsPassed(t *testing.T) {
	// Repaired lines count as passed in --json metadata.
	orig := fixFn
	fixFn = func(line, _ string) (string, bool) {
		return `{"name":"alice","age":30}`, true
	}
	t.Cleanup(func() { fixFn = orig })

	stdout, _, code := runValidate(t,
		`{"name":"alice","age":"wrong"}`,
		[]string{"--schema", `{"name":"string","age":"number"}`, "--fix", "--json"},
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONLines(t, stdout)
	meta := recs[len(recs)-1]
	if meta["total"] != float64(1) {
		t.Errorf("total = %v, want 1", meta["total"])
	}
	if meta["passed"] != float64(1) {
		t.Errorf("passed = %v, want 1 (repaired line counts as passed)", meta["passed"])
	}
	if meta["failed"] != float64(0) {
		t.Errorf("failed = %v, want 0", meta["failed"])
	}
}

func TestFixFlag_FixFails(t *testing.T) {
	// Injected fixFn fails — line stays invalid, warn to stderr, exit 0.
	orig := fixFn
	fixFn = func(line, _ string) (string, bool) { return "", false }
	t.Cleanup(func() { fixFn = orig })

	stdout, stderr, code := runValidate(t,
		`{"name":"alice","age":"wrong"}`,
		[]string{"--schema", `{"name":"string","age":"number"}`, "--fix"},
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (--fix failure degrades silently)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty when fix fails, got: %q", stdout)
	}
	if stderr == "" {
		t.Error("stderr must have a warning when fix fails")
	}
}

func TestFixFlag_FixFails_Strict(t *testing.T) {
	// --fix + --strict + fix fails → exit 1.
	orig := fixFn
	fixFn = func(line, _ string) (string, bool) { return "", false }
	t.Cleanup(func() { fixFn = orig })

	_, _, code := runValidate(t,
		`{"name":"alice","age":"wrong"}`,
		[]string{"--schema", `{"name":"string","age":"number"}`, "--fix", "--strict"},
	)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (--strict + fix failure)", code)
	}
}

// --- Property tests ---

func TestPropertyEveryStdoutLineIsValidJSON(t *testing.T) {
	// Any line emitted to stdout must parse as valid JSON.
	inputs := []string{
		`{"name":"alice","age":30}`,
		`{"name":"alice","age":30}` + "\n" + `{"name":"bob","age":25}`,
		`{"name":"alice","age":30}` + "\n" + `{"name":123}` + "\n" + `{"name":"carol"}`,
	}
	schema := `{"name":"string","age":"number"}`
	for _, in := range inputs {
		stdout, _, _ := runValidate(t, in, []string{"--schema", schema})
		for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
			if line == "" {
				continue
			}
			var m interface{}
			if err := json.Unmarshal([]byte(line), &m); err != nil {
				t.Errorf("input %q: stdout line not valid JSON: %v\nline: %q", in, err, line)
			}
		}
	}
}

func TestPropertyExitCodesOnly(t *testing.T) {
	// Any run must exit 0, 1, or 2 — never anything else.
	cases := []struct {
		stdin string
		args  []string
	}{
		{`{"name":"alice"}`, []string{"--schema", `{"name":"string"}`}},
		{`{"name":123}`, []string{"--schema", `{"name":"string"}`}},
		{`{"name":123}`, []string{"--schema", `{"name":"string"}`, "--strict"}},
		{"", []string{"--schema", `{"name":"string"}`}},
		{"not json", []string{"--schema", `{"name":"string"}`}},
		{"", nil},
	}
	for _, tc := range cases {
		_, _, code := runValidate(t, tc.stdin, tc.args)
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("stdin=%q args=%v: exit code = %d, want 0, 1, or 2", tc.stdin, tc.args, code)
		}
	}
}

func TestPropertyValidLinesPassedThrough(t *testing.T) {
	// Any valid record emitted to stdout must be byte-identical to the input line.
	schema := `{"name":"string"}`
	cases := []string{
		`{"name":"alice"}`,
		`{"name":"bob","extra":"ignored"}`,
		`{"name":"carol","arr":[1,2,3]}`,
	}
	for _, in := range cases {
		stdout, _, code := runValidate(t, in, []string{"--schema", schema})
		if code != 0 {
			t.Errorf("input %q: exit code = %d, want 0", in, code)
			continue
		}
		got := strings.TrimRight(stdout, "\n")
		if got != in {
			t.Errorf("input %q: stdout = %q — valid lines must pass through unchanged", in, got)
		}
	}
}

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on I/O error.
// Exit code is unaffected.
func TestQuietSuppressesStderr(t *testing.T) {
	orig := newStdinReader
	newStdinReader = func() io.Reader { return &errorReader{err: fmt.Errorf("injected read error")} }
	t.Cleanup(func() { newStdinReader = orig })

	_, stderr, code := runValidate(t, "", []string{"--schema", `{"x":"string"}`, "--quiet"})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (I/O error)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietSuppressesValidationWarnings verifies that --quiet suppresses
// per-line validation warnings emitted to stderr. Exit code and stdout are
// unaffected: invalid lines are not passed through.
func TestQuietSuppressesValidationWarnings(t *testing.T) {
	stdout, stderr, code := runValidate(t, `{"x":123}`, []string{"--schema", `{"x":"string"}`, "--quiet"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (warn mode, not strict)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty (warnings suppressed)", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (invalid line not passed through)", stdout)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	stdout, stderr, code := runValidate(t, `{"x":"hello"}`, []string{"--schema", `{"x":"string"}`, "--quiet"})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	if !strings.Contains(stdout, `"x"`) {
		t.Errorf("stdout = %q, want the valid record passed through", stdout)
	}
}
