package shared

import (
	"fmt"
	"os"
)

const (
	ExitOK    = 0
	ExitError = 1
	ExitUsage = 2
)

// ExitFunc is called by Die and DieUsage. Tests replace it to capture exit codes
// without terminating the process.
var ExitFunc = os.Exit

// Die writes an error message to stderr and exits with code 1 (runtime error).
// Only call from main() or shared utilities that cannot return an int.
// Inside Run() int, use Errorf instead — it prints the same message and returns 1.
func Die(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	ExitFunc(ExitError)
}

// DieUsage writes a usage error message to stderr and exits with code 2.
// Only call from main() or shared utilities that cannot return an int.
// Inside Run() int, use UsageErrorf instead — it prints the same message and returns 2.
func DieUsage(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "usage error: %s\n", msg)
	ExitFunc(ExitUsage)
}

// Errorf writes an error message to stderr and returns ExitError (1).
// Use this inside Run() int — it prints "error: <msg>" then returns 1 so
// the caller can do: return shared.Errorf("jwt: invalid token: %v", err)
func Errorf(format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	return ExitError
}

// UsageErrorf writes a usage error message to stderr and returns ExitUsage (2).
// Use this inside Run() int — it prints "usage error: <msg>" then returns 2 so
// the caller can do: return shared.UsageErrorf("missing required flag: --key")
func UsageErrorf(format string, args ...any) int {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "usage error: %s\n", msg)
	return ExitUsage
}

// Warn writes a warning to stderr and returns. Never exits.
func Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
}
