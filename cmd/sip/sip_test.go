package sip

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
)

// errReader is an io.Reader that always fails — used to simulate a mid-stream I/O error.
type errReader struct{}

func (errReader) Read(_ []byte) (int, error) {
	return 0, errors.New("simulated read error")
}

// runSip replaces OS globals, calls Run(), and returns captured stdout, stderr, and the
// exit code. Not parallel-safe — tests share global OS state.
//
// isTerminal and stdinReader are NOT managed here; override them in individual tests
// with defer so the cleanup ordering stays predictable.
func runSip(t *testing.T, stdin string, args ...string) (stdout, stderr string, code int) {
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
	if _, err := io.WriteString(stdinW, stdin); err != nil {
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

	os.Args = append([]string{"sip"}, args...)
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

// outputLines splits a non-empty stdout string into lines, dropping the trailing empty
// element produced by the final newline. Returns nil for empty input.
func outputLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	return lines
}

// parseLastJSON parses the last non-empty line of output as a JSON object.
func parseLastJSON(t *testing.T, stdout string) map[string]any {
	t.Helper()
	lines := outputLines(stdout)
	if len(lines) == 0 {
		t.Fatal("parseLastJSON: no output lines")
	}
	last := lines[len(lines)-1]
	var m map[string]any
	if err := json.Unmarshal([]byte(last), &m); err != nil {
		t.Fatalf("parseLastJSON: %v (got %q)", err, last)
	}
	return m
}

// ----- --first -----

func TestFirstHappyPath(t *testing.T) {
	// Build 100-line input: "1\n2\n...\n100\n"
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--first", "10")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want 10", len(lines))
	}
	if lines[0] != "1" {
		t.Errorf("first line = %q, want %q", lines[0], "1")
	}
	if lines[9] != "10" {
		t.Errorf("last line = %q, want %q", lines[9], "10")
	}
}

func TestFirstShorterStream(t *testing.T) {
	// Stream shorter than requested → emit all, exit 0.
	stdout, _, code := runSip(t, "1\n2\n3\n4\n5\n", "--first", "10")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
}

func TestFirstZero(t *testing.T) {
	_, stderr, code := runSip(t, "data\n", "--first", "0")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--first must be >= 1") {
		t.Errorf("stderr = %q, want '--first must be >= 1'", stderr)
	}
}

// ----- --every -----

func TestEveryHappyPath(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--every", "10")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	if len(lines) != 10 {
		t.Fatalf("got %d lines, want 10; got: %v", len(lines), lines)
	}
	if lines[0] != "10" {
		t.Errorf("first line = %q, want %q", lines[0], "10")
	}
	if lines[9] != "100" {
		t.Errorf("last line = %q, want %q", lines[9], "100")
	}
}

func TestEveryShorterStream(t *testing.T) {
	// seq 7 | vrk sip --every 3 → lines 3 and 6 only (7 does not hit next multiple)
	stdout, _, code := runSip(t, "1\n2\n3\n4\n5\n6\n7\n", "--every", "3")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2; lines: %v", len(lines), lines)
	}
	if lines[0] != "3" {
		t.Errorf("line[0] = %q, want %q", lines[0], "3")
	}
	if lines[1] != "6" {
		t.Errorf("line[1] = %q, want %q", lines[1], "6")
	}
}

func TestEveryZero(t *testing.T) {
	_, stderr, code := runSip(t, "data\n", "--every", "0")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--every must be >= 1") {
		t.Errorf("stderr = %q, want '--every must be >= 1'", stderr)
	}
}

// ----- --count / -n (reservoir) -----

func TestReservoirExact(t *testing.T) {
	// 1000-line stream, sample 100 → exactly 100 lines.
	var sb strings.Builder
	for i := 1; i <= 1000; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--count", "100")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	if len(lines) != 100 {
		t.Fatalf("got %d lines, want 100", len(lines))
	}
}

func TestReservoirShorterStream(t *testing.T) {
	// Stream shorter than count → emit all, exit 0.
	stdout, _, code := runSip(t, "1\n2\n3\n4\n5\n", "--count", "10")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	if len(lines) != 5 {
		t.Fatalf("got %d lines, want 5", len(lines))
	}
}

func TestReservoirShortFormN(t *testing.T) {
	// -n short form must work identically to --count.
	stdout, _, code := runSip(t, "1\n2\n3\n", "-n", "2")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(outputLines(stdout)) != 2 {
		t.Fatalf("got %d lines via -n, want 2", len(outputLines(stdout)))
	}
}

