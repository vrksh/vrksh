package shared

import (
	"golang.org/x/term"
)

// IsTerminal returns true if fd is a terminal.
//
// It is a variable so that tools can override it in their own unit tests
// (e.g. `var isTerminal = shared.IsTerminal` in the tool's package) to
// simulate interactive/non-interactive environments without a real TTY.
var IsTerminal = func(fd int) bool {
	return term.IsTerminal(fd)
}
