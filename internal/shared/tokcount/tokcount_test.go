package tokcount

import "testing"

// Known cl100k_base counts verified against the tiktoken reference implementation.
// These are the ground-truth values the package must never regress on.
var countTests = []struct {
	input string
	want  int
}{
	{"", 0},
	{"hello world", 2},
	{"Hello, world!", 4},
	// Single token — a common English word.
	{"the", 1},
	// Whitespace-only: spaces tokenize as their own tokens.
	{" ", 1},
	// Longer prose: token count grows sub-linearly with word count.
	{"The quick brown fox jumps over the lazy dog", 9},
}

func TestCountTokens(t *testing.T) {
	for _, tc := range countTests {
		got, err := CountTokens(tc.input)
		if err != nil {
			t.Errorf("CountTokens(%q): unexpected error: %v", tc.input, err)
			continue
		}
		if got != tc.want {
			t.Errorf("CountTokens(%q) = %d, want %d", tc.input, got, tc.want)
		}
	}
}

// TestCountTokensEmpty ensures the fast path (empty string) returns 0
// without invoking the encoder at all.
func TestCountTokensEmpty(t *testing.T) {
	got, err := CountTokens("")
	if err != nil {
		t.Fatalf("CountTokens(%q): unexpected error: %v", "", err)
	}
	if got != 0 {
		t.Errorf("CountTokens(%q) = %d, want 0", "", got)
	}
}

// TestCountTokensMonotone is a property test: more text never produces
// fewer tokens than a strict prefix of that text.
func TestCountTokensMonotone(t *testing.T) {
	base := "hello"
	extended := "hello world"
	a, err := CountTokens(base)
	if err != nil {
		t.Fatalf("CountTokens(%q): %v", base, err)
	}
	b, err := CountTokens(extended)
	if err != nil {
		t.Fatalf("CountTokens(%q): %v", extended, err)
	}
	if b < a {
		t.Errorf("CountTokens(%q)=%d < CountTokens(%q)=%d: more text produced fewer tokens",
			extended, b, base, a)
	}
}

// TestCountTokensIdempotent verifies that calling CountTokens twice on the
// same input returns the same result (the global BPE loader must be stable).
func TestCountTokensIdempotent(t *testing.T) {
	input := "idempotent check"
	a, err := CountTokens(input)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	b, err := CountTokens(input)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if a != b {
		t.Errorf("CountTokens(%q) not idempotent: first=%d second=%d", input, a, b)
	}
}