func TestReservoirZero(t *testing.T) {
	_, stderr, code := runSip(t, "data\n", "--count", "0")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--count must be >= 1") {
		t.Errorf("stderr = %q, want '--count must be >= 1'", stderr)
	}
}

func TestReservoirSeedDeterministic(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 1000; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	input := sb.String()

	out1, _, code1 := runSip(t, input, "--count", "100", "--seed", "42")
	out2, _, code2 := runSip(t, input, "--count", "100", "--seed", "42")

	if code1 != 0 || code2 != 0 {
		t.Fatalf("exit codes = %d, %d; want 0, 0", code1, code2)
	}
	if out1 != out2 {
		t.Error("same seed produced different output on two runs")
	}
}

func TestReservoirSeedZeroValid(t *testing.T) {
	// Seed value 0 is valid — must not fall back to random.
	var sb strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	input := sb.String()

	out1, _, code1 := runSip(t, input, "--count", "50", "--seed", "0")
	out2, _, code2 := runSip(t, input, "--count", "50", "--seed", "0")

	if code1 != 0 || code2 != 0 {
		t.Fatalf("exit codes = %d, %d; want 0, 0", code1, code2)
	}
	if out1 != out2 {
		t.Error("seed 0 produced non-deterministic output — seed 0 must not fall back to random")
	}
}

func TestReservoirSeedDifference(t *testing.T) {
	// Different seeds should (with overwhelming probability) produce different output.
	var sb strings.Builder
	for i := 1; i <= 1000; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	input := sb.String()

	out42, _, _ := runSip(t, input, "--count", "100", "--seed", "42")
	out99, _, _ := runSip(t, input, "--count", "100", "--seed", "99")

	if out42 == out99 {
		t.Error("different seeds produced identical output — highly unlikely if seeding works")
	}
}

// ----- --sample -----

func TestSampleAll(t *testing.T) {
	// 100% sample includes every line.
	var sb strings.Builder
	for i := 1; i <= 1000; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--sample", "100")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	if len(lines) != 1000 {
		t.Fatalf("got %d lines with --sample 100, want 1000", len(lines))
	}
}

func TestSampleZero(t *testing.T) {
	_, stderr, code := runSip(t, "data\n", "--sample", "0")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--sample must be > 0") {
		t.Errorf("stderr = %q, want '--sample must be > 0'", stderr)
	}
}

func TestSampleOver100(t *testing.T) {
	_, stderr, code := runSip(t, "data\n", "--sample", "101")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--sample must be <= 100") {
		t.Errorf("stderr = %q, want '--sample must be <= 100'", stderr)
	}
}

func TestSampleSeedDeterministic(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 1000; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	input := sb.String()

	out1, _, code1 := runSip(t, input, "--sample", "10", "--seed", "42")
	out2, _, code2 := runSip(t, input, "--sample", "10", "--seed", "42")

	if code1 != 0 || code2 != 0 {
		t.Fatalf("exit codes = %d, %d; want 0, 0", code1, code2)
	}
	if out1 != out2 {
		t.Error("same seed produced different output on two runs with --sample")
	}
}

// ----- strategy validation -----

func TestNoStrategy(t *testing.T) {
	_, stderr, code := runSip(t, "some data\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "a sampling flag is required") {
		t.Errorf("stderr = %q, want 'a sampling flag is required'", stderr)
	}
}

func TestMutualExclusion(t *testing.T) {
	_, stderr, code := runSip(t, "data\n", "--first", "5", "--count", "10")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "only one sampling strategy") {
		t.Errorf("stderr = %q, want 'only one sampling strategy'", stderr)
	}
}

// ----- TTY guard -----

func TestTTYNoStrategy(t *testing.T) {
	// TTY + no strategy flag → "a sampling flag is required" (strategy check fires first)
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	_, stderr, code := runSip(t, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "a sampling flag is required") {
		t.Errorf("stderr = %q, want 'a sampling flag is required'", stderr)
	}
}

func TestTTYWithFlag(t *testing.T) {
	// TTY + valid strategy flag → "no input: pipe lines to stdin"
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	_, stderr, code := runSip(t, "", "--count", "10")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "no input") {
		t.Errorf("stderr = %q, want 'no input'", stderr)
	}
}

func TestTTYWithFlagJSON(t *testing.T) {
	// TTY + --count + --json → error JSON on stdout, stderr empty, exit 2.
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runSip(t, "", "--count", "10", "--json")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json active", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (got %q)", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	if obj["code"] != float64(2) {
		t.Errorf("JSON code = %v, want 2", obj["code"])
	}
}

