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
func Die(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "error: %s\n", msg)
	ExitFunc(ExitError)
}

// DieUsage writes a usage error message to stderr and exits with code 2.
func DieUsage(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "usage error: %s\n", msg)
	ExitFunc(ExitUsage)
}

// Warn writes a warning to stderr and returns. Never exits.
func Warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "warning: %s\n", msg)
}
