package urlinfo

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// runURLInfo replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and
// returns captured stdout, stderr, and the exit code. Not parallel-safe.
func runURLInfo(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"urlinfo"}, args...)
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

// parseJSONRecord parses a single JSON line into a map.
func parseJSONRecord(t *testing.T, line string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &m); err != nil {
		t.Fatalf("invalid JSON %q: %v", line, err)
	}
	return m
}

// parseJSONLines parses all non-blank lines of stdout as JSON records.
func parseJSONLines(t *testing.T, stdout string) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimRight(stdout, "\n"), "\n") {
		if line == "" {
			continue
		}
		records = append(records, parseJSONRecord(t, line))
	}
	return records
}

// --- Required contract tests (items 1-10) ---

// TestHappyPath verifies a full URL parses to all expected JSON fields.
func TestHappyPath(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"https://api.example.com/v1/users?page=2&limit=10"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rec := parseJSONRecord(t, strings.TrimSpace(stdout))
	if rec["scheme"] != "https" {
		t.Errorf("scheme = %v, want https", rec["scheme"])
	}
	if rec["host"] != "api.example.com" {
		t.Errorf("host = %v, want api.example.com", rec["host"])
	}
	if rec["path"] != "/v1/users" {
		t.Errorf("path = %v, want /v1/users", rec["path"])
	}
	if rec["port"] != float64(0) {
		t.Errorf("port = %v, want 0", rec["port"])
	}
	if rec["fragment"] != "" {
		t.Errorf("fragment = %v, want empty", rec["fragment"])
	}
	if rec["user"] != "" {
		t.Errorf("user = %v, want empty", rec["user"])
	}
	query, ok := rec["query"].(map[string]any)
	if !ok {
		t.Fatalf("query is not a map: %T %v", rec["query"], rec["query"])
	}
	if query["page"] != "2" {
		t.Errorf("query.page = %v, want 2", query["page"])
	}
	if query["limit"] != "10" {
		t.Errorf("query.limit = %v, want 10", query["limit"])
	}
}

