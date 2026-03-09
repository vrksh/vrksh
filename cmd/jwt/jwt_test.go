package jwt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
)

// buildJWT constructs a minimal test JWT from a JSON payload string.
// The header is always {"alg":"HS256"} and the signature is always "fakesig".
// decodeJWT does not verify signatures, so this is valid for testing.
func buildJWT(payload string) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`))
	body := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return header + "." + body + ".fakesig"
}

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
	// New shape: {header, payload, signature, expired, valid} — no expires_in.
	stdout, _, code := runJWT(t, []string{"--json", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	for _, key := range []string{"header", "payload", "signature", "expired", "valid"} {
		if _, ok := envelope[key]; !ok {
			t.Errorf("envelope missing key %q", key)
		}
	}
	// expires_in must NOT appear in the new shape.
	if _, ok := envelope["expires_in"]; ok {
		t.Error("envelope must not contain deprecated key \"expires_in\"")
	}
}

func TestValidTokenJSONFlagExpiredValid(t *testing.T) {
	// validJWT has exp in year 2050 — expired=false, valid=true.
	stdout, _, code := runJWT(t, []string{"--json", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if expired, _ := envelope["expired"].(bool); expired {
		t.Errorf("expired = true, want false for a future-exp token")
	}
	if valid, _ := envelope["valid"].(bool); !valid {
		t.Errorf("valid = false, want true for a well-formed future-exp token")
	}
}

func TestExpiredTokenJSONFlagExpiredValid(t *testing.T) {
	// expiredJWT has exp in the past — expired=true, valid=false.
	stdout, _, code := runJWT(t, []string{"--json", expiredJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (default --json path never exits 1 for expiry)", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	if expired, _ := envelope["expired"].(bool); !expired {
		t.Errorf("expired = false, want true for a past-exp token")
	}
	if valid, _ := envelope["valid"].(bool); valid {
		t.Errorf("valid = true, want false for an expired token")
	}
}

func TestValidTokenJSONFlagSignature(t *testing.T) {
	// validJWT ends with a real signature — the signature field must be non-empty.
	stdout, _, code := runJWT(t, []string{"--json", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var envelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &envelope); err != nil {
		t.Fatalf("stdout is not valid JSON: %v", err)
	}
	sig, ok := envelope["signature"].(string)
	if !ok || sig == "" {
		t.Errorf("signature = %q, want non-empty string", sig)
	}
}

// --- --expired --json ---

func TestJSONExpiredFlagWithValidToken(t *testing.T) {
	// --expired --json with a valid (future-exp) token: stdout has {"expired":false}, exit 0.
	stdout, stderr, code := runJWT(t, []string{"--expired", "--json", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if expired, _ := obj["expired"].(bool); expired {
		t.Errorf("expired = true, want false for a future-exp token")
	}
}

func TestJSONExpiredFlagWithExpiredToken(t *testing.T) {
	// --expired --json with an expired token: stdout has {"expired":true}, exit 1.
	// When --json is active, error info goes to stdout as JSON; stderr is empty.
	stdout, stderr, code := runJWT(t, []string{"--expired", "--json", expiredJWT}, "")
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
	if expired, _ := obj["expired"].(bool); !expired {
		t.Errorf("expired = false, want true for a past-exp token")
	}
}

// --- --claim --json ---

func TestJSONClaimFound(t *testing.T) {
	// --claim sub --json → {"claim":"sub","value":"1234567890"}, exit 0.
	stdout, stderr, code := runJWT(t, []string{"--claim", "sub", "--json", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if claim, _ := obj["claim"].(string); claim != "sub" {
		t.Errorf("claim = %q, want %q", claim, "sub")
	}
	if value, _ := obj["value"].(string); value != "1234567890" {
		t.Errorf("value = %q, want %q", value, "1234567890")
	}
}

func TestJSONClaimMissing(t *testing.T) {
	// --claim missing --json → {"error":"...","code":1} on stdout, exit 1.
	stdout, stderr, code := runJWT(t, []string{"--claim", "missing_field", "--json", validJWT}, "")
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
	if code, _ := obj["code"].(float64); int(code) != 1 {
		t.Errorf("code = %v, want 1", obj["code"])
	}
}

// --- error JSON ---

func TestJSONInvalidToken(t *testing.T) {
	// Invalid token + --json → {"error":"...","code":1} on stdout, exit 1. Stderr empty.
	stdout, stderr, code := runJWT(t, []string{"--json", "notajwt"}, "")
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
	if code, _ := obj["code"].(float64); int(code) != 1 {
		t.Errorf("code = %v, want 1", obj["code"])
	}
}

func TestJSONNoInput(t *testing.T) {
	// No input + --json → {"error":"...","code":2} on stdout, exit 2. Stderr empty.
	stdout, stderr, code := runJWT(t, []string{"--json"}, "")
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
	if code, _ := obj["code"].(float64); int(code) != 2 {
		t.Errorf("code = %v, want 2", obj["code"])
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

// --- TrimSpace tests ---

// TestTrimSpaceOnInput verifies that leading/trailing whitespace in the token
// is stripped before decoding. Trailing whitespace that lands in the signature
// part is harmless (signature is not decoded), but leading whitespace corrupts
// the header. The test covers both paths: positional arg and stdin.
func TestTrimSpaceOnInput(t *testing.T) {
	// positional arg with leading spaces: without TrimSpace, parts[0] would be
	// "  eyJhbGci..." and base64 decoding would fail → exit 1.
	// With TrimSpace the leading spaces are stripped → exit 0.
	t.Run("leading whitespace in positional arg", func(t *testing.T) {
		stdout, _, code := runJWT(t, []string{"  " + validJWT + "  "}, "")
		if code != 0 {
			t.Fatalf("exit code = %d (arg with surrounding spaces), want 0", code)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
			t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
		}
		if payload["sub"] != "1234567890" {
			t.Errorf("sub = %v, want %q", payload["sub"], "1234567890")
		}
	})

	// stdin with \r\n: ReadInput strips the trailing \n leaving a \r before the
	// first dot — corrupting parts[0]. TrimSpace removes the \r.
	// Construct a token where \r lands in the header part, not the signature.
	// The simplest way: prepend \r so it becomes part of parts[0].
	t.Run("leading CR in positional arg", func(t *testing.T) {
		stdout, _, code := runJWT(t, []string{"\r" + validJWT}, "")
		if code != 0 {
			t.Fatalf("exit code = %d (arg with leading \\r), want 0", code)
		}
		var payload map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
			t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
		}
	})

	// stdin regression: echo "$TOKEN" appends \n — ReadInput strips it so
	// this already worked. Included to confirm no regression.
	t.Run("single trailing newline via stdin", func(t *testing.T) {
		stdoutWithNL, _, codeWithNL := runJWT(t, []string{}, validJWT+"\n")
		stdoutClean, _, codeClean := runJWT(t, []string{}, validJWT)
		if codeWithNL != 0 {
			t.Fatalf("exit code = %d (stdin with \\n), want 0", codeWithNL)
		}
		if codeClean != 0 {
			t.Fatalf("exit code = %d (clean stdin), want 0", codeClean)
		}
		if strings.TrimSpace(stdoutWithNL) != strings.TrimSpace(stdoutClean) {
			t.Errorf("outputs differ:\n  with \\n:  %q\n  without: %q", stdoutWithNL, stdoutClean)
		}
	})
}

// --- Multiple positional args test ---

// TestTooManyArguments verifies that passing more than one positional arg
// returns exit 2 (usage error) with a clear message, not a confusing
// "invalid JWT: expected 3 parts" runtime error.
func TestTooManyArguments(t *testing.T) {
	stdout, stderr, code := runJWT(t, []string{validJWT, validJWT}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
	if !strings.Contains(stderr, "too many arguments") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "too many arguments")
	}
}

// --- --valid flag tests ---

// TestValidFlagWithValidToken verifies that a well-formed token (exp in future,
// iat in past, no nbf) passes all --valid checks and exits 0 with payload JSON.
func TestValidFlagWithValidToken(t *testing.T) {
	// validJWT: exp=2524608000 (year 2050), iat=1516239022 (year 2018), no nbf.
	stdout, _, code := runJWT(t, []string{"--valid", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
}

// TestValidFlagWithExpiredToken verifies that --valid exits 1 and reports the
// expiry when the token's exp claim is in the past.
func TestValidFlagWithExpiredToken(t *testing.T) {
	stdout, stderr, code := runJWT(t, []string{"--valid", expiredJWT}, "")
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

// TestValidFlagNbfInFuture verifies that --valid exits 1 when nbf is in the
// far future (token not yet valid).
func TestValidFlagNbfInFuture(t *testing.T) {
	// nbf=9999999999 is year 2286 — always in the future.
	tok := buildJWT(`{"sub":"test","nbf":9999999999}`)
	stdout, stderr, code := runJWT(t, []string{"--valid", tok}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (nbf in future)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "not yet valid") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "not yet valid")
	}
}

// TestValidFlagNbfInPast verifies that --valid exits 0 when nbf is in the
// past (token is already valid per nbf).
func TestValidFlagNbfInPast(t *testing.T) {
	// nbf=1000000000 is year 2001 — always in the past.
	tok := buildJWT(`{"sub":"test","nbf":1000000000}`)
	stdout, _, code := runJWT(t, []string{"--valid", tok}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (nbf in past)", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
}

// TestValidFlagIatInFuture verifies that --valid exits 1 when iat is in the
// far future (token claims to be issued in the future — suspicious).
func TestValidFlagIatInFuture(t *testing.T) {
	// iat=9999999999 is year 2286 — always in the future.
	tok := buildJWT(`{"sub":"test","iat":9999999999}`)
	stdout, stderr, code := runJWT(t, []string{"--valid", tok}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (iat in future)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "issued in the future") {
		t.Errorf("stderr = %q, want it to contain %q", stderr, "issued in the future")
	}
}

// TestValidFlagNbfMissing verifies that --valid exits 0 for a token that has
// iat in the past but no nbf claim. A missing nbf must not be treated as
// "not yet valid".
func TestValidFlagNbfMissing(t *testing.T) {
	// iat=1000000000 (year 2001), no nbf, no exp.
	tok := buildJWT(`{"sub":"test","iat":1000000000}`)
	stdout, _, code := runJWT(t, []string{"--valid", tok}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (no nbf claim = nbf check skipped)", code)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
	}
}

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on error.
// Exit code is unaffected.
func TestQuietSuppressesStderr(t *testing.T) {
	stdout, stderr, code := runJWT(t, []string{"--quiet", "notajwt"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (runtime error)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	stdout, stderr, code := runJWT(t, []string{"--quiet", validJWT}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &payload); err != nil {
		t.Fatalf("stdout not valid JSON: %v\ngot: %q", err, stdout)
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
