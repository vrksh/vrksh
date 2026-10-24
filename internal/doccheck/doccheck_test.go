package main

import (
	"os"
	"testing"
	"time"
)

func selfEnv() (*env, func()) {
	dir, _ := os.MkdirTemp("", "doccheck-self")
	return &env{dir: dir, timeout: 20 * time.Second, environ: os.Environ()}, func() { _ = os.RemoveAll(dir) }
}

// The harness must classify a known-good file as all-pass and a known-bad
// file as exactly 4 failures (wrong output, wrong exit code, missing line
// after "...", unexpected extra expected line).
func TestSelfGoodAndBad(t *testing.T) {
	good, err := checkFile("../../testdata/doccheck/selftest/good.md", selfEnv)
	if err != nil {
		t.Fatal(err)
	}
	if len(good.fails) != 0 || good.passed != 11 || good.skipped != 1 {
		for _, f := range good.fails {
			t.Log(report(f))
		}
		t.Fatalf("good.md: passed=%d skipped=%d failed=%d, want 11/1/0", good.passed, good.skipped, len(good.fails))
	}
	bad, err := checkFile("../../testdata/doccheck/selftest/bad.md", selfEnv)
	if err != nil {
		t.Fatal(err)
	}
	if len(bad.fails) != 4 || bad.passed != 2 {
		for _, f := range bad.fails {
			t.Log(report(f))
		}
		t.Fatalf("bad.md: passed=%d failed=%d, want 2/4", bad.passed, len(bad.fails))
	}
}

func TestMatches(t *testing.T) {
	cases := []struct {
		exp, act string
		want     bool
	}{
		{"", "", true},
		{"a", "a\n", true},
		{"a", "a\nb", false},
		{"...", "", true},
		{"a\n...\nb", "a\nb", true},
		{"a\n...", "a\nx\ny", true},
		{"x ... y", "x 1 2 y", true},
		{"x ... y", "x 1 2 z", false},
	}
	for _, c := range cases {
		if got := matches(c.exp, c.act); got != c.want {
			t.Errorf("matches(%q,%q)=%v want %v", c.exp, c.act, got, c.want)
		}
	}
}
