// Package testutil provides RunContractTests, the shared test harness imported
// by every vrksh tool's _test.go. It captures stdout, stderr, and exit codes so
// tools can be tested without spawning subprocesses.
package testutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/vrksh/vrksh/internal/shared"
)

// ContractCase describes a single test scenario for a vrksh tool.
type ContractCase struct {
	Name     string   // test name passed to t.Run
	Args     []string // os.Args[1:] to set before calling Run
	Stdin    string   // content written to stdin; empty means no input
	WantOut  string   // exact expected stdout
	WantErr  string   // substring expected in stderr (empty = not checked)
	WantExit int      // expected exit code: 0, 1, or 2
}

// exitSentinel is the panic value used to intercept os.Exit calls.
type exitSentinel struct{ code int }

// RunContractTests runs each case by replacing os.Stdin/Stdout/Stderr and
// shared.ExitFunc, calling run(), and asserting the captured output and exit
// code against the expected values.
//
// Tools must route all exits through shared.Die or shared.DieUsage — direct
// os.Exit calls will terminate the test process.
func RunContractTests(t *testing.T, run func(), cases []ContractCase) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.Name, func(t *testing.T) {
			gotOut, gotErr, gotExit := runCase(t, run, tc)

			if gotOut != tc.WantOut {
				t.Errorf("stdout:\n  got:  %q\n  want: %q", gotOut, tc.WantOut)
			}
			if tc.WantErr != "" && !strings.Contains(gotErr, tc.WantErr) {
				t.Errorf("stderr %q does not contain %q", gotErr, tc.WantErr)
			}
			if gotExit != tc.WantExit {
				t.Errorf("exit code: got %d, want %d (stderr: %q)", gotExit, tc.WantExit, gotErr)
			}
		})
	}
}

// runCase executes a single ContractCase and returns captured stdout, stderr,
// and exit code. It restores all global state before returning.
func runCase(t *testing.T, run func(), tc ContractCase) (stdout, stderr string, exitCode int) {
	t.Helper()

	// --- set up stdin ---
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdin pipe: %v", err)
	}
	if _, err := io.WriteString(stdinW, tc.Stdin); err != nil {
		t.Fatalf("writing stdin: %v", err)
	}
	if err := stdinW.Close(); err != nil {
		t.Fatalf("closing stdin writer: %v", err)
	}

	// --- set up stdout capture ---
	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stdout pipe: %v", err)
	}

	// --- set up stderr capture ---
	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("creating stderr pipe: %v", err)
	}

	// --- save originals ---
	origStdin := os.Stdin
	origStdout := os.Stdout
	origStderr := os.Stderr
	origArgs := os.Args
	origExit := shared.ExitFunc

	// --- replace globals ---
	os.Stdin = stdinR
	os.Stdout = stdoutW
	os.Stderr = stderrW
	os.Args = append([]string{"vrk"}, tc.Args...)

	exitCode = 0
	shared.ExitFunc = func(code int) {
		exitCode = code
		panic(exitSentinel{code})
	}

	// --- call run, recover exit panics ---
	func() {
		defer func() {
			if r := recover(); r != nil {
				if _, ok := r.(exitSentinel); !ok {
					panic(r) // re-panic for unexpected panics
				}
				// exitCode was already set above
			}
		}()
		run()
	}()

	// --- close write ends so readers reach EOF ---
	_ = stdoutW.Close()
	_ = stderrW.Close()
	_ = stdinR.Close()

	// --- restore globals ---
	os.Stdin = origStdin
	os.Stdout = origStdout
	os.Stderr = origStderr
	os.Args = origArgs
	shared.ExitFunc = origExit

	// --- read captured output ---
	var outBuf, errBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, stdoutR); err != nil {
		t.Fatalf("reading stdout: %v", err)
	}
	if _, err := io.Copy(&errBuf, stderrR); err != nil {
		t.Fatalf("reading stderr: %v", err)
	}
	_ = stdoutR.Close()
	_ = stderrR.Close()

	return outBuf.String(), errBuf.String(), exitCode
}

// Logf is a convenience wrapper so test helpers can print diagnostic output
// that is visible only on failure. Wraps t.Logf with a "contract: " prefix.
func Logf(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Logf("contract: "+format, args...)
}

// Failf formats a message and calls t.Fatalf. Convenience for contract tests.
func Failf(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Fatalf("contract: "+format, args...)
}

// MustParse calls fs.Parse(args) and calls t.Fatalf on error.
func MustParse(t *testing.T, fs interface{ Parse([]string) error }, args []string) {
	t.Helper()
	if err := fs.Parse(args); err != nil {
		t.Fatalf("flag parse error: %v", err)
	}
}

// WantExitCode is a typed assertion helper for exit code tests.
func WantExitCode(t *testing.T, got, want int, context string) {
	t.Helper()
	if got != want {
		t.Errorf("%s: exit code = %d, want %d", context, got, want)
	}
}

// WantStdout is a typed assertion helper for exact stdout match.
func WantStdout(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("stdout:\n  got:  %q\n  want: %q", got, want)
	}
}

// WantStderrContains asserts that stderr contains the given substring.
func WantStderrContains(t *testing.T, stderr, substr string) {
	t.Helper()
	if !strings.Contains(stderr, substr) {
		t.Errorf("stderr %q does not contain %q", stderr, substr)
	}
}

// PrintDiff is a helper for debugging test failures — prints a labelled diff
// of got vs want as a test log line.
func PrintDiff(t *testing.T, label, got, want string) {
	t.Helper()
	if got == want {
		return
	}
	t.Logf("%s diff:\n  got:  %q\n  want: %q", label, got, want)
}

// FormatMismatch returns a formatted string describing a mismatch, suitable for
// embedding in longer error messages.
func FormatMismatch(label, got, want string) string {
	return fmt.Sprintf("%s: got %q, want %q", label, got, want)
}
