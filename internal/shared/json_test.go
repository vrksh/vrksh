package shared

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureStdout replaces os.Stdout with a pipe and returns a function
// that closes the pipe, restores os.Stdout, and returns the captured output.
func captureStdout(t *testing.T) func() string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	return func() string {
		_ = w.Close()
		os.Stdout = orig
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		return buf.String()
	}
}

func TestPrintJSON(t *testing.T) {
	type payload struct{ Name string }

	t.Run("marshals struct to stdout with newline", func(t *testing.T) {
		flush := captureStdout(t)
		if err := PrintJSON(payload{Name: "hello"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := flush()
		want := `{"Name":"hello"}` + "\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("output ends with newline", func(t *testing.T) {
		flush := captureStdout(t)
		if err := PrintJSON(payload{Name: "x"}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(flush(), "\n") {
			t.Error("output has no trailing newline")
		}
	})

	t.Run("nil returns error not die", func(t *testing.T) {
		err := PrintJSON(nil)
		if err == nil {
			t.Fatal("expected non-nil error for nil input, got nil")
		}
		if !strings.Contains(err.Error(), "cannot marshal nil") {
			t.Errorf("error %q does not contain %q", err.Error(), "cannot marshal nil")
		}
	})

	t.Run("nil writes nothing to stdout", func(t *testing.T) {
		flush := captureStdout(t)
		_ = PrintJSON(nil)
		if got := flush(); got != "" {
			t.Errorf("wrote %q to stdout on nil, want nothing", got)
		}
	})
}

func TestPrintJSONL(t *testing.T) {
	type item struct{ A int }

	t.Run("one line per item", func(t *testing.T) {
		flush := captureStdout(t)
		if err := PrintJSONL([]any{item{A: 1}, item{A: 2}}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := flush()
		want := "{\"A\":1}\n{\"A\":2}\n"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty slice produces no output", func(t *testing.T) {
		flush := captureStdout(t)
		if err := PrintJSONL([]any{}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := flush(); got != "" {
			t.Errorf("got %q, want empty", got)
		}
	})
}
