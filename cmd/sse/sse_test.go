package sse

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// runSSE replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and exit code. Restores all globals via t.Cleanup.
// Do not call t.Parallel() — tests share os.Stdin/Stdout/Stderr global state.
func runSSE(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"sse"}, args...)
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

// --- Basic parsing ---

func TestBasicDataParsing(t *testing.T) {
	// Single data: block with no event: field → event defaults to "message".
	stdout, _, code := runSSE(t, nil, "data: {\"text\":\"hello\"}\n\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["event"] != "message" {
		t.Errorf("event = %v, want %q", records[0]["event"], "message")
	}
	data, ok := records[0]["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data is not an object: %T %v", records[0]["data"], records[0]["data"])
	}
	if data["text"] != "hello" {
		t.Errorf("data.text = %v, want %q", data["text"], "hello")
	}
}

func TestTwoEvents(t *testing.T) {
	// Two consecutive blocks → two records in order.
	input := "data: {\"text\":\"hello\"}\n\ndata: {\"text\":\"world\"}\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 2 {
		t.Fatalf("got %d records, want 2\nstdout: %q", len(records), stdout)
	}
	for i, rec := range records {
		if rec["event"] != "message" {
			t.Errorf("record[%d].event = %v, want %q", i, rec["event"], "message")
		}
	}
}

// --- Named events ---

func TestNamedEvent(t *testing.T) {
	// event: field overrides the default "message" type.
	input := "event: content_block_delta\ndata: {\"delta\":{\"text\":\"hi\"}}\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["event"] != "content_block_delta" {
		t.Errorf("event = %v, want %q", records[0]["event"], "content_block_delta")
	}
	data, ok := records[0]["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data is not an object: %T", records[0]["data"])
	}
	delta, ok := data["delta"].(map[string]interface{})
	if !ok {
		t.Fatalf("data.delta is not an object: %T", data["delta"])
	}
	if delta["text"] != "hi" {
		t.Errorf("data.delta.text = %v, want %q", delta["text"], "hi")
	}
}

// --- --event filter ---

