package coax

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

// runCoax replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Restores globals via t.Cleanup.
// Do not call t.Parallel() - tests share global state (os.Stdin/Stdout/Stderr/Args).
func runCoax(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
	t.Helper()

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
	return runCoaxStdin(t, args, stdinR)
}

// runCoaxStdin is runCoax with a caller-supplied stdin file, for tests that
// need stdin to stay open (a pipe with a live writer).
func runCoaxStdin(t *testing.T, args []string, stdin *os.File) (stdout, stderr string, code int) {
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

	os.Stdin = stdin

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
// tests using `printf x >> <file>` - one byte per attempt.
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
	_, stderr, code := runCoax(t, []string{"--", "true"}, "")
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

	_, stderr, code := runCoax(t, []string{"--", "sh", "-c", script}, "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	attempts := readAttemptCount(t, counter)
	if attempts != 4 {
		t.Errorf("expected 4 total attempts (1 initial + 3 retries), got %d", attempts)
	}
	coaxLines := strings.Count(stderr, "coax:")
	if coaxLines != 4 {
		t.Errorf("expected 3 coax: retry lines plus 1 final line in stderr, got %d\nstderr: %q", coaxLines, stderr)
	}
	if !strings.Contains(stderr, "coax: all 4 attempts failed (last exit 1)") {
		t.Errorf("expected final exhaustion line, got: %q", stderr)
	}
}

func TestTimesFlag(t *testing.T) {
	dir := t.TempDir()
	counter := filepath.Join(dir, "count")
	script := fmt.Sprintf("printf x >> %s; exit 1", counter)

	_, _, code := runCoax(t, []string{"--times", "2", "--", "sh", "-c", script}, "")
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

	_, _, code := runCoax(t, []string{"--on", "42", "--", "sh", "-c", script}, "")
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

	_, stderr, code := runCoax(t, []string{"--on", "42", "--", "sh", "-c", script}, "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	// Both assertions together: a bug where coax retries silently would pass the
	// stderr check but fail the attempt count check - neither alone is sufficient.
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
	_, _, code := runCoax(t, []string{"--times", "3", "--backoff", "100ms", "--", "false"}, "")
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
	_, _, code := runCoax(t, []string{"--times", "3", "--backoff", "exp:100ms", "--", "false"}, "")
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
	_, _, code := runCoax(t, []string{"--times", "3", "--backoff", "exp:100ms", "--backoff-max", "150ms", "--", "false"}, "")
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
	_, stderr, code := runCoax(t, []string{"--times", "0", "--", "false"}, "")
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
	_, stderr, code := runCoax(t, []string{"--quiet", "--times", "2", "--", "sh", "-c", script}, "")
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
		{"--times", "1", "--", "true"},
		{"--backoff", "0s", "--", "true"},
		{"--times", "1", "--backoff", "exp:1ms", "--backoff-max", "1ms", "--", "true"},
		{"--on", "1", "--on", "2", "--", "true"},
		{"--times", "1", "--quiet", "--", "true"},
		{"--backoff", "badformat", "--times", "1", "--", "true"},
	}
	for _, args := range cases {
		runCoax(t, args, "") // just must not panic
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	// --times 0 is a usage error; with --json it must go to stdout as JSON.
	stdout, stderr, code := runCoax(t, []string{"--times", "0", "--json", "--", "false"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing key \"error\"")
	}
	if c, _ := obj["code"].(float64); int(c) != 2 {
		t.Errorf("code = %v, want 2", obj["code"])
	}
}

// --- argv execution (no shell) ---

func TestArgvPreservesQuoting(t *testing.T) {
	stdout, _, code := runCoax(t, []string{"--times", "1", "--", "printf", `%s\n`, "a b"}, "")
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if stdout != "a b\n" {
		t.Errorf("stdout = %q, want %q", stdout, "a b\n")
	}
}

func TestArgvNoShellInjection(t *testing.T) {
	stdout, _, code := runCoax(t, []string{"--times", "1", "--", "echo", "x; echo INJECTED"}, "")
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if stdout != "x; echo INJECTED\n" {
		t.Errorf("stdout = %q, want the argument echoed literally", stdout)
	}
}

func TestShCExitCodePassthrough(t *testing.T) {
	_, _, code := runCoax(t, []string{"--times", "1", "--backoff", "1ms", "--", "sh", "-c", "exit 3"}, "")
	if code != 3 {
		t.Fatalf("expected code 3, got %d", code)
	}
}

func TestCommandNotFound(t *testing.T) {
	stdout, stderr, code := runCoax(t, []string{"--times", "2", "--", "vrk-no-such-cmd-xyz", "arg"}, "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty, got %q", stdout)
	}
	if !strings.Contains(stderr, "coax: command not found: vrk-no-such-cmd-xyz") {
		t.Errorf("stderr = %q, want command not found message", stderr)
	}
	if strings.Contains(stderr, "retrying") {
		t.Errorf("a missing command must not be retried, got: %q", stderr)
	}
}

func TestCommandNotFoundJSON(t *testing.T) {
	stdout, stderr, code := runCoax(t, []string{"--json", "--", "vrk-no-such-cmd-xyz"}, "")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	if !strings.Contains(stdout, `"error":"coax: command not found: vrk-no-such-cmd-xyz"`) {
		t.Errorf("stdout = %q, want JSON error", stdout)
	}
}

// --- stdin ---

// An open pipe that never closes must not block a child that ignores stdin.
func TestOpenStdinDoesNotHang(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })

	done := make(chan int, 1)
	go func() {
		_, _, code := runCoaxStdin(t, []string{"--times", "1", "--", "true"}, r)
		done <- code
	}()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("expected code 0, got %d", code)
		}
	case <-time.After(time.Second):
		t.Fatal("coax blocked on an open stdin pipe")
	}
}

