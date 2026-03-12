package kv

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// newTempDB creates a temporary SQLite database file for the duration of the
// test and returns its path. The file is removed when the test ends.
func newTempDB(t *testing.T) string {
	t.Helper()
	f, err := os.CreateTemp("", "vrk-kv-test-*.db")
	if err != nil {
		t.Fatalf("create temp db: %v", err)
	}
	_ = f.Close()
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	return f.Name()
}

// runKV saves global state, routes through the real Run(), and returns captured
// stdout, stderr, and the exit code. Each call within a test reuses the same
// dbPath so state persists across set/get sequences.
// Do not call t.Parallel() — tests share global os state.
func runKV(t *testing.T, dbPath string, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	// Point kv at our temp database.
	t.Setenv("VRK_KV_PATH", dbPath)

	// Stdin — always a pipe so Run() never blocks on a terminal.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if stdinContent != "" {
		if _, err := io.WriteString(stdinW, stdinContent); err != nil {
			t.Fatalf("write stdin: %v", err)
		}
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

	// Simulate what main.go does: strip the tool name so os.Args[1] is the subcommand.
	os.Args = append([]string{"kv"}, args...)
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

// --- basic CRUD ---

func TestSetGet(t *testing.T) {
	db := newTempDB(t)
	_, _, code := runKV(t, db, []string{"set", "mykey", "myvalue"}, "")
	if code != 0 {
		t.Fatalf("set exit %d, want 0", code)
	}
	stdout, _, code := runKV(t, db, []string{"get", "mykey"}, "")
	if code != 0 {
		t.Fatalf("get exit %d, want 0", code)
	}
	if got := strings.TrimRight(stdout, "\n"); got != "myvalue" {
		t.Errorf("get stdout = %q, want %q", got, "myvalue")
	}
}

func TestGetNotFound(t *testing.T) {
	db := newTempDB(t)
	stdout, stderr, code := runKV(t, db, []string{"get", "nonexistent"}, "")
	if code != 1 {
		t.Fatalf("get exit %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "key not found") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "key not found")
	}
}

func TestSetOverwrite(t *testing.T) {
	db := newTempDB(t)
	runKV(t, db, []string{"set", "mykey", "oldvalue"}, "")
	runKV(t, db, []string{"set", "mykey", "newvalue"}, "")
	stdout, _, code := runKV(t, db, []string{"get", "mykey"}, "")
	if code != 0 {
		t.Fatalf("get exit %d, want 0", code)
	}
	if got := strings.TrimRight(stdout, "\n"); got != "newvalue" {
		t.Errorf("get stdout = %q, want %q", got, "newvalue")
	}
}

func TestDel(t *testing.T) {
	db := newTempDB(t)
	runKV(t, db, []string{"set", "mykey", "myvalue"}, "")
	_, _, code := runKV(t, db, []string{"del", "mykey"}, "")
	if code != 0 {
		t.Fatalf("del exit %d, want 0", code)
	}
	stdout, stderr, code := runKV(t, db, []string{"get", "mykey"}, "")
	if code != 1 {
		t.Fatalf("get-after-del exit %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty, got %q", stdout)
	}
	if !strings.Contains(stderr, "key not found") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "key not found")
	}
}

// --- list ---