// ----- empty input -----

func TestEmptyStream(t *testing.T) {
	// printf '' → no bytes → exit 0, no output.
	stdout, _, code := runSip(t, "", "--count", "10")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

func TestEmptyLine(t *testing.T) {
	// echo '' → one empty line → skipped → exit 0, no output.
	stdout, _, code := runSip(t, "\n", "--count", "10")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty (empty line must be skipped)", stdout)
	}
}

// ----- --help -----

func TestHelp(t *testing.T) {
	stdout, _, code := runSip(t, "", "--help")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "sip") {
		t.Errorf("--help stdout = %q, want it to contain 'sip'", stdout)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, code := runSip(t, "data\n", "--bogus")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// ----- TestJSONErrorToStdout -----

func TestJSONErrorToStdout(t *testing.T) {
	// I/O error mid-stream + --json → error JSON on stdout, stderr empty, exit 1.
	orig := stdinReader
	stdinReader = errReader{}
	defer func() { stdinReader = orig }()

	stdout, stderr, code := runSip(t, "", "--count", "10", "--json")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty when --json active", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v (got %q)", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	if obj["code"] != float64(1) {
		t.Errorf("JSON code = %v, want 1", obj["code"])
	}
}

// ----- --json trailer -----

func TestJSONTrailerFirst(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--first", "5", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	// 5 data lines + 1 JSON trailer
	if len(lines) != 6 {
		t.Fatalf("got %d lines, want 6 (5 data + 1 JSON trailer)", len(lines))
	}
	meta := parseLastJSON(t, stdout)
	if meta["_vrk"] != "sip" {
		t.Errorf("_vrk = %v, want 'sip'", meta["_vrk"])
	}
	if meta["strategy"] != "first" {
		t.Errorf("strategy = %v, want 'first'", meta["strategy"])
	}
	if meta["requested"] != float64(5) {
		t.Errorf("requested = %v, want 5", meta["requested"])
	}
	if meta["returned"] != float64(5) {
		t.Errorf("returned = %v, want 5", meta["returned"])
	}
	if meta["total_seen"] != float64(100) {
		t.Errorf("total_seen = %v, want 100", meta["total_seen"])
	}
}

func TestJSONTrailerEvery(t *testing.T) {
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--every", "10", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	meta := parseLastJSON(t, stdout)
	if meta["strategy"] != "every" {
		t.Errorf("strategy = %v, want 'every'", meta["strategy"])
	}
	if meta["requested"] != float64(10) {
		t.Errorf("requested = %v, want 10", meta["requested"])
	}
	if meta["returned"] != float64(10) {
		t.Errorf("returned = %v, want 10", meta["returned"])
	}
	if meta["total_seen"] != float64(100) {
		t.Errorf("total_seen = %v, want 100", meta["total_seen"])
	}
}

func TestJSONTrailerReservoir(t *testing.T) {
	// Stream shorter than n → returned = total_seen.
	stdout, _, code := runSip(t, "1\n2\n3\n4\n5\n", "--count", "10", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	meta := parseLastJSON(t, stdout)
	if meta["strategy"] != "reservoir" {
		t.Errorf("strategy = %v, want 'reservoir'", meta["strategy"])
	}
	if meta["requested"] != float64(10) {
		t.Errorf("requested = %v, want 10", meta["requested"])
	}
	if meta["returned"] != float64(5) {
		t.Errorf("returned = %v, want 5 (stream shorter than n)", meta["returned"])
	}
	if meta["total_seen"] != float64(5) {
		t.Errorf("total_seen = %v, want 5", meta["total_seen"])
	}
}

func TestJSONTrailerSample(t *testing.T) {
	// 100% sample on 50 lines → returned = 50.
	var sb strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--sample", "100", "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	meta := parseLastJSON(t, stdout)
	if meta["strategy"] != "sample" {
		t.Errorf("strategy = %v, want 'sample'", meta["strategy"])
	}
	if meta["requested"] != float64(100) {
		t.Errorf("requested = %v, want 100 (percentage value)", meta["requested"])
	}
	if meta["returned"] != float64(50) {
		t.Errorf("returned = %v, want 50", meta["returned"])
	}
	if meta["total_seen"] != float64(50) {
		t.Errorf("total_seen = %v, want 50", meta["total_seen"])
	}
}

// ----- --quiet -----

func TestQuietSuppressesStderr(t *testing.T) {
	// I/O error + --quiet → stderr empty, exit 1.
	orig := stdinReader
	stdinReader = errReader{}
	defer func() { stdinReader = orig }()

	_, stderr, code := runSip(t, "", "--count", "10", "--quiet")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

func TestQuietDoesNotAffectStdout(t *testing.T) {
	stdout, stderr, code := runSip(t, "a\nb\nc\n", "--first", "2", "--quiet")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty on success", stderr)
	}
	if !strings.Contains(stdout, "a") {
		t.Errorf("stdout = %q, want data output", stdout)
	}
}

// ----- property tests -----

func TestPropertyReservoirSubset(t *testing.T) {
	// Every line in reservoir output must be present in the input.
	input := "alpha\nbeta\ngamma\ndelta\nepsilon\nzeta\neta\ntheta\n"
	inputSet := make(map[string]bool)
	for _, l := range strings.Split(strings.TrimRight(input, "\n"), "\n") {
		inputSet[l] = true
	}

	stdout, _, code := runSip(t, input, "--count", "4", "--seed", "1")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, line := range outputLines(stdout) {
		if !inputSet[line] {
			t.Errorf("reservoir emitted %q which is not in input", line)
		}
	}
}

func TestPropertyFirstMaxN(t *testing.T) {
	// --first N output count is always <= N, for any stream size.
	cases := []struct{ n, stream int }{{5, 3}, {5, 5}, {5, 100}}
	for _, c := range cases {
		var sb strings.Builder
		for i := 1; i <= c.stream; i++ {
			fmt.Fprintf(&sb, "%d\n", i)
		}
		stdout, _, _ := runSip(t, sb.String(), "--first", fmt.Sprintf("%d", c.n))
		got := len(outputLines(stdout))
		if got > c.n {
			t.Errorf("--first %d on %d-line stream: got %d lines, want <= %d", c.n, c.stream, got, c.n)
		}
	}
}

func TestPropertyEveryDivisible(t *testing.T) {
	// All lines emitted by --every N are at positions divisible by N.
	// Using numeric input so we can parse position from value.
	var sb strings.Builder
	for i := 1; i <= 50; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--every", "7")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	for _, line := range outputLines(stdout) {
		var val int
		if _, err := fmt.Sscanf(line, "%d", &val); err != nil {
			t.Fatalf("non-numeric line %q from --every", line)
		}
		if val%7 != 0 {
			t.Errorf("--every 7 emitted %d, which is not divisible by 7", val)
		}
	}
}

func TestPropertyReservoirOrder(t *testing.T) {
	// Reservoir output must preserve input order: lines are emitted in strictly
	// increasing numeric order (because input is "1\n2\n...\n100\n").
	var sb strings.Builder
	for i := 1; i <= 100; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	stdout, _, code := runSip(t, sb.String(), "--count", "40", "--seed", "7")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := outputLines(stdout)
	if len(lines) == 0 {
		t.Fatal("no output lines")
	}
	prev := -1
	for _, line := range lines {
		var val int
		if _, err := fmt.Sscanf(line, "%d", &val); err != nil {
			t.Fatalf("non-numeric reservoir line %q", line)
		}
		if val <= prev {
			t.Errorf("reservoir output is out of order: %d after %d — input order not preserved", val, prev)
		}
		prev = val
	}
}

func TestPropertySampleBounds(t *testing.T) {
	// Returned count is always in [0, total_seen].
	// --sample 100 must return exactly all lines (Float64 returns [0,1), so threshold=1.0 always passes).
	var sb strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&sb, "%d\n", i)
	}
	input := sb.String()

	// Any percentage must not exceed total lines.
	for _, pct := range []string{"1", "50", "99"} {
		stdout, _, code := runSip(t, input, "--sample", pct, "--seed", "1")
		if code != 0 {
			t.Fatalf("--sample %s: exit code = %d, want 0", pct, code)
		}
		got := len(outputLines(stdout))
		if got > 200 {
			t.Errorf("--sample %s: got %d lines, exceeds total_seen 200", pct, got)
		}
	}

	// 100% must return all lines without exception.
	stdout, _, code := runSip(t, input, "--sample", "100")
	if code != 0 {
		t.Fatalf("--sample 100: exit code = %d, want 0", code)
	}
	if got := len(outputLines(stdout)); got != 200 {
		t.Errorf("--sample 100: got %d lines, want 200 (must return all)", got)
	}
}
