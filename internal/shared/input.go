package shared

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadInput returns input from positional args (joined with a space) or from
// stdin. Strips exactly one trailing newline from stdin — not all whitespace.
// Returns an error if no input is provided or if stdin contains only whitespace.
//
// When multiple positional args are given they are joined with a single space
// (e.g. ["hello", "world"] → "hello world"). Callers that must reject more
// than one arg should validate len(args) themselves before calling ReadInput.
func ReadInput(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	return readFromStdin()
}

// ReadInputOptional behaves like ReadInput but returns ("", nil) when stdin is
// empty, whitespace-only, or an interactive terminal — use for tools that have
// a meaningful default when no input is given. Real I/O errors are still propagated.
func ReadInputOptional(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	if IsTerminal(int(os.Stdin.Fd())) {
		return "", nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	if len(b) == 0 {
		return "", nil
	}
	s := strings.TrimSuffix(string(b), "\n")
	if strings.TrimSpace(s) == "" {
		return "", nil
	}
	return s, nil
}

// ReadInputLines reads stdin as a sequence of lines and returns them as a slice.
// If positional args are provided, each arg is treated as a line.
// The trailing empty element produced by a final newline is dropped.
// Returns an empty slice when stdin is an interactive terminal with no args.
func ReadInputLines(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
	}
	if IsTerminal(int(os.Stdin.Fd())) {
		return []string{}, nil
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}
	if len(b) == 0 {
		return []string{}, nil
	}
	lines := strings.Split(string(b), "\n")
	// Drop the trailing empty element produced by a final newline.
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines, nil
}

// MaxLineBytes is the longest line ScanLines accepts (64 MiB).
const MaxLineBytes = 64 << 20

// maxLineBytes is a var so tests can shrink the limit.
var maxLineBytes = MaxLineBytes

var errLineTooLong = errors.New("line exceeds 64 MiB limit")

// ScanLines returns a bufio.Scanner over r that accepts lines up to
// MaxLineBytes. Use for record-processing tools that read line-by-line. Never
// use io.ReadAll for those - it will OOM on large inputs.
//
// The initial buffer is 64 KiB and only grows when a line needs it. A longer
// line makes Scan return false with an error reading "line exceeds 64 MiB limit".
func ScanLines(r io.Reader) *bufio.Scanner {
	s := bufio.NewScanner(r)
	// +2 leaves room for a trailing \r\n on a line of exactly the limit.
	s.Buffer(make([]byte, 0, 64*1024), maxLineBytes+2)
	s.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		adv, tok, err := bufio.ScanLines(data, atEOF)
		if tok == nil && err == nil && len(data) > maxLineBytes+1 {
			return 0, nil, errLineTooLong
		}
		if len(tok) > maxLineBytes {
			return 0, nil, errLineTooLong
		}
		return adv, tok, err
	})
	return s
}

// readFromStdin reads all of stdin, strips exactly one trailing newline, and
// returns an error if the result is empty, whitespace-only, or stdin is an
// interactive terminal (which would otherwise block waiting for keyboard input).
func readFromStdin() (string, error) {
	if IsTerminal(int(os.Stdin.Fd())) {
		return "", fmt.Errorf("no input: provide as argument or via stdin")
	}
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		return "", fmt.Errorf("reading stdin: %w", err)
	}
	if len(b) == 0 {
		return "", fmt.Errorf("no input: provide as argument or via stdin")
	}
	// Strip exactly one trailing newline — not all whitespace.
	s := strings.TrimSuffix(string(b), "\n")
	if strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("no input: provide as argument or via stdin")
	}
	return s, nil
}
