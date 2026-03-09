package uuid

// Tests for vrk uuid.
//
// Do NOT call t.Parallel() — tests share os.Args, os.Stdin, os.Stdout, os.Stderr.
//
// UUID string layout (positions in "8-4-4-4-12" format):
//   xxxxxxxx-xxxx-Vxxx-Bxxx-xxxxxxxxxxxx
//   0      8 9  13 14 18 19 23 24      35
//
//   index 14 = version nibble (4 for v4, 7 for v7)
//   index 19 = variant nibble (8/9/a/b → high bits 10xx)

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"strings"
	"testing"
)

var uuidRe = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// runUUID replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Restores globals via t.Cleanup.
func runUUID(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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
		t.Fatalf("close stdin: %v", err)
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

	os.Args = append([]string{"uuid"}, args...)
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

// --- Behaviour tests ---

func TestHelp(t *testing.T) {
	stdout, _, code := runUUID(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("--help: exit code = %d, want 0", code)
	}
	if stdout == "" {
		t.Error("--help: stdout is empty, want usage text")
	}
}

func TestDefaultV4(t *testing.T) {
	stdout, _, code := runUUID(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	line := strings.TrimRight(stdout, "\n")
	if !uuidRe.MatchString(line) {
		t.Fatalf("output %q does not match UUID regex", line)
	}
	// Version nibble at index 14 must be '4'.
	if line[14] != '4' {
		t.Errorf("version nibble = %c at index 14, want '4' (full UUID: %s)", line[14], line)
	}
	// Variant nibble at index 19 must be 8, 9, a, or b (high bits 10xx).
	v := line[19]
	if v != '8' && v != '9' && v != 'a' && v != 'b' {
		t.Errorf("variant nibble = %c at index 19, want one of 8/9/a/b (full UUID: %s)", v, line)
	}
}

func TestV4TwoCallsDiffer(t *testing.T) {
	s1, _, _ := runUUID(t, nil, "")
	s2, _, _ := runUUID(t, nil, "")
	u1 := strings.TrimRight(s1, "\n")
	u2 := strings.TrimRight(s2, "\n")
	if u1 == u2 {
		t.Errorf("two consecutive v4 UUIDs are identical: %s", u1)
	}
}

func TestV7Flag(t *testing.T) {
	stdout, _, code := runUUID(t, []string{"--v7"}, "")
	if code != 0 {
		t.Fatalf("--v7: exit code = %d, want 0", code)
	}
	line := strings.TrimRight(stdout, "\n")
	if !uuidRe.MatchString(line) {
		t.Fatalf("--v7: output %q does not match UUID regex", line)
	}
	if line[14] != '7' {
		t.Errorf("--v7: version nibble = %c at index 14, want '7' (full UUID: %s)", line[14], line)
	}
}

func TestV7Ordered(t *testing.T) {
	stdout, _, code := runUUID(t, []string{"--v7", "--count", "5"}, "")
	if code != 0 {
		t.Fatalf("--v7 --count 5: exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("--v7 --count 5: got %d lines, want 5", len(lines))
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] < lines[i-1] {
			t.Errorf("--v7 --count 5: UUID[%d] %q < UUID[%d] %q (not ordered)", i, lines[i], i-1, lines[i-1])
		}
	}
}

func TestCountFive(t *testing.T) {
	stdout, _, code := runUUID(t, []string{"--count", "5"}, "")
	if code != 0 {
		t.Fatalf("--count 5: exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("--count 5: got %d lines, want 5", len(lines))
	}
	for i, line := range lines {
		if !uuidRe.MatchString(line) {
			t.Errorf("--count 5: line[%d] %q does not match UUID regex", i, line)
		}
	}
}

func TestCountFiveV7(t *testing.T) {
	stdout, _, code := runUUID(t, []string{"--v7", "--count", "5"}, "")
	if code != 0 {
		t.Fatalf("--v7 --count 5: exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("--v7 --count 5: got %d lines, want 5", len(lines))
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] < lines[i-1] {
			t.Errorf("--v7 --count 5: UUID[%d] %q < UUID[%d] %q (not lexicographically ordered)", i, lines[i], i-1, lines[i-1])
		}
	}
}

func TestJSONv4(t *testing.T) {
	stdout, _, code := runUUID(t, []string{"--json"}, "")
	if code != 0 {
		t.Fatalf("--json: exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimRight(stdout, "\n")), &obj); err != nil {
		t.Fatalf("--json: output is not valid JSON: %v\noutput: %s", err, stdout)
	}
	assertJSONUUIDObject(t, "--json", obj, 4)
}

func TestJSONv7(t *testing.T) {
	stdout, _, code := runUUID(t, []string{"--v7", "--json"}, "")
	if code != 0 {
		t.Fatalf("--v7 --json: exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimRight(stdout, "\n")), &obj); err != nil {
		t.Fatalf("--v7 --json: output is not valid JSON: %v\noutput: %s", err, stdout)
	}
	assertJSONUUIDObject(t, "--v7 --json", obj, 7)
}

func TestJSONCountFive(t *testing.T) {
	stdout, _, code := runUUID(t, []string{"--count", "5", "--json"}, "")
	if code != 0 {
		t.Fatalf("--count 5 --json: exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("--count 5 --json: got %d JSONL lines, want 5", len(lines))
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("--count 5 --json: line[%d] is not valid JSON: %v\nline: %s", i, err, line)
		}
		assertJSONUUIDObject(t, "--count 5 --json line "+string(rune('0'+i)), obj, 4)
	}
}

// assertJSONUUIDObject checks that a parsed JSON object has the required uuid,
// version, and generated_at fields with correct types and values.
func assertJSONUUIDObject(t *testing.T, ctx string, obj map[string]any, wantVersion int) {
	t.Helper()

	// uuid: must be a string matching the UUID regex
	uuidVal, ok := obj["uuid"]
	if !ok {
		t.Errorf("%s: JSON missing 'uuid' field", ctx)
	} else {
		uuidStr, isStr := uuidVal.(string)
		if !isStr {
			t.Errorf("%s: 'uuid' field is not a string: %T", ctx, uuidVal)
		} else if !uuidRe.MatchString(uuidStr) {
			t.Errorf("%s: 'uuid' value %q does not match UUID regex", ctx, uuidStr)
		}
	}

	// version: must be a number equal to wantVersion
	versionVal, ok := obj["version"]
	if !ok {
		t.Errorf("%s: JSON missing 'version' field", ctx)
	} else {
		// json.Unmarshal into interface{} uses float64 for all numbers
		versionNum, isNum := versionVal.(float64)
		if !isNum {
			t.Errorf("%s: 'version' field is not a number: %T", ctx, versionVal)
		} else if int(versionNum) != wantVersion {
			t.Errorf("%s: 'version' = %v, want %d", ctx, versionNum, wantVersion)
		}
	}

	// generated_at: must be a positive integer (unix seconds)
	genVal, ok := obj["generated_at"]
	if !ok {
		t.Errorf("%s: JSON missing 'generated_at' field", ctx)
	} else {
		genNum, isNum := genVal.(float64)
		if !isNum {
			t.Errorf("%s: 'generated_at' field is not a number: %T", ctx, genVal)
		} else if genNum <= 0 {
			t.Errorf("%s: 'generated_at' = %v, want a positive unix timestamp", ctx, genNum)
		}
	}
}

func TestCountZero(t *testing.T) {
	stdout, stderr, code := runUUID(t, []string{"--count", "0"}, "")
	if code != 2 {
		t.Fatalf("--count 0: exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("--count 0: stdout = %q, want empty", stdout)
	}
	if stderr == "" {
		t.Error("--count 0: stderr is empty, want error message")
	}
}

func TestCountNegative(t *testing.T) {
	// pflag parses --count -1 as count=-1 (a valid int), then our count < 1
	// validation fires and returns exit 2 via shared.UsageErrorf.
	_, _, code := runUUID(t, []string{"--count", "-1"}, "")
	if code != 2 {
		t.Fatalf("--count -1: exit code = %d, want 2", code)
	}
}

func TestIgnoresStdin(t *testing.T) {
	// Even when stdin has content, uuid should ignore it and produce a UUID.
	stdout, _, code := runUUID(t, nil, "this content should be ignored\n")
	if code != 0 {
		t.Fatalf("stdin ignored: exit code = %d, want 0", code)
	}
	line := strings.TrimRight(stdout, "\n")
	if !uuidRe.MatchString(line) {
		t.Errorf("stdin ignored: output %q does not match UUID regex", line)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, code := runUUID(t, []string{"--bogus"}, "")
	if code != 2 {
		t.Fatalf("--bogus: exit code = %d, want 2", code)
	}
}

// --- Property tests ---

func TestPropertyUUIDFormat(t *testing.T) {
	for i := 0; i < 100; i++ {
		stdout, _, code := runUUID(t, nil, "")
		if code != 0 {
			t.Fatalf("iteration %d: exit code = %d, want 0", i, code)
		}
		line := strings.TrimRight(stdout, "\n")
		if !uuidRe.MatchString(line) {
			t.Errorf("iteration %d: %q does not match UUID regex", i, line)
		}
	}
}

func TestPropertyV4VersionBit(t *testing.T) {
	for i := 0; i < 100; i++ {
		stdout, _, _ := runUUID(t, nil, "")
		line := strings.TrimRight(stdout, "\n")
		if len(line) < 15 {
			t.Fatalf("iteration %d: output too short: %q", i, line)
		}
		if line[14] != '4' {
			t.Errorf("iteration %d: version nibble = %c, want '4' (UUID: %s)", i, line[14], line)
		}
	}
}

func TestPropertyV4VariantBits(t *testing.T) {
	for i := 0; i < 100; i++ {
		stdout, _, _ := runUUID(t, nil, "")
		line := strings.TrimRight(stdout, "\n")
		if len(line) < 20 {
			t.Fatalf("iteration %d: output too short: %q", i, line)
		}
		v := line[19]
		if v != '8' && v != '9' && v != 'a' && v != 'b' {
			t.Errorf("iteration %d: variant nibble = %c, want one of 8/9/a/b (UUID: %s)", i, v, line)
		}
	}
}

func TestPropertyV7VersionBit(t *testing.T) {
	for i := 0; i < 100; i++ {
		stdout, _, _ := runUUID(t, []string{"--v7"}, "")
		line := strings.TrimRight(stdout, "\n")
		if len(line) < 15 {
			t.Fatalf("iteration %d: output too short: %q", i, line)
		}
		if line[14] != '7' {
			t.Errorf("iteration %d: version nibble = %c, want '7' (UUID: %s)", i, line[14], line)
		}
	}
}

func TestPropertyV7Order(t *testing.T) {
	// Generate 1000 v7 UUIDs in one batch and verify they are non-decreasing.
	stdout, _, code := runUUID(t, []string{"--v7", "--count", "1000"}, "")
	if code != 0 {
		t.Fatalf("--v7 --count 1000: exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1000 {
		t.Fatalf("--v7 --count 1000: got %d lines, want 1000", len(lines))
	}
	for i := 1; i < len(lines); i++ {
		if lines[i] < lines[i-1] {
			t.Errorf("v7 ordering violated at index %d: %q < %q", i, lines[i], lines[i-1])
		}
	}
}

func TestPropertyUniqueness(t *testing.T) {
	// Generate 1000 v4 UUIDs in one batch and verify all are distinct.
	stdout, _, code := runUUID(t, []string{"--count", "1000"}, "")
	if code != 0 {
		t.Fatalf("--count 1000: exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1000 {
		t.Fatalf("--count 1000: got %d lines, want 1000", len(lines))
	}
	seen := make(map[string]int, len(lines))
	for i, line := range lines {
		if prev, exists := seen[line]; exists {
			t.Errorf("duplicate UUID %q at indices %d and %d", line, prev, i)
		}
		seen[line] = i
	}
}