func TestList(t *testing.T) {
	db := newTempDB(t)
	for _, k := range []string{"cherry", "apple", "banana"} {
		runKV(t, db, []string{"set", k, "val"}, "")
	}
	stdout, _, code := runKV(t, db, []string{"list"}, "")
	if code != 0 {
		t.Fatalf("list exit %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("list line count = %d, want 3; output: %q", len(lines), stdout)
	}
	// Must be sorted alphabetically.
	want := []string{"apple", "banana", "cherry"}
	for i, w := range want {
		if lines[i] != w {
			t.Errorf("list[%d] = %q, want %q", i, lines[i], w)
		}
	}
}

func TestListEmpty(t *testing.T) {
	db := newTempDB(t)
	stdout, _, code := runKV(t, db, []string{"list"}, "")
	if code != 0 {
		t.Fatalf("list exit %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty for empty namespace, got %q", stdout)
	}
}

// --- empty string value ---

func TestSetEmptyValue(t *testing.T) {
	db := newTempDB(t)
	_, _, code := runKV(t, db, []string{"set", "mykey", ""}, "")
	if code != 0 {
		t.Fatalf("set empty exit %d, want 0", code)
	}
	stdout, _, code := runKV(t, db, []string{"get", "mykey"}, "")
	if code != 0 {
		t.Fatalf("get exit %d, want 0", code)
	}
	// get prints value + newline; empty value means stdout is exactly "\n".
	if stdout != "\n" {
		t.Errorf("get stdout = %q, want %q (empty value + newline)", stdout, "\n")
	}
}

// --- stdin value for set ---

func TestSetStdin(t *testing.T) {
	db := newTempDB(t)
	// echo appends \n — strip exactly one trailing newline before storing.
	_, _, code := runKV(t, db, []string{"set", "result"}, `{"status":"done"}`+"\n")
	if code != 0 {
		t.Fatalf("set stdin exit %d, want 0", code)
	}
	stdout, _, code := runKV(t, db, []string{"get", "result"}, "")
	if code != 0 {
		t.Fatalf("get exit %d, want 0", code)
	}
	if got := strings.TrimRight(stdout, "\n"); got != `{"status":"done"}` {
		t.Errorf("get stdout = %q, want %q", got, `{"status":"done"}`)
	}
}

// --- incr / decr ---

func TestIncr(t *testing.T) {
	db := newTempDB(t)
	for i, want := range []string{"1", "2", "3"} {
		stdout, _, code := runKV(t, db, []string{"incr", "counter"}, "")
		if code != 0 {
			t.Fatalf("incr[%d] exit %d, want 0", i, code)
		}
		if got := strings.TrimRight(stdout, "\n"); got != want {
			t.Errorf("incr[%d] stdout = %q, want %q", i, got, want)
		}
	}
}

func TestIncrBy(t *testing.T) {
	db := newTempDB(t)
	runKV(t, db, []string{"incr", "counter"}, "") // → 1
	runKV(t, db, []string{"incr", "counter"}, "") // → 2
	stdout, _, code := runKV(t, db, []string{"incr", "counter", "--by", "5"}, "")
	if code != 0 {
		t.Fatalf("incr --by 5 exit %d, want 0", code)
	}
	if got := strings.TrimRight(stdout, "\n"); got != "7" {
		t.Errorf("incr --by 5 stdout = %q, want %q", got, "7")
	}
}

func TestDecr(t *testing.T) {
	db := newTempDB(t)
	// incr to 6 first
	for i := 0; i < 6; i++ {
		runKV(t, db, []string{"incr", "counter"}, "")
	}
	stdout, _, code := runKV(t, db, []string{"decr", "counter"}, "")
	if code != 0 {
		t.Fatalf("decr exit %d, want 0", code)
	}
	if got := strings.TrimRight(stdout, "\n"); got != "5" {
		t.Errorf("decr stdout = %q, want %q", got, "5")
	}
}

func TestIncrOnMissing(t *testing.T) {
	db := newTempDB(t)
	stdout, _, code := runKV(t, db, []string{"incr", "counter"}, "")
	if code != 0 {
		t.Fatalf("incr-on-missing exit %d, want 0", code)
	}
	if got := strings.TrimRight(stdout, "\n"); got != "1" {
		t.Errorf("incr-on-missing stdout = %q, want %q", got, "1")
	}
}

func TestDecrOnMissing(t *testing.T) {
	db := newTempDB(t)
	stdout, _, code := runKV(t, db, []string{"decr", "counter"}, "")
	if code != 0 {
		t.Fatalf("decr-on-missing exit %d, want 0", code)
	}
	if got := strings.TrimRight(stdout, "\n"); got != "-1" {
		t.Errorf("decr-on-missing stdout = %q, want %q", got, "-1")
	}
}

func TestIncrNonNumeric(t *testing.T) {
	db := newTempDB(t)
	runKV(t, db, []string{"set", "counter", "notanumber"}, "")
	stdout, stderr, code := runKV(t, db, []string{"incr", "counter"}, "")
	if code != 1 {
		t.Fatalf("incr-non-numeric exit %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "value is not a number") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "value is not a number")
	}
}

// --- namespaces ---

func TestNamespaceIsolation(t *testing.T) {
	db := newTempDB(t)
	runKV(t, db, []string{"set", "--ns", "myjob", "step", "3"}, "")

	// Get from the same namespace → found.
	stdout, _, code := runKV(t, db, []string{"get", "--ns", "myjob", "step"}, "")
	if code != 0 {
		t.Fatalf("get --ns myjob exit %d, want 0", code)
	}
	if got := strings.TrimRight(stdout, "\n"); got != "3" {
		t.Errorf("get --ns myjob stdout = %q, want %q", got, "3")
	}

	// Get from default namespace → not found.
	_, stderr, code := runKV(t, db, []string{"get", "step"}, "")
	if code != 1 {
		t.Fatalf("get (no --ns) exit %d, want 1", code)
	}
	if !strings.Contains(stderr, "key not found") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "key not found")
	}
}

