package shared

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadAllLimited(t *testing.T) {
	b, err := ReadAllLimited(strings.NewReader("abcd"), 4)
	if err != nil || string(b) != "abcd" {
		t.Fatalf("at limit: got %q, %v", b, err)
	}
	if _, err := ReadAllLimited(strings.NewReader("abcde"), 4); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("over limit: want exceeds error, got %v", err)
	}
}

func TestReadFileLimited(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "f")
	if err := os.WriteFile(p, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if b, err := ReadFileLimited(p, 5, false); err != nil || string(b) != "hello" {
		t.Fatalf("regular: got %q, %v", b, err)
	}
	if _, err := ReadFileLimited(p, 4, false); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("over limit: got %v", err)
	}
	if _, err := ReadFileLimited(dir, 5, true); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("dir: got %v", err)
	}
	if _, err := ReadFileLimited("/dev/zero", 5, true); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("/dev/zero: got %v", err)
	}
}

func TestStripControls(t *testing.T) {
	in := "a\tb\nc\x1b]52;c;eA==\x07d\x7f\u0085e" + "\u202e" + "f" + "\u2066" + "g\fh\ri"
	if got, want := StripControls(in), "a\tb\nc]52;c;eA==defghi"; got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	// property: no forbidden rune survives, whatever the input
	for r := rune(0); r < 0x3000; r++ {
		s := StripControls(string(r))
		if s != "" && s != "\t" && s != "\n" && isControl(r) {
			t.Fatalf("rune %U survived", r)
		}
	}
}

func TestDecodeOne(t *testing.T) {
	var v any
	if err := DecodeOne(`{"a":1} `, &v); err != nil {
		t.Fatalf("single value: %v", err)
	}
	for _, s := range []string{`{"a":1}{"b":2}`, `{"a":1}garbage`, `1 2`} {
		if err := DecodeOne(s, &v); err == nil {
			t.Fatalf("%q: want trailing-data error", s)
		}
	}
}
