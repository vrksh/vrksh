// Package base tests for vrk base — encoding converter.
//
// No --json flag on base — output is raw encoded/decoded bytes; no JSON envelope.
// TestJSONErrorToStdout and TestInteractiveTTYWithJSONFlag are therefore not
// applicable. Add them if a --json flag is ever introduced.
package base

import (
	"bytes"
	"io"
	"os"
	"testing"
)

// runBase replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and
// returns captured stdout bytes, stderr bytes, and the exit code.
// stdin is written as raw bytes so binary input works correctly.
// Not parallel-safe — tests share global OS state.
func runBase(t *testing.T, args []string, stdin []byte) (stdout, stderr []byte, code int) {
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
	if len(stdin) > 0 {
		if _, err := stdinW.Write(stdin); err != nil {
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

	os.Args = append([]string{"base"}, args...)
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

	return outBuf.Bytes(), errBuf.Bytes(), code
}

// --- happy path: encode ---

func TestEncodeBase64(t *testing.T) {
	// echo 'hello' appends \n; encode strips it and encodes "hello".
	stdout, _, code := runBase(t, []string{"encode", "--to", "base64"}, []byte("hello\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if string(stdout) != "aGVsbG8=\n" {
		t.Errorf("stdout = %q, want %q", stdout, "aGVsbG8=\n")
	}
}

func TestEncodeBase64URL(t *testing.T) {
	stdout, _, code := runBase(t, []string{"encode", "--to", "base64url"}, []byte("hello\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// base64url: no padding, URL-safe alphabet (no + or /)
	if string(stdout) != "aGVsbG8\n" {
		t.Errorf("stdout = %q, want %q", stdout, "aGVsbG8\n")
	}
}

func TestEncodeHex(t *testing.T) {
	stdout, _, code := runBase(t, []string{"encode", "--to", "hex"}, []byte("hello\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if string(stdout) != "68656c6c6f\n" {
		t.Errorf("stdout = %q, want %q", stdout, "68656c6c6f\n")
	}
}

func TestEncodeBase32(t *testing.T) {
	stdout, _, code := runBase(t, []string{"encode", "--to", "base32"}, []byte("hello\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// base32("hello") = 5 bytes = 40 bits = 8 groups (multiple of 8) → no padding.
	// Note: the session spec showed "NBSWY3DPEB3W64TMMQ======" which is base32 of
	// "hello world" — a typo. NBSWY3DP is the correct RFC 4648 encoding of "hello".
	if string(stdout) != "NBSWY3DP\n" {
		t.Errorf("stdout = %q, want %q", stdout, "NBSWY3DP\n")
	}
}

// --- happy path: decode ---

func TestDecodeBase64(t *testing.T) {
	// Decode output is raw bytes with no added newline.
	stdout, _, code := runBase(t, []string{"decode", "--from", "base64"}, []byte("aGVsbG8=\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !bytes.Equal(stdout, []byte("hello")) {
		t.Errorf("stdout = %q, want %q", stdout, "hello")
	}
}

func TestDecodeBase64URL(t *testing.T) {
	stdout, _, code := runBase(t, []string{"decode", "--from", "base64url"}, []byte("aGVsbG8\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !bytes.Equal(stdout, []byte("hello")) {
		t.Errorf("stdout = %q, want %q", stdout, "hello")
	}
}

func TestDecodeHex(t *testing.T) {
	stdout, _, code := runBase(t, []string{"decode", "--from", "hex"}, []byte("68656c6c6f\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !bytes.Equal(stdout, []byte("hello")) {
		t.Errorf("stdout = %q, want %q", stdout, "hello")
	}
}

func TestDecodeBase32(t *testing.T) {
	stdout, _, code := runBase(t, []string{"decode", "--from", "base32"}, []byte("NBSWY3DP\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !bytes.Equal(stdout, []byte("hello")) {
		t.Errorf("stdout = %q, want %q", stdout, "hello")
	}
}

// --- exit 1: invalid decode input ---

func TestDecodeInvalidBase64(t *testing.T) {
	stdout, stderr, code := runBase(t, []string{"decode", "--from", "base64"}, []byte("not valid base64!!!\n"))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout = %q, want empty on error", stdout)
	}
	if !bytes.Contains(stderr, []byte("invalid base64 input")) {
		t.Errorf("stderr = %q, want 'invalid base64 input'", stderr)
	}
}

func TestDecodeInvalidHex(t *testing.T) {
	_, stderr, code := runBase(t, []string{"decode", "--from", "hex"}, []byte("gg\n"))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !bytes.Contains(stderr, []byte("invalid hex input")) {
		t.Errorf("stderr = %q, want 'invalid hex input'", stderr)
	}
}

func TestDecodeInvalidBase32(t *testing.T) {
	// Base32 alphabet is A-Z and 2-7; digits 0, 1, 8, 9 are not valid.
	_, stderr, code := runBase(t, []string{"decode", "--from", "base32"}, []byte("00000000\n"))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !bytes.Contains(stderr, []byte("invalid base32 input")) {
		t.Errorf("stderr = %q, want 'invalid base32 input'", stderr)
	}
}

func TestDecodeInvalidBase64URL(t *testing.T) {
	_, stderr, code := runBase(t, []string{"decode", "--from", "base64url"}, []byte("not valid!!!\n"))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !bytes.Contains(stderr, []byte("invalid base64url input")) {
		t.Errorf("stderr = %q, want 'invalid base64url input'", stderr)
	}
}

// --- exit 2: usage errors ---

func TestNoSubcommand(t *testing.T) {
	stdout, stderr, code := runBase(t, []string{}, nil)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout = %q, want empty on usage error", stdout)
	}
	if !bytes.Contains(stderr, []byte("usage")) {
		t.Errorf("stderr = %q, want 'usage'", stderr)
	}
}

func TestMissingToFlag(t *testing.T) {
	_, stderr, code := runBase(t, []string{"encode"}, []byte("hello\n"))
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !bytes.Contains(stderr, []byte("--to is required")) {
		t.Errorf("stderr = %q, want '--to is required'", stderr)
	}
}

func TestMissingFromFlag(t *testing.T) {
	_, stderr, code := runBase(t, []string{"decode"}, []byte("aGVsbG8=\n"))
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !bytes.Contains(stderr, []byte("--from is required")) {
		t.Errorf("stderr = %q, want '--from is required'", stderr)
	}
}

func TestUnknownEncoding(t *testing.T) {
	_, stderr, code := runBase(t, []string{"encode", "--to", "bogus"}, []byte("hello\n"))
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !bytes.Contains(stderr, []byte("unsupported encoding")) {
		t.Errorf("stderr = %q, want 'unsupported encoding'", stderr)
	}
}

func TestUnknownFlag(t *testing.T) {
	_, _, code := runBase(t, []string{"encode", "--bogus"}, nil)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
}

func TestUnknownSubcommand(t *testing.T) {
	_, stderr, code := runBase(t, []string{"transform"}, nil)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !bytes.Contains(stderr, []byte("unknown subcommand")) {
		t.Errorf("stderr = %q, want 'unknown subcommand'", stderr)
	}
}

// --- --help: exit 0 ---

func TestHelpExitZero(t *testing.T) {
	stdout, _, code := runBase(t, []string{"--help"}, nil)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !bytes.Contains(stdout, []byte("base")) {
		t.Errorf("--help stdout = %q, want it to contain 'base'", stdout)
	}
}

func TestHelpEncodeSubcommand(t *testing.T) {
	// --help on a subcommand exits 0 and prints usage.
	stdout, _, code := runBase(t, []string{"encode", "--help"}, nil)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !bytes.Contains(stdout, []byte("base")) {
		t.Errorf("encode --help stdout = %q, want it to contain 'base'", stdout)
	}
}

// --- interactive TTY ---

func TestTTYNoInput(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	// Provide --to so flag parsing succeeds; the TTY guard fires during readInput.
	stdout, stderr, code := runBase(t, []string{"encode", "--to", "base64"}, nil)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout = %q, want empty on usage error", stdout)
	}
	if !bytes.Contains(stderr, []byte("no input")) {
		t.Errorf("stderr = %q, want 'no input'", stderr)
	}
}

// --- empty stdin ---

func TestEmptyStdinEncode(t *testing.T) {
	stdout, _, code := runBase(t, []string{"encode", "--to", "base64"}, nil)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout = %q, want empty for empty input", stdout)
	}
}

func TestEmptyStdinDecode(t *testing.T) {
	stdout, _, code := runBase(t, []string{"decode", "--from", "base64"}, nil)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout = %q, want empty for empty input", stdout)
	}
}

// TestEmptyAfterStrip verifies that a sole newline byte (from e.g. `echo ""`)
// produces no output — the \n is stripped and the result is empty.
func TestEmptyAfterStrip(t *testing.T) {
	stdout, _, code := runBase(t, []string{"encode", "--to", "hex"}, []byte("\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if len(stdout) != 0 {
		t.Errorf("stdout = %q, want empty (sole newline stripped)", stdout)
	}
}

// --- binary safety ---

func TestBinarySafeHex(t *testing.T) {
	input := []byte{0x00, 0x01, 0x02, 0xff}

	stdout, _, code := runBase(t, []string{"encode", "--to", "hex"}, input)
	if code != 0 {
		t.Fatalf("encode exit code = %d, want 0", code)
	}
	if string(stdout) != "000102ff\n" {
		t.Errorf("hex encode: got %q, want %q", stdout, "000102ff\n")
	}

	// Decode the encoded output (strip is a no-op; stdout ends with \n which is stripped).
	decoded, _, code2 := runBase(t, []string{"decode", "--from", "hex"}, stdout)
	if code2 != 0 {
		t.Fatalf("decode exit code = %d, want 0", code2)
	}
	if !bytes.Equal(decoded, input) {
		t.Errorf("binary round-trip: got %v, want %v", decoded, input)
	}
}

func TestBinarySafeBase64(t *testing.T) {
	input := []byte{0x00, 0x01, 0x02, 0xff}

	stdout, _, code := runBase(t, []string{"encode", "--to", "base64"}, input)
	if code != 0 {
		t.Fatalf("encode exit code = %d, want 0", code)
	}

	decoded, _, code2 := runBase(t, []string{"decode", "--from", "base64"}, stdout)
	if code2 != 0 {
		t.Fatalf("decode exit code = %d, want 0", code2)
	}
	if !bytes.Equal(decoded, input) {
		t.Errorf("binary round-trip: got %v, want %v", decoded, input)
	}
}

// --- round-trip property tests ---
// Inputs must NOT end with \n so the strip does not consume content.

func TestRoundTripBase64(t *testing.T) {
	testRoundTrip(t, "base64", []byte("round-trip property test"))
}

func TestRoundTripBase64URL(t *testing.T) {
	testRoundTrip(t, "base64url", []byte("round-trip property test"))
}

func TestRoundTripHex(t *testing.T) {
	testRoundTrip(t, "hex", []byte("round-trip property test"))
}

func TestRoundTripBase32(t *testing.T) {
	testRoundTrip(t, "base32", []byte("round-trip property test"))
}

func testRoundTrip(t *testing.T, enc string, input []byte) {
	t.Helper()

	encoded, _, c1 := runBase(t, []string{"encode", "--to", enc}, input)
	if c1 != 0 {
		t.Fatalf("%s encode exit code = %d, want 0", enc, c1)
	}

	// encoded includes a trailing \n (from fmt.Fprintln); decode strips it.
	decoded, _, c2 := runBase(t, []string{"decode", "--from", enc}, encoded)
	if c2 != 0 {
		t.Fatalf("%s decode exit code = %d, want 0", enc, c2)
	}

	if !bytes.Equal(decoded, input) {
		t.Errorf("%s round-trip: got %q, want %q", enc, decoded, input)
	}
}

// --- --quiet flag ---

func TestQuietSuppressesStderr(t *testing.T) {
	// Invalid hex input causes exit 1; --quiet must silence stderr.
	_, stderr, code := runBase(t, []string{"decode", "--from", "hex", "--quiet"}, []byte("bad\n"))
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if len(stderr) != 0 {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

func TestQuietDoesNotSuppressStdout(t *testing.T) {
	stdout, _, code := runBase(t, []string{"encode", "--to", "hex", "--quiet"}, []byte("hello\n"))
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if string(stdout) != "68656c6c6f\n" {
		t.Errorf("stdout = %q, want %q", stdout, "68656c6c6f\n")
	}
}

// --- positional argument ---

func TestPositionalArg(t *testing.T) {
	// Positional arg bypasses stdin — TTY guard must not fire.
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	defer func() { isTerminal = orig }()

	stdout, _, code := runBase(t, []string{"encode", "--to", "hex", "hello"}, nil)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if string(stdout) != "68656c6c6f\n" {
		t.Errorf("stdout = %q, want %q", stdout, "68656c6c6f\n")
	}
}