func TestStdinReplayedOnRetry(t *testing.T) {
	stdout, _, code := runCoax(t, []string{"--times", "2", "--backoff", "1ms", "--", "sh", "-c", "cat; exit 1"}, "abc")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if stdout != "abcabcabc" {
		t.Errorf("stdout = %q, want %q", stdout, "abcabcabc")
	}
}

// A child that reads only part of stdin: the next attempt gets the bytes
// seen so far plus the unread rest, so every attempt sees the full input.
func TestStdinReplayAfterPartialRead(t *testing.T) {
	script := `head -c 2; exit 1`
	stdout, _, code := runCoax(t, []string{"--times", "1", "--", "sh", "-c", script}, "abcdef")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if stdout != "abab" {
		t.Errorf("stdout = %q, want %q", stdout, "abab")
	}
}

func TestStdinTooLargeToReplay(t *testing.T) {
	orig := maxReplay
	maxReplay = 4
	t.Cleanup(func() { maxReplay = orig })

	_, stderr, code := runCoax(t, []string{"--times", "1", "--", "sh", "-c", "cat >/dev/null; exit 1"}, "abcdefgh")
	if code != 1 {
		t.Fatalf("expected code 1, got %d", code)
	}
	if !strings.Contains(stderr, "coax: stdin larger than 64 MiB cannot be replayed") {
		t.Errorf("stderr = %q, want replay limit message", stderr)
	}
}

// Over the replay limit is fine when no retry is needed: input streams through.
func TestStdinLargeNoRetryStreams(t *testing.T) {
	orig := maxReplay
	maxReplay = 4
	t.Cleanup(func() { maxReplay = orig })

	stdout, _, code := runCoax(t, []string{"--times", "1", "--", "cat"}, "abcdefgh")
	if code != 0 {
		t.Fatalf("expected code 0, got %d", code)
	}
	if stdout != "abcdefgh" {
		t.Errorf("stdout = %q, want full input", stdout)
	}
}

// --- signals ---

func TestChildKilledBySignalExitCode(t *testing.T) {
	_, _, code := runCoax(t, []string{"--times", "1", "--on", "1", "--", "sh", "-c", "kill -TERM $$"}, "")
	if code != 128+int(syscall.SIGTERM) {
		t.Fatalf("expected code %d, got %d", 128+int(syscall.SIGTERM), code)
	}
}

func TestSignalForwardedToChild(t *testing.T) {
	go func() {
		time.Sleep(300 * time.Millisecond)
		_ = syscall.Kill(os.Getpid(), syscall.SIGTERM)
	}()
	start := time.Now()
	_, stderr, code := runCoax(t, []string{"--times", "3", "--", "sleep", "5"}, "")
	if time.Since(start) > 3*time.Second {
		t.Fatal("child was not stopped by the forwarded signal")
	}
	if code != 128+int(syscall.SIGTERM) {
		t.Fatalf("expected code %d, got %d", 128+int(syscall.SIGTERM), code)
	}
	if strings.Contains(stderr, "retrying") {
		t.Errorf("coax must not retry after it was signalled, got: %q", stderr)
	}
}
