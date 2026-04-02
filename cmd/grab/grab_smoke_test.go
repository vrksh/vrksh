//go:build smoke

package grab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const smokeHTML = `<!DOCTYPE html>
<html>
<head><title>Smoke Test Page</title></head>
<body>
<nav><a href="/">Home</a></nav>
<main>
<h1>Smoke Test Heading</h1>
<p>This is the main content paragraph used in smoke tests.</p>
<a href="https://example.com">Example link</a>
</main>
<footer>Footer</footer>
</body>
</html>`

// vrkBinary returns the path to the vrk binary. Checks VRK env var first,
// then falls back to ../../vrk relative to the package directory.
func vrkBinary(t *testing.T) string {
	t.Helper()
	if v := os.Getenv("VRK"); v != "" {
		if _, err := os.Stat(v); err == nil {
			return v
		}
		t.Fatalf("VRK=%q: file not found", v)
	}
	bin := filepath.Join("..", "..", "vrk")
	if _, err := os.Stat(bin); err == nil {
		return bin
	}
	t.Skip("vrk binary not found at ../../vrk; run 'make build' first or set VRK env var")
	return ""
}

// runBinary invokes the vrk binary with "grab" and the given args, returns
// stdout, stderr, and exit code.
func runBinary(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	vrk := vrkBinary(t)
	cmd := exec.Command(vrk, append([]string{"grab", "--allow-internal"}, args...)...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode = 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	return outBuf.String(), errBuf.String(), exitCode
}

func smokeHTMLServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, smokeHTML)
	}))
}

func TestSmokeDefaultMarkdown(t *testing.T) {
	srv := smokeHTMLServer()
	defer srv.Close()

	stdout, _, code := runBinary(t, srv.URL)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout == "" {
		t.Fatal("stdout is empty")
	}
	if !strings.Contains(stdout, "Smoke Test Heading") {
		t.Errorf("stdout = %q, want it to contain heading text", stdout)
	}
	if !strings.Contains(stdout, "#") {
		t.Errorf("stdout = %q, want markdown heading '#'", stdout)
	}
}

func TestSmokeRawMode(t *testing.T) {
	srv := smokeHTMLServer()
	defer srv.Close()

	stdout, _, code := runBinary(t, srv.URL, "--raw")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "<") {
		t.Errorf("--raw stdout = %q, want raw HTML with '<'", stdout)
	}
	if !strings.Contains(stdout, "Smoke Test Heading") {
		t.Errorf("--raw stdout = %q, want page content", stdout)
	}
}

func TestSmokeTextMode(t *testing.T) {
	srv := smokeHTMLServer()
	defer srv.Close()

	stdout, _, code := runBinary(t, srv.URL, "--text")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "# ") {
		t.Errorf("--text stdout %q must not contain markdown heading '# '", stdout)
	}
	if strings.Contains(stdout, "](") {
		t.Errorf("--text stdout %q must not contain markdown link syntax", stdout)
	}
	if !strings.Contains(stdout, "Smoke Test Heading") {
		t.Errorf("--text stdout = %q, want page content", stdout)
	}
}

func TestSmokeJSONMode(t *testing.T) {
	srv := smokeHTMLServer()
	defer srv.Close()

	stdout, _, code := runBinary(t, srv.URL, "--json")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("--json stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	for _, field := range []string{"url", "title", "content", "fetched_at", "status", "token_estimate"} {
		if _, ok := obj[field]; !ok {
			t.Errorf("--json output missing field %q", field)
		}
	}
	est, _ := obj["token_estimate"].(float64)
	if est <= 0 {
		t.Errorf("token_estimate = %v, want > 0", est)
	}
}

func TestSmokeInvalidURL(t *testing.T) {
	_, _, code := runBinary(t, "not-a-url")
	if code != 2 {
		t.Fatalf("invalid URL: exit code = %d, want 2", code)
	}
}

func TestSmokeNoURLInteractive(t *testing.T) {
	// Run with no args and stdin connected to /dev/null (not a TTY but empty).
	// Empty stdin → no URL → exit 2.
	vrk := vrkBinary(t)
	cmd := exec.Command(vrk, "grab")
	cmd.Stdin = strings.NewReader("")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}
	if exitCode != 2 {
		t.Fatalf("no URL: exit code = %d, want 2\nstdout: %q\nstderr: %q",
			exitCode, outBuf.String(), errBuf.String())
	}
}

func TestSmokeRedirectCap(t *testing.T) {
	hop := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hop++
		http.Redirect(w, r, fmt.Sprintf("/?n=%d", hop), http.StatusFound)
	}))
	defer srv.Close()

	stdout, _, code := runBinary(t, srv.URL)
	if code != 1 {
		t.Fatalf("redirect cap: exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on redirect error, got %q", stdout)
	}
}
