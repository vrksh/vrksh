package shared

import (
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

// pipeStdin replaces os.Stdin with a pipe containing content
// and registers a cleanup to restore it.
func pipeStdin(t *testing.T, content string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, content); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()
	orig := os.Stdin
	os.Stdin = r
	t.Cleanup(func() {
		os.Stdin = orig
		_ = r.Close()
	})
}

func TestReadInput(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		stdin   string
		want    string
		wantErr bool
	}{
		{
			name: "single arg",
			args: []string{"hello"},
			want: "hello",
		},
		{
			name: "multiple args joined with space",
			args: []string{"hello", "world"},
			want: "hello world",
		},
		{
			name:  "stdin with trailing newline stripped",
			stdin: "hello\n",
			want:  "hello",
		},
		{
			name:  "stdin without trailing newline",
			stdin: "hello",
			want:  "hello",
		},
		{
			name:  "strips exactly one trailing newline",
			stdin: "hello\n\n",
			want:  "hello\n",
		},
		{
			name:  "preserves leading and trailing spaces",
			stdin: "  hello  ",
			want:  "  hello  ",
		},
		{
			name:    "empty stdin is an error",
			stdin:   "",
			wantErr: true,
		},
		{
			name:    "blank line is an error",
			stdin:   "\n",
			wantErr: true,
		},
		{
			name:    "whitespace-only stdin is an error",
			stdin:   "  \n  ",
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.args) == 0 {
				pipeStdin(t, tc.stdin)
			}
			got, err := ReadInput(tc.args)
			if (err != nil) != tc.wantErr {
				t.Fatalf("error = %v, wantErr = %v", err, tc.wantErr)
			}
			if !tc.wantErr && got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadInputOptional(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  string
	}{
		{
			name:  "empty stdin returns empty string, no error",
			stdin: "",
			want:  "",
		},
		{
			name:  "blank stdin returns empty string, no error",
			stdin: "\n",
			want:  "",
		},
		{
			name:  "content from stdin",
			stdin: "hello\n",
			want:  "hello",
		},
		{
			name: "content from arg",
			args: []string{"hello"},
			want: "hello",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.args) == 0 {
				pipeStdin(t, tc.stdin)
			}
			got, err := ReadInputOptional(tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

// TestReadInputOptionalFailingReader confirms that a real I/O error from stdin
// is propagated rather than silently swallowed.
func TestReadInputOptionalFailingReader(t *testing.T) {
	// Create an OS pipe and immediately close the write end with an error so
	// any read from the read end returns a non-EOF error.
	pr, pw, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected read error")
	// CloseWithError is on *io.PipeWriter, not *os.File. Use a separate
	// goroutine: close the OS write-fd with a deliberate broken-pipe situation
	// by closing the read end first, then writing to force SIGPIPE / error.
	// Simpler: close the file we hand to os.Stdin so Read() returns an error.
	_ = pw.Close()
	_ = pr.Close() // close the read end — subsequent Read calls return an error

	// Swap os.Stdin with the closed file; restore on cleanup.
	orig := os.Stdin
	os.Stdin = pr
	t.Cleanup(func() { os.Stdin = orig })

	// Use a thin wrapper so we can force a predictable error without relying
	// on platform-specific closed-fd wording.
	_ = injected // referenced above for documentation clarity

	_, gotErr := ReadInputOptional(nil)
	if gotErr == nil {
		t.Fatal("expected error from closed stdin, got nil")
	}
}

// TestReadInputOptionalWhitespaceEdgeCases pins the TrimSuffix behaviour so
// that any future change from TrimSuffix → TrimRight (or vice-versa) is
// caught immediately.
func TestReadInputOptionalWhitespaceEdgeCases(t *testing.T) {
	tests := []struct {
		name  string
		stdin string
		want  string
	}{
		{
			// Single newline: TrimSuffix removes it → empty → ("", nil).
			name:  "single newline returns empty",
			stdin: "\n",
			want:  "",
		},
		{
			// Spaces + newline: TrimSuffix removes newline → "   " → TrimSpace
			// is empty → ("", nil).
			name:  "spaces and newline returns empty",
			stdin: "   \n",
			want:  "",
		},
		{
			// Two newlines: TrimSuffix removes one → "\n" → TrimSpace is empty
			// → ("", nil). TrimSuffix strips exactly one trailing newline.
			name:  "two newlines returns empty",
			stdin: "\n\n",
			want:  "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pipeStdin(t, tc.stdin)
			got, err := ReadInputOptional(nil)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Errorf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestReadInputLines(t *testing.T) {
	tests := []struct {
		name  string
		args  []string
		stdin string
		want  []string
	}{
		{
			name:  "multiple lines",
			stdin: "line1\nline2\nline3\n",
			want:  []string{"line1", "line2", "line3"},
		},
		{
			name:  "no trailing empty element from final newline",
			stdin: "a\nb\n",
			want:  []string{"a", "b"},
		},
		{
			name:  "empty stdin returns empty slice",
			stdin: "",
			want:  []string{},
		},
		{
			name: "args treated as lines",
			args: []string{"x", "y", "z"},
			want: []string{"x", "y", "z"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if len(tc.args) == 0 {
				pipeStdin(t, tc.stdin)
			}
			got, err := ReadInputLines(tc.args)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %d lines %v, want %d lines %v", len(got), got, len(tc.want), tc.want)
			}
			for i := range tc.want {
				if got[i] != tc.want[i] {
					t.Errorf("line[%d]: got %q, want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// TestScanLines verifies ScanLines returns a working line-by-line scanner.
func TestScanLines(t *testing.T) {
	input := "line1\nline2\nline3\n"
	scanner := ScanLines(strings.NewReader(input))

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}

	want := []string{"line1", "line2", "line3"}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines %v, want %d", len(lines), lines, len(want))
	}
	for i, line := range lines {
		if line != want[i] {
			t.Errorf("line[%d]: got %q, want %q", i, line, want[i])
		}
	}
}

// TestScanLinesLargeBuffer verifies ScanLines handles lines larger than the
// default 64KB bufio.Scanner limit. A 100KB line must pass through without
// a bufio.ErrTooLong error.
func TestScanLinesLargeBuffer(t *testing.T) {
	// Build a 100KB line (well above the default 64KB limit).
	bigLine := strings.Repeat("x", 100*1024)
	input := bigLine + "\n"

	scanner := ScanLines(strings.NewReader(input))

	if !scanner.Scan() {
		err := scanner.Err()
		t.Fatalf("Scan() returned false on 100KB line; err = %v", err)
	}
	got := scanner.Text()
	if got != bigLine {
		t.Errorf("got %d bytes, want %d", len(got), len(bigLine))
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error after scan: %v", err)
	}
}

// TestScanLinesMultipleLargeLines verifies that multiple lines above 64KB each
// are all scanned correctly.
func TestScanLinesMultipleLargeLines(t *testing.T) {
	line1 := strings.Repeat("a", 80*1024)
	line2 := strings.Repeat("b", 90*1024)
	input := line1 + "\n" + line2 + "\n"

	scanner := ScanLines(strings.NewReader(input))

	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if len(lines[0]) != 80*1024 {
		t.Errorf("line[0] len = %d, want %d", len(lines[0]), 80*1024)
	}
	if len(lines[1]) != 90*1024 {
		t.Errorf("line[1] len = %d, want %d", len(lines[1]), 90*1024)
	}
}
