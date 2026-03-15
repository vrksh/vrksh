package digest

import (
	"bytes"
	"encoding/json"
	"errors"
	"hash"
	"io"
	"os"
	"strings"
	"testing"
)

// runDigest replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runDigest(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"digest"}, args...)
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

// writeTempFile creates a temp file with the given content and returns its path.
func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp("", "digest-test-*")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(f.Name()) })
	if _, err := io.WriteString(f, content); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close temp file: %v", err)
	}
	return f.Name()
}

func TestHappyPathSHA256(t *testing.T) {
	stdout, _, code := runDigest(t, nil, "hello\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// After streaming fix: stdin bytes are hashed verbatim including the trailing \n.
	// sha256("hello\n") = 5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03
	want := "sha256:5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestHappyPathMD5(t *testing.T) {
	stdout, _, code := runDigest(t, []string{"--algo", "md5"}, "hello\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// md5("hello\n") = b1946ac92492d2347c6235b4d2611184
	want := "md5:b1946ac92492d2347c6235b4d2611184\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestHappyPathSHA512(t *testing.T) {
	stdout, _, code := runDigest(t, []string{"--algo", "sha512"}, "hello\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(stdout, "sha512:") {
		t.Errorf("stdout = %q, want sha512: prefix", stdout)
	}
	hexPart := strings.TrimPrefix(strings.TrimSuffix(stdout, "\n"), "sha512:")
	if len(hexPart) != 128 {
		t.Errorf("sha512 hex length = %d, want 128; hex=%q", len(hexPart), hexPart)
	}
}

func TestBare(t *testing.T) {
	stdout, _, code := runDigest(t, []string{"--bare"}, "hello\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// sha256("hello\n") bare
	want := "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if strings.Contains(strings.TrimSuffix(stdout, "\n"), ":") {
		t.Errorf("--bare output should not contain colon, got %q", stdout)
	}
}

func TestJSONOutput(t *testing.T) {
	stdout, _, code := runDigest(t, []string{"--json"}, "hello\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if obj["input_bytes"] != float64(6) {
		t.Errorf("input_bytes = %v, want 6 (raw bytes including newline)", obj["input_bytes"])
	}
	if obj["algo"] != "sha256" {
		t.Errorf("algo = %q, want sha256", obj["algo"])
	}
	// sha256("hello\n") — stdin bytes hashed verbatim after streaming fix
	if obj["hash"] != "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03" {
		t.Errorf("hash = %q, want sha256 of 'hello\\n'", obj["hash"])
	}
}

func TestPositionalArg(t *testing.T) {
	// Positional arg bypasses stdin entirely — TTY guard must not fire.
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runDigest(t, []string{"hello"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// Positional arg hashes the literal string — no newline appended.
	// sha256("hello") = 2cf24dba... (different from stdin "hello\n" after streaming fix).
	want := "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestPositionalArgJSONInputBytes(t *testing.T) {
	// Positional arg: input_bytes = len("hello") = 5, not 6.
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runDigest(t, []string{"--json", "hello"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("not valid JSON: %v, got %q", err, stdout)
	}
	if obj["input_bytes"] != float64(5) {
		t.Errorf("input_bytes = %v, want 5 (no newline in positional arg)", obj["input_bytes"])
	}
}

func TestEmptyStdin(t *testing.T) {
	stdout, _, code := runDigest(t, nil, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855\n"
	if stdout != want {
		t.Errorf("stdout = %q, want hash of empty string", stdout)
	}
}

func TestEmptyStdinEcho(t *testing.T) {
	// echo '' pipes a single \n byte — after streaming fix, that byte IS hashed.
	// sha256("\n") = 01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b
	stdout, _, code := runDigest(t, nil, "\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "sha256:01ba4719c80b6fe911b091a7c05124b64eeece964e09c058ef8f9805daca546b\n"
	if stdout != want {
		t.Errorf("stdout = %q, want sha256 of newline byte", stdout)
	}
}

func TestFileHash(t *testing.T) {
	// File content "hello" — no newline stripping for files.
	path := writeTempFile(t, "hello")
	stdout, _, code := runDigest(t, []string{"--file", path}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "sha256:2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestFileNotFound(t *testing.T) {
	_, _, code := runDigest(t, []string{"--file", "/no/such/file/digest-test-xyz"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestCompareMatch(t *testing.T) {
	f1 := writeTempFile(t, "hello")
	f2 := writeTempFile(t, "hello")
	stdout, _, code := runDigest(t, []string{"--file", f1, "--file", f2, "--compare"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "match: true" {
		t.Errorf("stdout = %q, want 'match: true'", stdout)
	}
}

func TestCompareMismatch(t *testing.T) {
	f1 := writeTempFile(t, "hello")
	f2 := writeTempFile(t, "world")
	stdout, _, code := runDigest(t, []string{"--file", f1, "--file", f2, "--compare"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (mismatch is informational, not an error)", code)
	}
	if strings.TrimSpace(stdout) != "match: false" {
		t.Errorf("stdout = %q, want 'match: false'", stdout)
	}
}

func TestCompareJSON(t *testing.T) {
	f1 := writeTempFile(t, "hello")
	f2 := writeTempFile(t, "hello")
	stdout, _, code := runDigest(t, []string{"--file", f1, "--file", f2, "--compare", "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("not valid JSON: %v, got %q", err, stdout)
	}
	if obj["match"] != true {
		t.Errorf("match = %v, want true", obj["match"])
	}
	if obj["algo"] != "sha256" {
		t.Errorf("algo = %q, want sha256", obj["algo"])
	}
	files, ok := obj["files"].([]any)
	if !ok || len(files) != 2 {
		t.Errorf("files = %v, want 2-element array", obj["files"])
	}
	hashes, ok := obj["hashes"].([]any)
	if !ok || len(hashes) != 2 {
		t.Errorf("hashes = %v, want 2-element array", obj["hashes"])
	}
}

func TestHMAC(t *testing.T) {
	stdout, _, code := runDigest(t, []string{"--hmac", "--key", "secret"}, "payload\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(stdout, "sha256:") {
		t.Errorf("stdout = %q, want sha256: prefix", stdout)
	}
	hexPart := strings.TrimPrefix(strings.TrimSuffix(stdout, "\n"), "sha256:")
	if len(hexPart) != 64 {
		t.Errorf("HMAC hex length = %d, want 64; hex=%q", len(hexPart), hexPart)
	}
}

func TestHMACVerifyMatch(t *testing.T) {
	// Produce HMAC then verify — must exit 0.
	bare, _, code := runDigest(t, []string{"--hmac", "--key", "secret", "--bare"}, "payload\n")
	if code != 0 {
		t.Fatalf("produce HMAC: exit code = %d, want 0", code)
	}
	knownHMAC := strings.TrimSpace(bare)

	_, _, code = runDigest(t, []string{"--hmac", "--key", "secret", "--verify", knownHMAC}, "payload\n")
	if code != 0 {
		t.Fatalf("verify match: exit code = %d, want 0", code)
	}
}

func TestHMACVerifyMismatch(t *testing.T) {
	// 64-char hex string that won't match the real HMAC.
	wrongHMAC := strings.Repeat("de", 32)
	_, _, code := runDigest(t, []string{"--hmac", "--key", "secret", "--verify", wrongHMAC}, "payload\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (mismatch)", code)
	}
}

func TestHMACFile(t *testing.T) {
	// After the streaming fix, stdin bytes are hashed verbatim. Use printf-style
	// input (no trailing newline) to get the same bytes as the file.
	// Both hash "payload" (7 bytes) → identical HMAC.
	path := writeTempFile(t, "payload")
	stdout, _, code := runDigest(t, []string{"--hmac", "--key", "secret", "--file", path}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.HasPrefix(stdout, "sha256:") {
		t.Errorf("stdout = %q, want sha256: prefix", stdout)
	}
	fileHex := strings.TrimPrefix(strings.TrimSuffix(stdout, "\n"), "sha256:")

	// stdin path: "payload" (no newline) → hashes "payload" → same as file
	stdinBare, _, _ := runDigest(t, []string{"--hmac", "--key", "secret", "--bare"}, "payload")
	stdinHex := strings.TrimSpace(stdinBare)

	if fileHex != stdinHex {
		t.Errorf("file HMAC %q != stdin HMAC %q (both hash 'payload')", fileHex, stdinHex)
	}
}

func TestHelpExitsZero(t *testing.T) {
	stdout, _, code := runDigest(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "digest") {
		t.Errorf("--help stdout = %q, want it to contain 'digest'", stdout)
	}
}

func TestTTYNoFile(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runDigest(t, nil, "")
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

func TestTTYWithJSON(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, stderr, code := runDigest(t, []string{"--json"}, "")
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

func TestJSONErrorToStdout(t *testing.T) {
	origCopyToHash := copyToHash
	copyToHash = func(_ hash.Hash, _ io.Reader) (int64, error) {
		return 0, errors.New("simulated read error")
	}
	defer func() { copyToHash = origCopyToHash }()

	stdout, stderr, code := runDigest(t, []string{"--json"}, "any input")
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

func TestBareAndJSONMutuallyExclusive(t *testing.T) {
	_, _, code := runDigest(t, []string{"--bare", "--json"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestUnknownAlgo(t *testing.T) {
	_, _, code := runDigest(t, []string{"--algo", "crc32"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestHMACMissingKey(t *testing.T) {
	_, _, code := runDigest(t, []string{"--hmac"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestVerifyWithoutHMAC(t *testing.T) {
	_, _, code := runDigest(t, []string{"--verify", "abc123"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestCompareNeedsFiles(t *testing.T) {
	f1 := writeTempFile(t, "hello")
	_, _, code := runDigest(t, []string{"--file", f1, "--compare"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestCompareNoFiles(t *testing.T) {
	_, _, code := runDigest(t, []string{"--compare"}, "hello\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, code := runDigest(t, []string{"--notaflag"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on I/O error.
// Exit code is unaffected.
func TestQuietSuppressesStderr(t *testing.T) {
	origCopyToHash := copyToHash
	copyToHash = func(_ hash.Hash, _ io.Reader) (int64, error) {
		return 0, errors.New("simulated read error")
	}
	t.Cleanup(func() { copyToHash = origCopyToHash })

	_, stderr, code := runDigest(t, []string{"--quiet"}, "hello")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (I/O error)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	stdout, stderr, code := runDigest(t, []string{"--quiet"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	if !strings.Contains(stdout, "sha256:") {
		t.Errorf("stdout = %q, want hash output with sha256: prefix", stdout)
	}
}

func TestPropertyHMACVerify(t *testing.T) {
	// Property: for any input and key, produce HMAC then verify → always exit 0.
	cases := []struct {
		input string
		key   string
	}{
		{"hello\n", "secret"},
		{"test payload with spaces\n", "my-secret-key"},
		{"\n", "k"}, // empty after stripping
		{"", "key"}, // truly empty
		{"data\n", "k3y-w1th-sp3c1al-ch@rs!"},
	}
	for _, tc := range cases {
		bare, _, code := runDigest(t, []string{"--hmac", "--key", tc.key, "--bare"}, tc.input)
		if code != 0 {
			t.Errorf("input=%q key=%q: produce HMAC exit %d, want 0", tc.input, tc.key, code)
			continue
		}
		knownHMAC := strings.TrimSpace(bare)
		_, _, code = runDigest(t, []string{"--hmac", "--key", tc.key, "--verify", knownHMAC}, tc.input)
		if code != 0 {
			t.Errorf("input=%q key=%q: verify exit %d, want 0 (HMAC=%q)", tc.input, tc.key, code, knownHMAC)
		}
	}
}
