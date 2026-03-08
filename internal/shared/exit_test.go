package shared

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// captureExit replaces os.Stderr and ExitFunc, calls f, and returns
// the captured stderr text and exit code. If f returns without calling
// ExitFunc, exitCode is -1.
func captureExit(t *testing.T, f func()) (stderr string, exitCode int) {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	origStderr := os.Stderr
	os.Stderr = w

	origExit := ExitFunc
	exitCode = -1
	ExitFunc = func(code int) {
		exitCode = code
		panic("exit:" + string(rune('0'+code)))
	}

	defer func() {
		ExitFunc = origExit
		os.Stderr = origStderr

		if r2 := recover(); r2 != nil {
			s, ok := r2.(string)
			if !ok || !strings.HasPrefix(s, "exit:") {
				panic(r2)
			}
		}

		_ = w.Close()
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		_ = r.Close()
		stderr = buf.String()
	}()

	f()
	return
}

func TestDie(t *testing.T) {
	tests := []struct {
		name       string
		fn         func()
		wantStderr string
		wantCode   int
	}{
		{
			name:       "exits with code 1",
			fn:         func() { Die("something broke") },
			wantStderr: "error: something broke\n",
			wantCode:   ExitError,
		},
		{
			name:       "formats arguments",
			fn:         func() { Die("file %q not found", "foo.txt") },
			wantStderr: "error: file \"foo.txt\" not found\n",
			wantCode:   ExitError,
		},
		{
			name: "writes nothing to stdout",
			fn: func() {
				rOut, wOut, err := os.Pipe()
				if err != nil {
					panic(err)
				}
				origOut := os.Stdout
				os.Stdout = wOut
				Die("oops")
				_ = wOut.Close()
				os.Stdout = origOut
				var buf bytes.Buffer
				_, _ = io.Copy(&buf, rOut)
				_ = rOut.Close()
				if buf.Len() != 0 {
					t.Errorf("Die wrote %q to stdout, want nothing", buf.String())
				}
			},
			wantCode: ExitError,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stderr, code := captureExit(t, tc.fn)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d", code, tc.wantCode)
			}
			if tc.wantStderr != "" && stderr != tc.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, tc.wantStderr)
			}
		})
	}
}

func TestDieUsage(t *testing.T) {
	tests := []struct {
		name       string
		fn         func()
		wantStderr string
		wantCode   int
	}{
		{
			name:       "exits with code 2",
			fn:         func() { DieUsage("missing flag --foo") },
			wantStderr: "usage error: missing flag --foo\n",
			wantCode:   ExitUsage,
		},
		{
			name:       "formats arguments",
			fn:         func() { DieUsage("flag %q is required", "--out") },
			wantStderr: "usage error: flag \"--out\" is required\n",
			wantCode:   ExitUsage,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stderr, code := captureExit(t, tc.fn)
			if code != tc.wantCode {
				t.Errorf("exit code = %d, want %d", code, tc.wantCode)
			}
			if stderr != tc.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, tc.wantStderr)
			}
		})
	}
}

func TestWarn(t *testing.T) {
	tests := []struct {
		name       string
		fn         func()
		wantStderr string
	}{
		{
			name:       "writes warning prefix",
			fn:         func() { Warn("disk almost full") },
			wantStderr: "warning: disk almost full\n",
		},
		{
			name:       "formats arguments",
			fn:         func() { Warn("retrying in %ds", 5) },
			wantStderr: "warning: retrying in 5s\n",
		},
		{
			name: "returns to caller",
			fn: func() {
				returned := false
				Warn("just a warning")
				returned = true
				if !returned {
					t.Error("Warn did not return to caller")
				}
			},
			wantStderr: "warning: just a warning\n",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stderr, code := captureExit(t, tc.fn)
			if code != -1 {
				t.Errorf("Warn called ExitFunc with code %d, want no exit", code)
			}
			if stderr != tc.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, tc.wantStderr)
			}
		})
	}
}

func TestErrorf(t *testing.T) {
	t.Run("returns ExitError (1)", func(t *testing.T) {
		r, w, _ := os.Pipe()
		orig := os.Stderr
		os.Stderr = w
		got := Errorf("something broke")
		_ = w.Close()
		os.Stderr = orig
		_ = r.Close()
		if got != ExitError {
			t.Errorf("Errorf() = %d, want %d", got, ExitError)
		}
	})

	t.Run("writes error prefix to stderr", func(t *testing.T) {
		stderr, _ := captureExit(t, func() { _ = Errorf("disk full") })
		if stderr != "error: disk full\n" {
			t.Errorf("stderr = %q, want %q", stderr, "error: disk full\n")
		}
	})

	t.Run("formats arguments", func(t *testing.T) {
		stderr, _ := captureExit(t, func() { _ = Errorf("file %q: %v", "x.txt", "not found") })
		want := "error: file \"x.txt\": not found\n"
		if stderr != want {
			t.Errorf("stderr = %q, want %q", stderr, want)
		}
	})

	t.Run("does not call ExitFunc", func(t *testing.T) {
		_, code := captureExit(t, func() { _ = Errorf("oops") })
		if code != -1 {
			t.Errorf("Errorf called ExitFunc with code %d, want no exit", code)
		}
	})
}

func TestUsageErrorf(t *testing.T) {
	t.Run("returns ExitUsage (2)", func(t *testing.T) {
		r, w, _ := os.Pipe()
		orig := os.Stderr
		os.Stderr = w
		got := UsageErrorf("missing flag --key")
		_ = w.Close()
		os.Stderr = orig
		_ = r.Close()
		if got != ExitUsage {
			t.Errorf("UsageErrorf() = %d, want %d", got, ExitUsage)
		}
	})

	t.Run("writes usage error prefix to stderr", func(t *testing.T) {
		stderr, _ := captureExit(t, func() { _ = UsageErrorf("missing flag") })
		if stderr != "usage error: missing flag\n" {
			t.Errorf("stderr = %q, want %q", stderr, "usage error: missing flag\n")
		}
	})

	t.Run("does not call ExitFunc", func(t *testing.T) {
		_, code := captureExit(t, func() { _ = UsageErrorf("bad args") })
		if code != -1 {
			t.Errorf("UsageErrorf called ExitFunc with code %d, want no exit", code)
		}
	})
}

func TestExitConstants(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"ExitOK", ExitOK, 0},
		{"ExitError", ExitError, 1},
		{"ExitUsage", ExitUsage, 2},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("%s = %d, want %d", tc.name, tc.got, tc.want)
			}
		})
	}
}
