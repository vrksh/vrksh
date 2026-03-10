package coax

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// runCoax replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Restores globals via t.Cleanup.
// Do not call t.Parallel() — tests share global state (os.Stdin/Stdout/Stderr/Args).
func runCoax(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"coax"}, args...)
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

// readAttemptCount returns the number of bytes in a counter file written by
// tests using `printf x >> <file>` — one byte per attempt.
func readAttemptCount(t *testing.T, counterFile string) int {
	t.Helper()
	data, err := os.ReadFile(counterFile)
	if err != nil {
		return 0
	}
	return len(data)
}

// --- success ---

func TestSuccessImmediate(t *testing.T) {
	_, stderr, code := runCoax(t, []string{"--", "exit", "0"}, "")
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if strings.Contains(stderr, "coax:") {
		t.Errorf("expected no coax: progress lines on success, got: %q", stderr)
	}
}

// --- retry exhaustion ---

func TestRetryExhaustedDefault(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "count")
	script := fmt.Sprintf("printf x >> %s; exit 1", counter)

	_, stderr, code := runCoax(t, []string{"--", script}, "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	attempts := readAttemptCount(t, counter)
	if attempts != 4 {
		t.Errorf("expected 4 total attempts (1 initial + 3 retries), got %d", attempts)
	}
	coaxLines := strings.Count(stderr, "coax:")
	if coaxLines != 3 {
		t.Errorf("expected 3 coax: retry lines in stderr, got %d\nstderr: %q", coaxLines, stderr)
	}
}

func TestTimesFlag(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "count")
	script := fmt.Sprintf("printf x >> %s; exit 1", counter)

	_, _, code := runCoax(t, []string{"--times", "2", "--", script}, "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	attempts := readAttemptCount(t, counter)
	if attempts != 3 {
		t.Errorf("expected 3 total attempts (1 initial + 2 retries), got %d", attempts)
	}
}

// --- --on flag ---

func TestOnMatchRetries(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "count")
	script := fmt.Sprintf("printf x >> %s; exit 42", counter)

	_, _, code := runCoax(t, []string{"--on", "42", "--", script}, "")
	if code != 42 {
		t.Fatalf("expected code 42, got %d", code)
	}
	attempts := readAttemptCount(t, counter)
	if attempts != 4 {
		t.Errorf("expected 4 total attempts (--on 42 triggers retries), got %d", attempts)
	}
}

func TestOnNoMatchExitsImmediately(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "count")
	script := fmt.Sprintf("printf x >> %s; exit 1", counter)

	_, stderr, code := runCoax(t, []string{"--on", "42", "--", script}, "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	// Both assertions together: a bug where coax retries silently would pass the
	// stderr check but fail the attempt count check — neither alone is sufficient.
	if strings.Contains(stderr, "coax:") {
		t.Errorf("expected no coax: retry lines (exit 1 not in --on list), got: %q", stderr)
	}
	attempts := readAttemptCount(t, counter)
	if attempts != 1 {
		t.Errorf("expected exactly 1 attempt, got %d", attempts)
	}
}

// --- backoff ---

func TestBackoffFixed(t *testing.T) {
	start := time.Now()
	_, _, code := runCoax(t, []string{"--times", "3", "--backoff", "100ms", "--", "exit", "1"}, "")
	elapsed := time.Since(start)

	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	// 3 retries × 100ms = ~300ms; allow ±200ms for CI scheduler jitter
	if elapsed < 100*time.Millisecond || elapsed > 500*time.Millisecond {
		t.Errorf("expected wall time ~300ms ±200ms, got %v", elapsed)
	}
}

func TestBackoffExp(t *testing.T) {
	start := time.Now()
	_, _, code := runCoax(t, []string{"--times", "3", "--backoff", "exp:100ms", "--", "exit", "1"}, "")
	elapsed := time.Since(start)

	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	// 100ms + 200ms + 400ms = ~700ms; allow ±300ms for CI scheduler jitter
	if elapsed < 400*time.Millisecond || elapsed > 1000*time.Millisecond {
		t.Errorf("expected wall time ~700ms ±300ms, got %v", elapsed)
	}
}

func TestBackoffExpMax(t *testing.T) {
	start := time.Now()
	_, _, code := runCoax(t, []string{"--times", "3", "--backoff", "exp:100ms", "--backoff-max", "150ms", "--", "exit", "1"}, "")
	elapsed := time.Since(start)

	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	// 100ms + 150ms + 150ms = ~400ms; allow ±250ms for CI scheduler jitter
	if elapsed < 150*time.Millisecond || elapsed > 650*time.Millisecond {
		t.Errorf("expected wall time ~400ms ±250ms, got %v", elapsed)
	}
}

// --- --until ---

func TestUntilCondition(t *testing.T) {
	dir := t.TempDir()
	donefile := filepath.Join(dir, "done")
	until := fmt.Sprintf("test -f %s", donefile)

	_, _, code := runCoax(t, []string{"--times", "5", "--until", until, "--", "touch", donefile}, "")
	if code != 0 {
		t.Fatalf("expected code 0 (--until condition satisfied on first attempt), got %d", code)
	}
}

// --- stdin re-pipe ---

func TestStdinRePipe(t *testing.T) {
	stdout, _, code := runCoax(t, []string{"--times", "3", "--", "cat"}, "hello\n")
	if code != 0 {
		t.Fatalf("expected code 0 (cat exits 0), got %d", code)
	}
	if stdout != "hello\n" {
		t.Errorf("expected stdout %q, got %q", "hello\n", stdout)
	}
}

// --- usage errors ---

func TestTimesZero(t *testing.T) {
	_, stderr, code := runCoax(t, []string{"--times", "0", "--", "exit", "1"}, "")
	if code != 2 {
		t.Fatalf("expected code 2 (usage error), got %d", code)
	}
	if !strings.Contains(stderr, "times must be >= 1") {
		t.Errorf("expected 'times must be >= 1' in stderr, got: %q", stderr)
	}
}

func TestNoCommand(t *testing.T) {
	_, stderr, code := runCoax(t, []string{}, "")
	if code != 2 {
		t.Fatalf("expected code 2 (usage error), got %d", code)
	}
	if !strings.Contains(stderr, "missing command") {
		t.Errorf("expected 'missing command' in stderr, got: %q", stderr)
	}
}

// --- --quiet ---

func TestQuietSuppressesRetryOutput(t *testing.T) {
	script := `printf 'from subprocess\n' >&2; exit 1`
	_, stderr, code := runCoax(t, []string{"--quiet", "--times", "2", "--", script}, "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if strings.Contains(stderr, "coax:") {
		t.Errorf("--quiet: expected no coax: retry lines in stderr, got: %q", stderr)
	}
	if !strings.Contains(stderr, "from subprocess") {
		t.Errorf("--quiet: subprocess stderr must still pass through, got: %q", stderr)
	}
}

// --- property: never panic ---

func TestPropertyNeverPanics(t *testing.T) {
	cases := [][]string{
		{"--times", "1", "--", "exit", "0"},
		{"--backoff", "0s", "--", "exit", "0"},
		{"--times", "1", "--backoff", "exp:1ms", "--backoff-max", "1ms", "--", "exit", "0"},
		{"--on", "1", "--on", "2", "--", "exit", "0"},
		{"--times", "1", "--quiet", "--", "exit", "0"},
		{"--backoff", "badformat", "--times", "1", "--", "exit", "0"},
	}
	for _, args := range cases {
		runCoax(t, args, "") // just must not panic
	}
}