// TestExitCodeSuccess verifies exit code 0 on a valid URL.
func TestExitCodeSuccess(t *testing.T) {
	_, _, code := runURLInfo(t, []string{"https://example.com"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
}

// TestInvalidURL verifies exit 1 when both scheme and host are empty.
func TestInvalidURL(t *testing.T) {
	_, stderr, code := runURLInfo(t, []string{"not a url"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "invalid URL") {
		t.Errorf("stderr = %q, want 'invalid URL'", stderr)
	}
}

// TestUnknownFlag verifies exit 2 on an unknown flag.
func TestUnknownFlag(t *testing.T) {
	_, _, code := runURLInfo(t, []string{"--bogus"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// TestHelp verifies --help exits 0 and stdout contains "urlinfo".
func TestHelp(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "urlinfo") {
		t.Errorf("--help stdout = %q, want it to contain 'urlinfo'", stdout)
	}
}

// TestTTYNoInput verifies that an interactive TTY with no args exits 2.
func TestTTYNoInput(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runURLInfo(t, nil, "")
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

// TestTTYWithJSON verifies that TTY + --json → exit 2, error JSON on stdout, stderr empty.
func TestTTYWithJSON(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runURLInfo(t, []string{"--json"}, "")
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

// TestJSONErrorToStdout verifies --json + I/O error → error JSON on stdout, stderr empty, exit 1.
func TestJSONErrorToStdout(t *testing.T) {
	origReadAll := readAll
	readAll = func(r io.Reader) ([]byte, error) {
		return nil, errors.New("simulated read error")
	}
	defer func() { readAll = origReadAll }()

	stdout, stderr, code := runURLInfo(t, []string{"--json"}, "any input")
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

// TestEmptyStdin verifies empty stdin (non-TTY) exits 0 with no output.
func TestEmptyStdin(t *testing.T) {
	stdout, _, code := runURLInfo(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// TestProperty verifies invariants across a variety of valid inputs: all
// required fields present, port >= 0, query is a map, no password field.
func TestProperty(t *testing.T) {
	urls := []string{
		"https://example.com",
		"https://example.com/path?a=1&b=2#frag",
		"https://user:pass@host:8080/p",
		"http://localhost:3000",
		"https://example.com/no-query",
		"//example.com/path",
	}
	for _, u := range urls {
		stdout, _, code := runURLInfo(t, []string{u}, "")
		if code != 0 {
			t.Errorf("url %q: exit code = %d, want 0", u, code)
			continue
		}
		rec := parseJSONRecord(t, strings.TrimSpace(stdout))
		for _, field := range []string{"scheme", "host", "path", "fragment", "user"} {
			if _, ok := rec[field]; !ok {
				t.Errorf("url %q: missing field %q", u, field)
			}
		}
		port, ok := rec["port"].(float64)
		if !ok {
			t.Errorf("url %q: port is not a number: %T %v", u, rec["port"], rec["port"])
		} else if port < 0 {
			t.Errorf("url %q: port = %v, want >= 0", u, port)
		}
		if _, ok := rec["query"].(map[string]any); !ok {
			t.Errorf("url %q: query is not a map: %T", u, rec["query"])
		}
		if _, hasPassword := rec["password"]; hasPassword {
			t.Errorf("url %q: password field must never appear in output", u)
		}
	}
}

// --- Additional spec cases ---

// TestFieldHost verifies --field host extracts the hostname as plain text.
func TestFieldHost(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"--field", "host", "https://api.example.com/path"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "api.example.com" {
		t.Errorf("stdout = %q, want %q", strings.TrimRight(stdout, "\n"), "api.example.com")
	}
}

// TestFieldQueryParam verifies --field query.page extracts a nested query param.
func TestFieldQueryParam(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"--field", "query.page", "https://example.com?page=2"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "2" {
		t.Errorf("stdout = %q, want %q", strings.TrimRight(stdout, "\n"), "2")
	}
}

// TestFieldPortAbsent verifies --field port returns empty string when no explicit port.
func TestFieldPortAbsent(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"--field", "port", "https://example.com"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "" {
		t.Errorf("stdout = %q, want empty (port not present in URL)", stdout)
	}
}

// TestFieldQueryMissing verifies --field query.missing returns empty string, exit 0.
func TestFieldQueryMissing(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"--field", "query.missing", "https://example.com"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimRight(stdout, "\n") != "" {
		t.Errorf("stdout = %q, want empty (missing query key)", stdout)
	}
}

// TestPasswordOmitted verifies that a URL with user:pass never outputs the password.
func TestPasswordOmitted(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"https://user:pass@host:8080/path#anchor"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "pass") || strings.Contains(stdout, "password") {
		t.Errorf("stdout contains password: %q", stdout)
	}
	rec := parseJSONRecord(t, strings.TrimSpace(stdout))
	if rec["user"] != "user" {
		t.Errorf("user = %v, want 'user'", rec["user"])
	}
	if rec["port"] != float64(8080) {
		t.Errorf("port = %v, want 8080", rec["port"])
	}
	if rec["fragment"] != "anchor" {
		t.Errorf("fragment = %v, want 'anchor'", rec["fragment"])
	}
}

// TestBatch verifies multiline stdin produces one JSON record per URL line.
func TestBatch(t *testing.T) {
	stdout, _, code := runURLInfo(t, nil, "https://example.com\nhttps://api.example.com\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONLines(t, stdout)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2; stdout=%q", len(recs), stdout)
	}
	if recs[0]["host"] != "example.com" {
		t.Errorf("recs[0].host = %v, want example.com", recs[0]["host"])
	}
	if recs[1]["host"] != "api.example.com" {
		t.Errorf("recs[1].host = %v, want api.example.com", recs[1]["host"])
	}
}

// TestStdinEquivalence verifies positional arg and stdin produce identical output.
func TestStdinEquivalence(t *testing.T) {
	const u = "https://example.com/path?x=1"
	stdoutArg, _, codeArg := runURLInfo(t, []string{u}, "")
	stdoutStdin, _, codeStdin := runURLInfo(t, nil, u+"\n")
	if codeArg != 0 {
		t.Fatalf("positional: exit code = %d, want 0", codeArg)
	}
	if codeStdin != 0 {
		t.Fatalf("stdin: exit code = %d, want 0", codeStdin)
	}
	if stdoutArg != stdoutStdin {
		t.Errorf("positional output:\n%s\nstdin output:\n%s\n(want identical)", stdoutArg, stdoutStdin)
	}
}

// TestJSONTrailingMetadata verifies --json appends {"_vrk":"urlinfo","count":N}.
func TestJSONTrailingMetadata(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"--json"}, "https://example.com\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONLines(t, stdout)
	if len(recs) != 2 {
		t.Fatalf("got %d records, want 2 (url record + metadata); stdout=%q", len(recs), stdout)
	}
	meta := recs[len(recs)-1]
	if meta["_vrk"] != "urlinfo" {
		t.Errorf("metadata _vrk = %v, want urlinfo", meta["_vrk"])
	}
	if meta["count"] != float64(1) {
		t.Errorf("metadata count = %v, want 1", meta["count"])
	}
}

// TestJSONBatchTrailerCount verifies --json batch count matches record count.
func TestJSONBatchTrailerCount(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"--json"}, "https://a.com\nhttps://b.com\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONLines(t, stdout)
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3 (2 urls + metadata); stdout=%q", len(recs), stdout)
	}
	meta := recs[len(recs)-1]
	if meta["count"] != float64(2) {
		t.Errorf("metadata count = %v, want 2", meta["count"])
	}
}

// TestSchemeRelativeURL verifies //host/path is exit 0 (host is non-empty).
func TestSchemeRelativeURL(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"//example.com/path"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rec := parseJSONRecord(t, strings.TrimSpace(stdout))
	if rec["host"] != "example.com" {
		t.Errorf("host = %v, want example.com", rec["host"])
	}
	if rec["scheme"] != "" {
		t.Errorf("scheme = %v, want empty for scheme-relative URL", rec["scheme"])
	}
}

// TestAllFieldKeys verifies all --field paths return expected plain text.
func TestAllFieldKeys(t *testing.T) {
	cases := []struct {
		field string
		url   string
		want  string
	}{
		{"scheme", "https://example.com", "https"},
		{"host", "https://example.com", "example.com"},
		{"path", "https://example.com/a/b", "/a/b"},
		{"fragment", "https://example.com#sec", "sec"},
		{"user", "https://alice@example.com", "alice"},
		{"port", "https://example.com:8080", "8080"},
		{"query", "https://example.com?page=2&limit=10", "page=2&limit=10"},
	}
	for _, tc := range cases {
		stdout, _, code := runURLInfo(t, []string{"--field", tc.field, tc.url}, "")
		if code != 0 {
			t.Errorf("--field %s: exit code = %d, want 0", tc.field, code)
			continue
		}
		got := strings.TrimRight(stdout, "\n")
		if got != tc.want {
			t.Errorf("--field %s: got %q, want %q", tc.field, got, tc.want)
		}
	}
}

// TestOutputFieldsStableShape verifies the JSON output shape is stable — all
// fields always present, including zero values (port:0, fragment:"", user:"").
func TestOutputFieldsStableShape(t *testing.T) {
	stdout, _, code := runURLInfo(t, []string{"https://example.com/path"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	rec := parseJSONRecord(t, strings.TrimSpace(stdout))
	required := []string{"scheme", "host", "port", "path", "query", "fragment", "user"}
	for _, field := range required {
		if _, ok := rec[field]; !ok {
			t.Errorf("missing field %q in output (must be present even as zero value)", field)
		}
	}
}
