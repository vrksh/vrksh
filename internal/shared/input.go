package shared

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// ReadInput returns input from positional args (joined with a space) or from
// stdin. Strips exactly one trailing newline from stdin — not all whitespace.
// Returns an error if no input is provided or if stdin contains only whitespace.
func ReadInput(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	return readFromStdin()
}

// ReadInputOptional behaves like ReadInput but returns ("", nil) instead of an
// error when no input is present. Use for tools that have a meaningful default
// when no input is given.
func ReadInputOptional(args []string) (string, error) {
	if len(args) > 0 {
		return strings.Join(args, " "), nil
	}
	s, err := readFromStdin()
	if err != nil {
		return "", nil
	}
	return s, nil
}

// ReadInputLines reads stdin as a sequence of lines and returns them as a slice.
// If positional args are provided, each arg is treated as a line.
// The trailing empty element produced by a final newline is dropped.
func ReadInputLines(args []string) ([]string, error) {
	if len(args) > 0 {
		return args, nil
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

// ScanLines returns a bufio.Scanner over r. Use for JSONL record-processing
// tools that read input line-by-line. Never use io.ReadAll for record-processing
// tools — it will OOM on large inputs.
func ScanLines(r io.Reader) *bufio.Scanner {
	return bufio.NewScanner(r)
}

// readFromStdin reads all of stdin, strips exactly one trailing newline, and
// returns an error if the result is empty or whitespace-only.
func readFromStdin() (string, error) {
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