// --- TTL expiry ---

func TestTTLExpiry(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping TTL test in short mode (requires sleep)")
	}
	db := newTempDB(t)
	runKV(t, db, []string{"set", "expiring", "value", "--ttl", "1s"}, "")

	// Key is present immediately after set.
	_, _, code := runKV(t, db, []string{"get", "expiring"}, "")
	if code != 0 {
		t.Fatalf("get before expiry exit %d, want 0", code)
	}

	time.Sleep(2 * time.Second)

	stdout, stderr, code := runKV(t, db, []string{"get", "expiring"}, "")
	if code != 1 {
		t.Fatalf("get after expiry exit %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on expired key, got %q", stdout)
	}
	if !strings.Contains(stderr, "key not found") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "key not found")
	}
}

// --- dry-run ---

func TestDryRun(t *testing.T) {
	db := newTempDB(t)
	stdout, _, code := runKV(t, db, []string{"set", "result", "done", "--dry-run"}, "")
	if code != 0 {
		t.Fatalf("set --dry-run exit %d, want 0", code)
	}
	if !strings.Contains(stdout, "would set result = done") {
		t.Errorf("stdout = %q, want it to contain %q", stdout, "would set result = done")
	}

	// Dry-run must not write anything.
	_, _, code = runKV(t, db, []string{"get", "result"}, "")
	if code != 1 {
		t.Fatalf("get after dry-run exit %d, want 1 (key must not exist)", code)
	}
}

// --- usage errors ---

func TestUnknownSubcommand(t *testing.T) {
	db := newTempDB(t)
	stdout, _, code := runKV(t, db, []string{"bogus"}, "")
	if code != 2 {
		t.Fatalf("unknown subcommand exit %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
}

func TestNoSubcommand(t *testing.T) {
	db := newTempDB(t)
	stdout, _, code := runKV(t, db, []string{}, "")
	if code != 2 {
		t.Fatalf("no subcommand exit %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
}

func TestUnknownFlag(t *testing.T) {
	db := newTempDB(t)
	stdout, _, code := runKV(t, db, []string{"get", "--bogus", "key"}, "")
	if code != 2 {
		t.Fatalf("unknown flag exit %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
}

func TestHelp(t *testing.T) {
	db := newTempDB(t)
	stdout, _, code := runKV(t, db, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("--help exit %d, want 0", code)
	}
	if stdout == "" {
		t.Error("stdout must contain usage text, got empty")
	}
}

// --- concurrency ---

// TestIncrConcurrent calls incrVal directly from 10 goroutines sharing a single
// *sql.DB. This validates that the SQL read-modify-write is atomic under
// goroutine concurrency. Process-level concurrency is covered by the smoke test.
func TestIncrConcurrent(t *testing.T) {
	dbPath := newTempDB(t)
	t.Setenv("VRK_KV_PATH", dbPath)

	db, err := openDB()
	if err != nil {
		t.Fatalf("openDB: %v", err)
	}
	defer func() { _ = db.Close() }()

	const n = 10
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			if _, err := incrVal(db, "default", "counter", 1); err != nil {
				t.Errorf("incrVal: %v", err)
			}
		}()
	}
	wg.Wait()

	val, err := getVal(db, "default", "counter")
	if err != nil {
		t.Fatalf("getVal: %v", err)
	}
	if val != fmt.Sprintf("%d", n) {
		t.Errorf("counter = %q, want %q", val, fmt.Sprintf("%d", n))
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	// kv get on a missing key with --json must route the error to stdout as JSON.
	db := newTempDB(t)
	stdout, stderr, code := runKV(t, db, []string{"get", "--json", "nonexistent"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
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
	if key, _ := obj["key"].(string); key != "nonexistent" {
		t.Errorf("key = %q, want %q", key, "nonexistent")
	}
	if c, _ := obj["code"].(float64); int(c) != 1 {
		t.Errorf("code = %v, want 1", obj["code"])
	}
}