func TestEventFilter(t *testing.T) {
	// Three events; --event content_block_delta passes only the matching one.
	input := "event: ping\ndata: {\"a\":1}\n\n" +
		"event: content_block_delta\ndata: {\"delta\":{\"text\":\"hi\"}}\n\n" +
		"event: ping\ndata: {\"b\":2}\n\n"
	stdout, _, code := runSSE(t, []string{"--event", "content_block_delta"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["event"] != "content_block_delta" {
		t.Errorf("event = %v, want %q", records[0]["event"], "content_block_delta")
	}
}

func TestEventFilterNoMatch(t *testing.T) {
	// --event filter with no matching events → no output, exit 0.
	input := "event: ping\ndata: {\"a\":1}\n\n"
	stdout, _, code := runSSE(t, []string{"--event", "other"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// --- [DONE] termination ---

func TestDoneTermination(t *testing.T) {
	// [DONE] after a real event: first record is emitted, then parsing stops.
	input := "data: {\"text\":\"hello\"}\n\ndata: [DONE]\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1 (only record before [DONE])\nstdout: %q", len(records), stdout)
	}
}

func TestDoneOnly(t *testing.T) {
	// A stream that is only [DONE] → no record emitted, exit 0.
	stdout, _, code := runSSE(t, nil, "data: [DONE]\n\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestDoneBypassesEventFilter(t *testing.T) {
	// [DONE] terminates even when --event filter is active.
	// It is a protocol signal, not a data event.
	stdout, _, code := runSSE(t, []string{"--event", "other"}, "data: [DONE]\n\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// --- Skipping malformed and comment lines ---

func TestMalformedLinesSkipped(t *testing.T) {
	// Lines without a recognised field prefix are silently dropped.
	input := "garbage line\nsome: other\ndata: {\"a\":1}\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
}

func TestCommentLinesSkipped(t *testing.T) {
	// Lines starting with ':' are SSE comments — silently dropped.
	input := ": this is a comment\n: another comment\ndata: {\"a\":1}\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
}

func TestEventWithNoData(t *testing.T) {
	// event: with no following data: — partial block has no data, dropped.
	input := "event: ping\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (no data field → block dropped)", stdout)
	}
}

// --- Empty stream ---

func TestEmptyStream(t *testing.T) {
	// Stdin closes immediately (zero bytes) → exit 0, no output.
	stdout, _, code := runSSE(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// --- Non-JSON data ---

func TestNonJSONDataEmittedAsString(t *testing.T) {
	// data: with non-JSON value → record is still emitted with data as a string.
	input := "data: not-json\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	if records[0]["data"] != "not-json" {
		t.Errorf("data = %v, want %q", records[0]["data"], "not-json")
	}
}

// --- Multi-line data ---

func TestMultiLineDataConcatenated(t *testing.T) {
	// Two data: lines in one block → joined with \n.
	input := "data: line1\ndata: line2\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	// "line1\nline2" contains a real newline character.
	if records[0]["data"] != "line1\nline2" {
		t.Errorf("data = %q, want %q", records[0]["data"], "line1\nline2")
	}
}

func TestMultiLineJSONData(t *testing.T) {
	// A JSON object split across two data: lines → concatenated string is valid JSON.
	// Lines: `{"a":1,`  and  `"b":2}` → joined: `{"a":1,\n"b":2}` → parses fine.
	input := "data: {\"a\":1,\ndata: \"b\":2}\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	records := parseJSONL(t, stdout)
	if len(records) != 1 {
		t.Fatalf("got %d records, want 1", len(records))
	}
	data, ok := records[0]["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data is not an object: %T %v", records[0]["data"], records[0]["data"])
	}
	// json.Unmarshal into interface{} represents numbers as float64.
	if data["a"] != float64(1) || data["b"] != float64(2) {
		t.Errorf("data = %v, want {a:1, b:2}", data)
	}
}

// --- --field extraction ---

func TestFieldExtraction(t *testing.T) {
	// --field data.delta.text extracts a nested string value.
	input := "event: content_block_delta\ndata: {\"delta\":{\"text\":\"hi\"}}\n\n"
	stdout, _, code := runSSE(t, []string{"--field", "data.delta.text"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "hi\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hi\n")
	}
}

func TestFieldExtractionMultipleEvents(t *testing.T) {
	// --field emits one value per matching event, one per line.
	input := "data: {\"text\":\"hello\"}\n\ndata: {\"text\":\"world\"}\n\n"
	stdout, _, code := runSSE(t, []string{"--field", "data.text"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 2 || lines[0] != "hello" || lines[1] != "world" {
		t.Errorf("stdout = %q, want %q", stdout, "hello\nworld\n")
	}
}

func TestFieldExtractionNumber(t *testing.T) {
	// Non-string number → JSON representation (bare digits, no surrounding quotes).
	input := "data: {\"count\":42}\n\n"
	stdout, _, code := runSSE(t, []string{"--field", "data.count"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "42\n" {
		t.Errorf("stdout = %q, want %q", stdout, "42\n")
	}
}

func TestFieldExtractionBool(t *testing.T) {
	// Non-string boolean → JSON representation ("true" or "false", no quotes).
	input := "data: {\"flag\":true}\n\n"
	stdout, _, code := runSSE(t, []string{"--field", "data.flag"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "true\n" {
		t.Errorf("stdout = %q, want %q", stdout, "true\n")
	}
}

func TestFieldExtractionEventField(t *testing.T) {
	// --field event extracts the event name itself from the top-level record.
	input := "event: content_block_delta\ndata: {\"a\":1}\n\n"
	stdout, _, code := runSSE(t, []string{"--field", "event"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "content_block_delta\n" {
		t.Errorf("stdout = %q, want %q", stdout, "content_block_delta\n")
	}
}

func TestFieldNotFound(t *testing.T) {
	// Path not found in the record → skip silently (no output), exit 0.
	input := "data: {\"a\":1}\n\n"
	stdout, _, code := runSSE(t, []string{"--field", "data.b"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (path not found → skip silently)", stdout)
	}
}

func TestFieldOnNonJSONData(t *testing.T) {
	// --field on a record where data is not JSON → skip silently.
	input := "data: not-json\n\n"
	stdout, _, code := runSSE(t, []string{"--field", "data.text"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (non-JSON data → skip silently)", stdout)
	}
}

func TestFieldCombinedWithEventFilter(t *testing.T) {
	// --field and --event work together: filter first, then extract.
	input := "event: ping\ndata: {\"text\":\"ignored\"}\n\n" +
		"event: delta\ndata: {\"text\":\"hi\"}\n\n"
	stdout, _, code := runSSE(t, []string{"--event", "delta", "--field", "data.text"}, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "hi\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hi\n")
	}
}

// --- Usage errors ---

func TestUnknownFlag(t *testing.T) {
	_, stderr, code := runSSE(t, []string{"--unknown-flag"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage text, got empty")
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runSSE(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "sse") {
		t.Errorf("--help stdout = %q: missing tool name", stdout)
	}
}

// --- SSE spec edge cases ---

func TestSSEExactSpaceStripping(t *testing.T) {
	// SSE spec: strip exactly one leading space from field values after the colon.
	// "data: hello"  → "hello"   (one space stripped)
	// "data:  hello" → " hello"  (one stripped, one remains)
	// "data:hello"   → "hello"   (no space present, nothing to strip)
	cases := []struct {
		input   string
		wantVal string
	}{
		{"data: hello\n\n", "hello"},
		{"data:  hello\n\n", " hello"},
		{"data:hello\n\n", "hello"},
	}
	for _, tc := range cases {
		stdout, _, code := runSSE(t, nil, tc.input)
		if code != 0 {
			t.Errorf("input %q: exit code = %d, want 0", tc.input, code)
			continue
		}
		recs := parseJSONL(t, stdout)
		if len(recs) != 1 {
			t.Errorf("input %q: got %d records, want 1", tc.input, len(recs))
			continue
		}
		if recs[0]["data"] != tc.wantVal {
			t.Errorf("input %q: data = %q, want %q", tc.input, recs[0]["data"], tc.wantVal)
		}
	}
}

func TestSSEEOFWithoutTrailingBlankLine(t *testing.T) {
	// Stdin closes mid-block (no blank line to dispatch) → pending block dropped,
	// exit 0, no output. Per SSE spec: dispatch happens only on blank line.
	input := "data: {\"a\":1}" // no trailing \n\n
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (pending block dropped on EOF)", stdout)
	}
}

func TestSSEEmptyDataField(t *testing.T) {
	// "data:" with no value contributes an empty string to the accumulation buffer.
	// "data:\ndata: hello" → buffer joins as "" + "\n" + "hello" = "\nhello".
	input := "data:\ndata: hello\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONL(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	// The data string contains a real newline character before "hello".
	if recs[0]["data"] != "\nhello" {
		t.Errorf("data = %q, want %q", recs[0]["data"], "\nhello")
	}
}

// --- SSE spec: id/retry fields and multiple event fields ---

func TestSSESkipsIdAndRetryFields(t *testing.T) {
	// id: and retry: are recognised SSE fields but vrk sse does not use them.
	// They are silently skipped; the data record is still emitted correctly.
	input := "event: message\nid: abc123\nretry: 3000\ndata: {\"text\":\"hello\"}\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONL(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["event"] != "message" {
		t.Errorf("event = %v, want %q", recs[0]["event"], "message")
	}
	data, ok := recs[0]["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("data is not an object: %T", recs[0]["data"])
	}
	if data["text"] != "hello" {
		t.Errorf("data.text = %v, want %q", data["text"], "hello")
	}
}

func TestSSEMultipleEventFieldsLastWins(t *testing.T) {
	// SSE spec: if multiple event: fields appear in one block, the last one wins.
	input := "event: first\nevent: last\ndata: {\"a\":1}\n\n"
	stdout, _, code := runSSE(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	recs := parseJSONL(t, stdout)
	if len(recs) != 1 {
		t.Fatalf("got %d records, want 1", len(recs))
	}
	if recs[0]["event"] != "last" {
		t.Errorf("event = %v, want %q (last event: field wins)", recs[0]["event"], "last")
	}
}

// --- Property tests ---

func TestPropertyEveryOutputLineIsValidJSON(t *testing.T) {
	// For any SSE input that produces output, every stdout line must be valid JSON.
	inputs := []string{
		"data: {\"text\":\"hello\"}\n\n",
		"event: ping\ndata: {\"a\":1}\n\nevent: pong\ndata: {\"b\":2}\n\n",
		"data: [DONE]\n\n",
		"data: not-json\n\n",
		": comment\ndata: {\"x\":1}\n\n",
		"data: {\"n\":42,\"b\":true,\"s\":\"str\"}\n\n",
		"data: line1\ndata: line2\n\n",
	}
	for _, input := range inputs {
		stdout, _, _ := runSSE(t, nil, input)
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
	// Any input must produce exit code 0, 1, or 2 — never anything else.
	inputs := []string{
		"data: {\"text\":\"hello\"}\n\n",
		"data: [DONE]\n\n",
		"",
		"garbage\n\n",
		": comment only\n\n",
		"event: test\ndata: {\"a\":1}\n\n",
	}
	for _, input := range inputs {
		_, _, code := runSSE(t, nil, input)
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("input %q: exit code = %d, want 0, 1, or 2", input, code)
		}
	}
}

// --- Fuzz ---

func FuzzSSE(f *testing.F) {
	f.Add("data: {\"text\":\"hello\"}\n\n")
	f.Add("event: test\ndata: {\"a\":1}\n\n")
	f.Add("data: [DONE]\n\n")
	f.Add(": comment\n\n")
	f.Add("")
	f.Add("data: not-json\n\n")
	f.Add("data: line1\ndata: line2\n\n")
	f.Add("data:\ndata: hello\n\n")
	f.Add("data:  double-space\n\n")

	f.Fuzz(func(t *testing.T, input string) {
		_, _, code := runSSE(t, nil, input)
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("exit code = %d, want 0, 1, or 2", code)
		}
	})
}
