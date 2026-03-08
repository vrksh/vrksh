package jwt

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

const (
	// validJWT has exp: 2524608000 (year 2050) — always in the future.
	// Payload: {"sub":"1234567890","name":"John Doe","admin":true,"exp":2524608000,"iat":1516239022}
	validJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImV4cCI6MjUyNDYwODAwMCwiaWF0IjoxNTE2MjM5MDIyfQ.1n2qLms2Fy9TOojNHoEplIoS0Oyu4PKT3wYwRv5_0Ok"

	// expiredJWT has exp: 1772983763 — in the past as of 2026-03-08.
	expiredJWT = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiYWRtaW4iOnRydWUsImV4cCI6MTc3Mjk4Mzc2MywiaWF0IjoxNTE2MjM5MDIyfQ.Ox-nWmGb-ehO0U38wefNLdP18uC6-HjGum6pcNXVVM4"
)

// runJWT replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. t.Cleanup restores the originals.
// Do not call t.Parallel() on tests that use this helper — they share global state.
func runJWT(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

	// stdin: always provide a pipe so Run() never blocks waiting for a terminal.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if stdinContent != "" {
		if _, err := stdinW.WriteString(stdinContent); err != nil {
			t.Fatalf("write stdin: %v", err)
		}
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

	os.Args = append([]string{"jwt"}, args...)

	code = Run()

	// Close write ends before reading so io.Copy sees EOF.
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

func TestValidTokenDefaultOutput(t *testing.T) {
	stdout, _, code := runJWT(t, []string{validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if payload["sub"] != "1234567890" {
		t.Errorf("sub = %v, want %q", payload["sub"], "1234567890")
	}
}

func TestValidTokenStdin(t *testing.T) {
	stdout, _, code := runJWT(t, []string{}, validJWT)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if payload["sub"] != "1234567890" {
		t.Errorf("sub = %v, want %q", payload["sub"], "1234567890")
	}
}

func TestValidTokenClaimFound(t *testing.T) {
	stdout, _, code := runJWT(t, []string{"--claim", "sub", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	got := strings.TrimSpace(stdout)
	if got != "1234567890" {
		t.Errorf("stdout = %q, want %q", got, "1234567890")
	}
}

func TestValidTokenClaimMissing(t *testing.T) {
	stdout, stderr, code := runJWT(t, []string{"--claim", "missing_field", validJWT}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "not found") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "not found")
	}
}

func TestValidTokenJSONFlag(t *testing.T) {
	stdout, _, code := runJWT(t, []string{"--json", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	for _, key := range []string{"header", "payload", "expires_in"} {
		if _, ok := envelope[key]; !ok {
			t.Errorf("envelope missing key %q", key)
		}
	}
}

func TestValidTokenJSONFlagExpiresIn(t *testing.T) {
	stdout, _, code := runJWT(t, []string{"--json", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	expiresIn, ok := envelope["expires_in"].(string)
	if !ok {
		t.Fatalf("expires_in is not a string: %T %v", envelope["expires_in"], envelope["expires_in"])
	}
	if expiresIn == "" || expiresIn == "expired" {
		t.Errorf("expires_in = %q, want a future duration string", expiresIn)
	}
}

func TestExpiredTokenDefaultOutput(t *testing.T) {
	// Default mode does NOT check expiry — expired token should still decode and exit 0.
	stdout, _, code := runJWT(t, []string{expiredJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (default path does not check expiry)", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
}

func TestExpiredTokenExpiredFlag(t *testing.T) {
	stdout, stderr, code := runJWT(t, []string{"--expired", expiredJWT}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "token expired") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "token expired")
	}
}

func TestValidTokenExpiredFlag(t *testing.T) {
	// Valid (far-future) token with --expired should exit 0 and print payload.
	stdout, _, code := runJWT(t, []string{"--expired", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
}

func TestInvalidToken(t *testing.T) {
	stdout, stderr, code := runJWT(t, []string{"notajwt"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "invalid JWT") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "invalid JWT")
	}
}

func TestNoInputTTY(t *testing.T) {
	// No args, empty stdin (simulates no piped input) — must exit 2 with usage hint.
	stdout, stderr, code := runJWT(t, []string{}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty, got %q", stdout)
	}
	if stderr == "" {
		t.Error("stderr must contain a usage hint, got empty")
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runJWT(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout == "" {
		t.Error("stdout must contain usage text, got empty")
	}
}

// TestPropertyAnyValidJWT verifies that a syntactically valid JWT (three dot-separated
// parts) never triggers a usage error (exit 2). Exit 2 is for missing input or bad flags,
// not for tokens that fail to decode.
func TestPropertyAnyValidJWT(t *testing.T) {
	cases := []string{
		validJWT,
		expiredJWT,
		"e30.e30.e30",
		"eyJhbGciOiJub25lIn0.!!!.sig",
		"eyJhbGciOiJub25lIn0.bm90anNvbg.sig",
	}
	for _, tok := range cases {
		_, _, code := runJWT(t, []string{tok}, "")
		if code == 2 {
			t.Errorf("token %q: exit 2 (usage error) — must be 0 or 1 only", tok)
		}
	}
}

func FuzzJwt(f *testing.F) {
	f.Add(validJWT)
	f.Add(expiredJWT)
	f.Add("notajwt")
	f.Add("")
	f.Add("a.b.c")
	f.Add("e30.e30.e30")

	f.Fuzz(func(t *testing.T, input string) {
		_, _, code := runJWT(t, []string{input}, "")
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("exit code = %d, want 0, 1, or 2", code)
		}
	})
}
