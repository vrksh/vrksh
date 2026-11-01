package mask

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"math/rand"
	"os"
	"strings"
	"testing"
)

// runMask replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and exit code. Restores all globals via t.Cleanup.
// Do not call t.Parallel() — tests share os.Stdin/Stdout/Stderr global state.
func runMask(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	os.Args = append([]string{"mask"}, args...)
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

// errReader is an io.Reader that always returns an error on Read.
type errReader struct{ err error }

func (e *errReader) Read(_ []byte) (int, error) { return 0, e.err }

// --- Built-in patterns ---

func TestBuiltinBearer(t *testing.T) {
	stdout, _, code := runMask(t, nil, "Authorization: Bearer sk-ant-abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "Authorization: Bearer [REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBuiltinPassword(t *testing.T) {
	stdout, _, code := runMask(t, nil, "password=hunter2\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "password=[REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBuiltinSecret(t *testing.T) {
	stdout, _, code := runMask(t, nil, "secret=abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "secret=[REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBuiltinApiKey(t *testing.T) {
	stdout, _, code := runMask(t, nil, "api_key=abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "api_key=[REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestBuiltinToken(t *testing.T) {
	stdout, _, code := runMask(t, nil, "token=abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "token=[REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

// --- Entropy ---

func TestEntropyDefaultThreshold(t *testing.T) {
	// zXpNrT5wYbsH8cDgQmVk — 20 unique chars, H = log2(20) ≈ 4.32 > 4.0.
	// No builtin pattern matches, so entropy detection must fire.
	stdout, _, code := runMask(t, nil, "zXpNrT5wYbsH8cDgQmVk\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "[REDACTED]") {
		t.Errorf("stdout = %q: expected [REDACTED] for high-entropy token", stdout)
	}
}

func TestEntropyFlagLower(t *testing.T) {
	// sk-ant-AAABBBCCC111222333444555 has entropy ≈ 3.6 — redacted at --entropy 3.0
	// but not at default 4.0. No builtin pattern matches this input either.
	stdout, _, code := runMask(t, []string{"--entropy", "3.0"}, "sk-ant-AAABBBCCC111222333444555\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "[REDACTED]") {
		t.Errorf("stdout = %q: expected [REDACTED] at --entropy 3.0", stdout)
	}
}

func TestEntropyFlagHigher(t *testing.T) {
	// abc123 is 6 chars — below the 8-char token floor, never entropy-checked.
	// With --entropy 4.5 and no builtin match, output is unchanged.
	stdout, _, code := runMask(t, []string{"--entropy", "4.5"}, "abc123\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "[REDACTED]") {
		t.Errorf("stdout = %q: expected no redaction for short token below length floor", stdout)
	}
}

// --- Custom patterns ---

func TestCustomPatternSingle(t *testing.T) {
	stdout, _, code := runMask(t, []string{"--pattern", `sk-ant-[A-Za-z0-9]+`}, "key: sk-ant-AAAA\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	want := "key: [REDACTED]\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
}

func TestCustomPatternRepeated(t *testing.T) {
	// Two distinct --pattern flags both fire; both regex strings appear in patterns_matched.
	stdout, _, code := runMask(t,
		[]string{"--pattern", `PAT1-[A-Za-z]+`, "--pattern", `PAT2-[A-Za-z]+`, "--json"},
		"value: PAT1-AAABBB and PAT2-CCCDDD\n",
	)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 lines (text + metadata), got %d: %q", len(lines), stdout)
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &meta); err != nil {
		t.Fatalf("last line not valid JSON: %v\nline: %q", err, lines[len(lines)-1])
	}
	pm, ok := meta["patterns_matched"].([]interface{})
	if !ok {
		t.Fatalf("patterns_matched is not array: %T %v", meta["patterns_matched"], meta["patterns_matched"])
	}
	var names []string
	for _, v := range pm {
		if s, ok := v.(string); ok {
			names = append(names, s)
		}
	}
	hasPAT1, hasPAT2 := false, false
	for _, n := range names {
		if n == `PAT1-[A-Za-z]+` {
			hasPAT1 = true
		}
		if n == `PAT2-[A-Za-z]+` {
			hasPAT2 = true
		}
	}
	if !hasPAT1 || !hasPAT2 {
		t.Errorf("patterns_matched = %v, want both PAT1-[A-Za-z]+ and PAT2-[A-Za-z]+", names)
	}
}

func TestInvalidPatternExitsBeforeStdin(t *testing.T) {
	// An invalid regex must produce exit 2 before any stdin is consumed.
	_, stderr, code := runMask(t, []string{"--pattern", "[bad"}, "should not be read\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage error message, got empty")
	}
}

// --- Multi-line input ---

func TestMultiLine(t *testing.T) {
	// Only the line with a secret should change; clean lines pass through unchanged.
	input := "line1\nBearer abc123xyz\nline3\n"
	stdout, _, code := runMask(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	parts := strings.Split(stdout, "\n")
	// parts[0]="line1", parts[1]="Bearer [REDACTED]", parts[2]="line3", parts[3]=""
	if len(parts) < 4 {
		t.Fatalf("expected 4 output parts (3 lines + trailing empty), got %d: %q", len(parts), stdout)
	}
	if parts[0] != "line1" {
		t.Errorf("line 0 = %q, want %q", parts[0], "line1")
	}
	if parts[1] != "Bearer [REDACTED]" {
		t.Errorf("line 1 = %q, want %q", parts[1], "Bearer [REDACTED]")
	}
	if parts[2] != "line3" {
		t.Errorf("line 2 = %q, want %q", parts[2], "line3")
	}
}

func TestUnchangedInput(t *testing.T) {
	// Text with no secrets passes through byte-for-byte identical, exit 0.
	input := "no secrets here\n"
	stdout, _, code := runMask(t, nil, input)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != input {
		t.Errorf("stdout = %q, want %q (unchanged)", stdout, input)
	}
}

// --- Empty line ---

func TestEmptyLine(t *testing.T) {
	// echo '' outputs a single newline; mask should echo it back unchanged.
	stdout, _, code := runMask(t, nil, "\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "\n" {
		t.Errorf("stdout = %q, want %q (empty line preserved)", stdout, "\n")
	}
}

// --- --json flag ---

func TestJSONNormalShape(t *testing.T) {
	stdout, _, code := runMask(t, []string{"--json"}, "Authorization: Bearer sk-ant-abc123def456\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimRight(stdout, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected ≥2 lines (text + metadata), got %d: %q", len(lines), stdout)
	}
	// First line: redacted text output.
	if !strings.Contains(lines[0], "[REDACTED]") {
		t.Errorf("first line = %q: expected [REDACTED]", lines[0])
	}
	// Last line: JSON metadata envelope.
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &meta); err != nil {
		t.Fatalf("last line not valid JSON: %v\nline: %q", err, lines[len(lines)-1])
	}
	if meta["_vrk"] != "mask" {
		t.Errorf("_vrk = %v, want %q", meta["_vrk"], "mask")
	}
	if meta["lines"] != float64(1) {
		t.Errorf("lines = %v, want 1", meta["lines"])
	}
	if meta["redacted"] != float64(1) {
		t.Errorf("redacted = %v, want 1", meta["redacted"])
	}
	if _, ok := meta["patterns_matched"]; !ok {
		t.Error("metadata missing patterns_matched field")
	}
}

func TestIOErrorExit1(t *testing.T) {
	// Inject an I/O error via stdinReader. Without --json:
	//   stderr: "error: mask: reading stdin: ..."
	//   stdout: empty
	//   Run() returns 1
	origReader := stdinReader
	stdinReader = &errReader{err: errors.New("injected read error")}
	t.Cleanup(func() { stdinReader = origReader })

	stdout, stderr, code := runMask(t, nil, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (I/O error must not silently exit 0)", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty on error", stdout)
	}
	if !strings.Contains(stderr, "mask: reading stdin") {
		t.Errorf("stderr = %q, want 'mask: reading stdin'", stderr)
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	// Inject an I/O error via stdinReader. With --json:
	//   stdout: {"error":"...","code":1}
	//   stderr: empty
	//   Run() returns 1
	origReader := stdinReader
	stdinReader = &errReader{err: errors.New("injected read error")}
	t.Cleanup(func() { stdinReader = origReader })

	stdout, stderr, code := runMask(t, []string{"--json"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty (errors go to stdout with --json active)", stderr)
	}
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Fatal("stdout is empty, want JSON error envelope")
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(trimmed), &m); err != nil {
		t.Fatalf("stdout not valid JSON: %v\nstdout: %q", err, stdout)
	}
	if m["error"] == nil {
		t.Error("JSON envelope missing 'error' field")
	}
	if m["code"] != float64(1) {
		t.Errorf("JSON envelope code = %v, want 1", m["code"])
	}
}

// --- Usage errors ---

func TestInteractiveTTY(t *testing.T) {
	// Simulate TTY by overriding the isTerminal var to always return true.
	origTTY := isTerminal
	isTerminal = func(_ int) bool { return true }
	t.Cleanup(func() { isTerminal = origTTY })

	_, stderr, code := runMask(t, nil, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (TTY → usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage error message, got empty")
	}
}

// TestInteractiveTTYJSON verifies that --json routes the TTY usage error to
// stdout as a JSON envelope and leaves stderr empty.
func TestInteractiveTTYJSON(t *testing.T) {
	origTTY := isTerminal
	isTerminal = func(_ int) bool { return true }
	t.Cleanup(func() { isTerminal = origTTY })

	stdout, stderr, code := runMask(t, []string{"--json"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (TTY + --json → usage error via JSON)", code)
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

func TestUnknownFlag(t *testing.T) {
	_, stderr, code := runMask(t, []string{"--unknown-flag"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stderr == "" {
		t.Error("stderr must contain usage error message, got empty")
	}
}

// --- Property tests ---

func TestPropertyExitCodesOnly(t *testing.T) {
	// Any valid invocation must produce exit code 0, 1, or 2 — never anything else.
	cases := []struct {
		args  []string
		input string
	}{
		{nil, "no secrets here\n"},
		{nil, "Bearer abc123xyz\n"},
		{nil, ""},
		{[]string{"--json"}, "hello world\n"},
		{[]string{"--entropy", "2.0"}, "zXpNrT5wYbsH8cDgQmVk\n"},
	}
	for _, tc := range cases {
		_, _, code := runMask(t, tc.args, tc.input)
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("args=%v input=%q: exit code = %d, want 0, 1, or 2", tc.args, tc.input, code)
		}
	}
}

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on error.
// Exit code is unaffected.
func TestQuietSuppressesStderr(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	_, stderr, code := runMask(t, []string{"--quiet"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage: no piped input)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	stdout, stderr, code := runMask(t, []string{"--quiet"}, "hello world")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	if !strings.Contains(stdout, "hello world") {
		t.Errorf("stdout = %q, want it to contain the input line", stdout)
	}
}

func TestPropertyRedactedNeverLeaksOriginal(t *testing.T) {
	// After redaction, the original secret value must not appear anywhere in stdout.
	secret := "sk-ant-SuperSecretValue123"
	input := "Authorization: Bearer " + secret + "\n"
	stdout, _, _ := runMask(t, nil, input)
	if strings.Contains(stdout, secret) {
		t.Errorf("stdout contains original secret %q: %q", secret, stdout)
	}
}

// --- Corpus: recall on secrets, no false positives on decoys ---

func readCorpus(t *testing.T, name string) []string {
	t.Helper()
	b, err := os.ReadFile("../../testdata/mask/" + name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
}

func TestCorpusSecretsRedacted(t *testing.T) {
	for _, line := range readCorpus(t, "secrets.txt") {
		out, _ := redactLine(line, nil, 4.0)
		if !strings.Contains(out, "[REDACTED]") {
			t.Errorf("secret not redacted: %q", line)
		}
	}
}

func TestCorpusDecoysUnchanged(t *testing.T) {
	for _, line := range readCorpus(t, "decoys.txt") {
		if out, _ := redactLine(line, nil, 4.0); out != line {
			t.Errorf("decoy changed:\n  in:  %q\n  out: %q", line, out)
		}
	}
}

func TestSecretValuesDoNotLeak(t *testing.T) {
	cases := []struct{ in, secret string }{
		{"https://api.example.com/v1/data?api_key=Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3Hd0Gf5Ja8U", "Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3Hd0Gf5Ja8U"},
		{"https://user:Zx8Pq2Lm7Nv4Rt9Ks1@git.example.com/repo.git", "Zx8Pq2Lm7Nv4Rt9Ks1"},
		{"DATABASE_URL=postgres://admin:Tr0ub4dor&3xK9@db.internal:5432/prod", "Tr0ub4dor&3xK9"},
		{"see https://x.example.com/cb?token=Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3Hd0Gf5Ja8U&page=2", "Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3Hd0Gf5Ja8U"},
		{"see https://x.example.com/cb?SIG=Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3Hd0Gf5Ja8U", "Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3Hd0Gf5Ja8U"},
		{"(sk-proj-FAKE0000example0000notreal0000000)", "FAKE0000example0000notreal0000000"},
		{"/tmp/sk-proj-FAKE0000example0000notreal0000000", "FAKE0000example0000notreal0000000"},
		{"registry.example.com/img:ghp_FAKE0000example0000notreal0000000000", "xK9mN2pL4qR7sT0uW3yZ5bD8fH1jM6nP9rV2"},
	}
	for _, c := range cases {
		out, _ := redactLine(c.in, nil, 4.0)
		if strings.Contains(out, c.secret) {
			t.Errorf("secret leaked: in %q, out %q", c.in, out)
		}
	}
}

func TestKeepsPunctuation(t *testing.T) {
	cases := []struct{ in, want string }{
		{"key sk-proj-FAKE0000example0000notreal0000000, please", "key [REDACTED], please"},
		{"(sk-proj-FAKE0000example0000notreal0000000).", "([REDACTED])."},
		{`"sk-proj-FAKE0000example0000notreal0000000";`, `"[REDACTED]";`},
		{"see https://api.example.com/v1?sig=Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3Hd0Gf5Ja8U&page=2.", "see https://api.example.com/v1?sig=[REDACTED]&page=2."},
		{"hook https://hooks.example.com/services/T0001/B0001/Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3", "hook https://hooks.example.com/services/T0001/B0001/[REDACTED]"},
		{"at /tmp/Zx8Pq2Lm7Nv4Rt9Ks1Wc6Yb3/out.log;", "at /tmp/[REDACTED]/out.log;"},
	}
	for _, c := range cases {
		if out, _ := redactLine(c.in, nil, 4.0); out != c.want {
			t.Errorf("redactLine(%q) = %q, want %q", c.in, out, c.want)
		}
	}
}

func TestDeployLogKeepsContext(t *testing.T) {
	in := "GET https://payments.internal/v1/charges failed\npulling registry.example.com/api-gateway:2.31.4\n"
	stdout, _, code := runMask(t, nil, in)
	if code != 0 || stdout != in {
		t.Errorf("exit %d, stdout %q, want %q", code, stdout, in)
	}
}

// TXT-1: quoted JSON/YAML keys, names ending in a secret word, XML, flags.
func TestKeywordForms(t *testing.T) {
	cases := []struct{ in, secret, want string }{
		{`{"password": "hunter2hunter2"}`, "hunter2", `{"password": "[REDACTED]"}`},
		{`{"password":"hunter2"}`, "hunter2", `{"password":"[REDACTED]"}`},
		{`"Password" : "hunter2"`, "hunter2", `"Password" : "[REDACTED]"`},
		{`{"token": "d8ebd4e10d21ef6bb07f93a5e80c56d8271b835c"}`, "d8ebd4e1", `{"token": "[REDACTED]"}`},
		{`{"api_key": "3f2a9c1e-7b4d-4e8a-9c2f-1a2b3c4d5e6f"}`, "3f2a9c1e", `{"api_key": "[REDACTED]"}`},
		{`{"id": 0, "password": "pw0", "token": "abc\"def"}`, "pw0", `{"id": 0, "password": "[REDACTED]", "token": "[REDACTED]"}`},
		{`DB_PASS=hunter2hunter2`, "hunter2", `DB_PASS=[REDACTED]`},
		{`<password>hunter2hunter2</password>`, "hunter2", `<password>[REDACTED]</password>`},
		{`--password hunter2hunter2`, "hunter2", `--password [REDACTED]`},
		{`password = correct horse battery staple`, "horse", `password = [REDACTED]`},
		{`  db_password: 'correct horse'`, "horse", `  db_password: '[REDACTED]'`},
		{`client_secret=abc123`, "abc123", `client_secret=[REDACTED]`},
		{`X-API-Key: abcd1234`, "abcd1234", `X-API-Key: [REDACTED]`},
		{`error: login failed, password=hunter2 user=bob`, "hunter2", `error: login failed, password=[REDACTED] user=bob`},
	}
	for _, c := range cases {
		out, _ := redactLine(c.in, nil, 4.0)
		if strings.Contains(out, c.secret) || out != c.want {
			t.Errorf("redactLine(%q)\n  got  %q\n  want %q", c.in, out, c.want)
		}
	}
}

// TXT-1: patterns_matched keeps the old category names for the new forms.
func TestKeywordFormsNames(t *testing.T) {
	cases := map[string]string{
		`{"password": "x1"}`:    "password",
		`{"client_secret":"x"}`: "secret",
		`{"api_key": "x"}`:      "api_key",
		`"access_token": "x"`:   "token",
	}
	for in, name := range cases {
		_, fired := redactLine(in, nil, 4.0)
		if len(fired) == 0 || fired[0] != name {
			t.Errorf("%q fired %v, want %s first", in, fired, name)
		}
	}
}

// TXT-2: Basic auth, Authorization: token, URL userinfo, curl -u.
func TestAuthCarriers(t *testing.T) {
	cases := []struct{ in, secret string }{
		{"Authorization: Basic YWRtaW46aHVudGVyMg==", "YWRtaW46aHVudGVyMg"},
		{"Authorization: token 3f2a9c1e7b4d4e8a9c2f1a2b3c4d5e6f7a8b9c0d", "3f2a9c1e"},
		{`"authorization": "Basic YWRtaW46aHVudGVyMg=="`, "YWRtaW46aHVudGVyMg"},
		{"amqp://guest:guest@mq", "guest@"},
		{"https://u:p4ss@x.io", "p4ss"},
		{"curl -u admin:hunter2 https://x", "hunter2"},
		{"curl --user admin:hunter2 https://x", "hunter2"},
	}
	for _, c := range cases {
		out, _ := redactLine(c.in, nil, 4.0)
		if strings.Contains(out, c.secret) || !strings.Contains(out, "[REDACTED]") {
			t.Errorf("redactLine(%q) = %q, leaks %q", c.in, out, c.secret)
		}
	}
}

// TXT-3: random hex can never reach 4.0 bits, so it gets its own check.
func TestHexSecrets(t *testing.T) {
	r := rand.New(rand.NewSource(1))
	b := make([]byte, 20)
	r.Read(b)
	h := hex.EncodeToString(b)
	for _, in := range []string{h, "https://hooks.example.com/hook/" + h, "webhook " + h + "."} {
		out, _ := redactLine(in, nil, 4.0)
		if strings.Contains(out, h) {
			t.Errorf("hex secret leaked: %q", out)
		}
	}
}

// Things that look close to a secret but are not.
func TestKeywordDecoys(t *testing.T) {
	for _, in := range []string{
		`{"tokens":42,"model":"o200k_base"}`,
		`{"input_tokens": 42, "max_tokens": 100}`,
		"https://example.com/a",
		"we have a token budget of 4000",
		"the basic idea is simple",
		"vrk tok --token-budget 100",
		"bypass: true",
		"compass: north",
		`{"password": ""}`,
		"0000000000000000000000000000000000000000",
		"merge 3f9a2c7e1b4d8f6a0c5e9b2d7f1a4c8e6b0d3f5a into main",
	} {
		if out, _ := redactLine(in, nil, 4.0); out != in {
			t.Errorf("decoy changed:\n  in:  %q\n  out: %q", in, out)
		}
	}
}
