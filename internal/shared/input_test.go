package shared

import (
	"io"
	"os"
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
