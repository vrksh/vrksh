package moniker

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// No TestJSONErrorToStdout — moniker reads no stdin and performs no I/O that can fail.
// The only error paths are usage errors (exit 2). Same omission pattern as cmd/base.

// runMoniker replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runMoniker(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	// stdin: set up a pipe (moniker ignores content, but TestStdinIgnored needs a non-TTY fd).
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

	os.Args = append([]string{"moniker"}, args...)
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

// TestHappyPath verifies default output: one line, lowercase, two words, hyphen-separated.
func TestHappyPath(t *testing.T) {
	stdout, _, code := runMoniker(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	line := strings.TrimRight(stdout, "\n")
	parts := strings.Split(line, "-")
	if len(parts) != 2 {
		t.Fatalf("output %q: want exactly 2 hyphen-separated words, got %d", line, len(parts))
	}
	for _, p := range parts {
		if p == "" {
			t.Errorf("output %q: word is empty", line)
		}
		if p != strings.ToLower(p) {
			t.Errorf("output %q: word %q is not lowercase", line, p)
		}
	}
}

// TestCount verifies --count 5 emits exactly 5 lines and exits 0.
func TestCount(t *testing.T) {
	stdout, _, code := runMoniker(t, []string{"--count", "5"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5; stdout=%q", len(lines), stdout)
	}
}

// TestCountZero verifies --count 0 exits 2 (usage error).
func TestCountZero(t *testing.T) {
	_, _, code := runMoniker(t, []string{"--count", "0"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// TestCountNegative verifies --count -1 exits 2 (usage error).
func TestCountNegative(t *testing.T) {
	_, _, code := runMoniker(t, []string{"--count", "-1"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// TestSeparator verifies --separator _ uses underscore instead of hyphen.
func TestSeparator(t *testing.T) {
	stdout, _, code := runMoniker(t, []string{"--separator", "_"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	line := strings.TrimRight(stdout, "\n")
	if strings.Contains(line, "-") {
		t.Errorf("output %q: contains hyphen, want underscore separator", line)
	}
	if !strings.Contains(line, "_") {
		t.Errorf("output %q: missing underscore separator", line)
	}
}

// TestWords3 verifies --words 3 produces a name with exactly 2 hyphens (3 words).
func TestWords3(t *testing.T) {
	stdout, _, code := runMoniker(t, []string{"--words", "3"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	line := strings.TrimRight(stdout, "\n")
	count := strings.Count(line, "-")
	if count != 2 {
		t.Errorf("output %q: has %d hyphens, want 2 (3 words)", line, count)
	}
}

// TestWords1 verifies --words 1 exits 2 (minimum is 2).
func TestWords1(t *testing.T) {
	_, _, code := runMoniker(t, []string{"--words", "1"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// TestSeedDeterminism verifies --seed 42 produces the same output on two consecutive runs.
func TestSeedDeterminism(t *testing.T) {
	out1, _, code1 := runMoniker(t, []string{"--seed", "42"}, "")
	out2, _, code2 := runMoniker(t, []string{"--seed", "42"}, "")
	if code1 != 0 || code2 != 0 {
		t.Fatalf("exit codes = %d, %d, want 0, 0", code1, code2)
	}
	if out1 != out2 {
		t.Errorf("--seed 42 run1=%q run2=%q, want identical output", out1, out2)
	}
}

// TestSeedZero verifies --seed 0 is a valid seed that produces deterministic output
// and does NOT fall back to random (which would differ between runs).
func TestSeedZero(t *testing.T) {
	out1, _, code1 := runMoniker(t, []string{"--seed", "0"}, "")
	out2, _, code2 := runMoniker(t, []string{"--seed", "0"}, "")
	if code1 != 0 || code2 != 0 {
		t.Fatalf("exit codes = %d, %d, want 0, 0", code1, code2)
	}
	if out1 == "" {
		t.Fatal("--seed 0 produced empty output")
	}
	if out1 != out2 {
		t.Errorf("--seed 0 run1=%q run2=%q: seed 0 must be deterministic, not fall back to random", out1, out2)
	}
}

// TestSeedDifferent verifies --seed 42 and --seed 99 produce different output.
func TestSeedDifferent(t *testing.T) {
	out42, _, _ := runMoniker(t, []string{"--seed", "42"}, "")
	out99, _, _ := runMoniker(t, []string{"--seed", "99"}, "")
	if out42 == out99 {
		t.Errorf("--seed 42 and --seed 99 produced identical output %q, want different", out42)
	}
}

// TestJSONDefault verifies --json emits {"name":"...","words":["w1","w2"]} for 2-word names.
// The words array is consistent across all --words values so agent code can rely on one shape.
func TestJSONDefault(t *testing.T) {
	stdout, _, code := runMoniker(t, []string{"--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &rec); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	name, _ := rec["name"].(string)
	if name == "" {
		t.Errorf("JSON 'name' field is empty; got %v", rec)
	}
	words, ok := rec["words"].([]any)
	if !ok {
		t.Fatalf("JSON 'words' field missing or not an array; got %v", rec)
	}
	if len(words) != 2 {
		t.Errorf("words array has %d elements, want 2 (default --words)", len(words))
	}
	for i, w := range words {
		if s, _ := w.(string); s == "" {
			t.Errorf("words[%d] is empty", i)
		}
	}
	// name must equal words joined by default separator "-"
	w0, _ := words[0].(string)
	w1, _ := words[1].(string)
	if name != w0+"-"+w1 {
		t.Errorf("name=%q does not equal words[0]+'-'+words[1]=%q", name, w0+"-"+w1)
	}
	// must NOT have adjective/noun fields — consistent shape regardless of --words
	if _, ok := rec["adjective"]; ok {
		t.Error("JSON must not have 'adjective' field; use 'words' array for consistent shape")
	}
	if _, ok := rec["noun"]; ok {
		t.Error("JSON must not have 'noun' field; use 'words' array for consistent shape")
	}
}

// TestJSONCount verifies --json --count 5 emits 5 JSONL lines, each valid JSON.
func TestJSONCount(t *testing.T) {
	stdout, _, code := runMoniker(t, []string{"--json", "--count", "5"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5; stdout=%q", len(lines), stdout)
	}
	for i, line := range lines {
		var rec map[string]any
		if err := json.Unmarshal([]byte(line), &rec); err != nil {
			t.Errorf("line %d is not valid JSON: %v\ngot: %q", i+1, err, line)
		}
	}
}

// TestJSONWords3 verifies --json --words 3 emits {"name":"...","words":[w1,w2,w3]}.
// Same shape as 2-word — only the array length differs.
func TestJSONWords3(t *testing.T) {
	stdout, _, code := runMoniker(t, []string{"--json", "--words", "3"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &rec); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := rec["name"]; !ok {
		t.Error("JSON missing 'name' field")
	}
	words, ok := rec["words"].([]any)
	if !ok {
		t.Fatalf("JSON 'words' field missing or not an array; got %v", rec)
	}
	if len(words) != 3 {
		t.Errorf("words array has %d elements, want 3", len(words))
	}
	for i, w := range words {
		if s, _ := w.(string); s == "" {
			t.Errorf("words[%d] is empty", i)
		}
	}
}

// TestHelp verifies --help exits 0 and stdout contains "moniker".
func TestHelp(t *testing.T) {
	stdout, _, code := runMoniker(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "moniker") {
		t.Errorf("--help stdout = %q, want it to contain 'moniker'", stdout)
	}
}

// TestUnknownFlag verifies an unknown flag exits 2.
func TestUnknownFlag(t *testing.T) {
	_, _, code := runMoniker(t, []string{"--bogus"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// TestStdinIgnored verifies that piped stdin does not affect output format.
// Moniker generates names from embedded wordlists and ignores stdin entirely.
func TestStdinIgnored(t *testing.T) {
	stdout, _, code := runMoniker(t, nil, "this input must be ignored\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	line := strings.TrimRight(stdout, "\n")
	parts := strings.Split(line, "-")
	if len(parts) != 2 {
		t.Errorf("output %q: want 2 hyphen-separated words; stdin was not ignored", line)
	}
}

// TestQuiet verifies --quiet suppresses stderr on usage error; exit code unchanged.
func TestQuiet(t *testing.T) {
	_, stderr, code := runMoniker(t, []string{"--quiet", "--count", "0"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error unchanged by --quiet)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestPropertyUnique verifies --count 1000 produces exactly 1000 unique names.
// This is the canonical uniqueness property test: no duplicates within a batch.
func TestPropertyUnique(t *testing.T) {
	stdout, _, code := runMoniker(t, []string{"--count", "1000", "--seed", "42"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 1000 {
		t.Fatalf("got %d lines, want 1000", len(lines))
	}
	seen := make(map[string]struct{}, 1000)
	for _, l := range lines {
		if _, dup := seen[l]; dup {
			t.Errorf("duplicate name %q in --count 1000 batch", l)
			return
		}
		seen[l] = struct{}{}
	}
}
