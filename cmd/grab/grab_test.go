package grab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

const testHTML = `<!DOCTYPE html>
<html>
<head><title>Test Page</title></head>
<body>
<nav><a href="/">Home</a></nav>
<main>
<h1>Hello World</h1>
<p>This is a test paragraph with enough content to be extracted cleanly.</p>
<a href="https://example.com">A link</a>
<blockquote>A quoted passage.</blockquote>
</main>
<footer>Footer content</footer>
</body>
</html>`

// runGrab replaces os.Stdin/Stdout/Stderr and os.Args, calls Run(), and returns
// captured stdout, stderr, and the exit code. Not parallel-safe — tests share
// global OS state.
func runGrab(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
	t.Helper()

	origStdin := os.Stdin
	origStdout := os.Stdout
	origStderr := os.Stderr
	origArgs := os.Args

	t.Cleanup(func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
		os.Stderr = origStderr
		os.Args = origArgs
	})

	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if _, err := io.WriteString(stdinW, stdinContent); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := stdinW.Close(); err != nil {
		t.Fatalf("close stdin write end: %v", err)
	}
	os.Stdin = stdinR

	stdoutR, stdoutW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdout pipe: %v", err)
	}
	os.Stdout = stdoutW

	stderrR, stderrW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stderr pipe: %v", err)
	}
	os.Stderr = stderrW

	os.Args = append([]string{"grab"}, args...)

	code = Run()

	_ = stdoutW.Close()
	_ = stderrW.Close()

	var outBuf, errBuf bytes.Buffer
	if _, err := io.Copy(&outBuf, stdoutR); err != nil {
		t.Fatalf("read stdout: %v", err)
	}
	if _, err := io.Copy(&errBuf, stderrR); err != nil {
		t.Fatalf("read stderr: %v", err)
	}
	_ = stdoutR.Close()
	_ = stderrR.Close()

	return outBuf.String(), errBuf.String(), code
}

func htmlServer() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, testHTML)
	}))
}

// --- success: default markdown ---

func TestDefaultMarkdown(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout == "" {
		t.Fatal("stdout is empty, want non-empty markdown")
	}
	if !strings.Contains(stdout, "Hello World") {
		t.Errorf("stdout = %q, want it to contain 'Hello World'", stdout)
	}
	if !strings.Contains(stdout, "#") {
		t.Errorf("stdout = %q, want markdown heading with '#'", stdout)
	}
}

// --- success: --raw ---

func TestRawMode(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL, "--raw"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "<") {
		t.Errorf("--raw stdout = %q, want raw HTML with '<'", stdout)
	}
	if !strings.Contains(stdout, "Hello World") {
		t.Errorf("--raw stdout = %q, want it to contain 'Hello World'", stdout)
	}
}

// --- success: --text ---

func TestTextMode(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL, "--text"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout == "" {
		t.Fatal("--text stdout is empty")
	}
	if strings.Contains(stdout, "# ") {
		t.Errorf("--text stdout %q must not contain markdown heading '# '", stdout)
	}
	if strings.Contains(stdout, "](") {
		t.Errorf("--text stdout %q must not contain markdown link syntax", stdout)
	}
	if !strings.Contains(stdout, "Hello World") {
		t.Errorf("--text stdout = %q, want it to contain 'Hello World'", stdout)
	}
}

// --- success: --json ---

func TestJSONMode(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL, "--json"}, "")
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
	if int(obj["status"].(float64)) != 200 {
		t.Errorf("status = %v, want 200", obj["status"])
	}
	urlField, _ := obj["url"].(string)
	if !strings.HasPrefix(urlField, srv.URL) {
		t.Errorf("url = %q, want prefix %q", urlField, srv.URL)
	}
}

func TestTokenEstimatePositive(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL, "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	est := obj["token_estimate"].(float64)
	if est <= 0 {
		t.Errorf("token_estimate = %v, want > 0", est)
	}
}

// --- success: stdin URL ---

func TestStdinURL(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	stdout, _, code := runGrab(t, nil, srv.URL+"\n")
	if code != 0 {
		t.Fatalf("stdin URL: exit code = %d, want 0", code)
	}
	if stdout == "" {
		t.Fatal("stdout is empty")
	}
}

// --- relative link resolution ---

func TestRelativeLinksResolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html><html><body><main>
<a href="/about">About</a>
<a href="notes/">Notes</a>
<a href="https://example.com">External</a>
</main></body></html>`)
	}))
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "](/about)") {
		t.Errorf("relative link /about was not resolved, stdout = %q", stdout)
	}
	if !strings.Contains(stdout, srv.URL+"/about") {
		t.Errorf("want resolved link %q in stdout, got %q", srv.URL+"/about", stdout)
	}
	// Absolute links must pass through unchanged.
	if !strings.Contains(stdout, "https://example.com") {
		t.Errorf("absolute link was mangled, stdout = %q", stdout)
	}
}

// --- link text normalization ---

func TestLinkTextNoNewlines(t *testing.T) {
	// Anchor wrapping a heading + paragraph (common in project list pages).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html><html><body><main>
<ul><li><a href="/project"><h3>mudra</h3><p>Virtual points system</p></a></li></ul>
</main></body></html>`)
	}))
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "[\n") || strings.Contains(stdout, "\n]") {
		t.Errorf("link text contains newline, markdown is broken:\n%s", stdout)
	}
}

// --- User-Agent header ---

func TestUserAgentHeader(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, testHTML)
	}))
	defer srv.Close()

	_, _, code := runGrab(t, []string{srv.URL}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if gotUA != "vrk/0 (https://vrk.sh)" {
		t.Errorf("User-Agent = %q, want %q", gotUA, "vrk/0 (https://vrk.sh)")
	}
}

// --- non-HTML content type ---

func TestNonHTMLContentType(t *testing.T) {
	body := `{"key":"value"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL}, "")
	if code != 0 {
		t.Fatalf("non-HTML content type: exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, body) {
		t.Errorf("stdout = %q, want it to contain raw body %q", stdout, body)
	}
}

func TestNonHTMLWithJSONFlag(t *testing.T) {
	body := `{"key":"value"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL, "--json"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("invalid JSON: %v\ngot: %q", err, stdout)
	}
	content, _ := obj["content"].(string)
	if !strings.Contains(content, "key") {
		t.Errorf("content = %q, want it to contain raw body", content)
	}
}

func TestNonHTMLWithTextFlag(t *testing.T) {
	body := `{"key":"value"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprint(w, body)
	}))
	defer srv.Close()

	// --text is a no-op for non-HTML; body passes through unchanged
	stdout, _, code := runGrab(t, []string{srv.URL, "--text"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, body) {
		t.Errorf("stdout = %q, want raw body %q (--text is no-op for non-HTML)", stdout, body)
	}
}

// --- error cases ---

func TestInvalidURLNoScheme(t *testing.T) {
	stdout, _, code := runGrab(t, []string{"not a url"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
}

func TestInvalidURLFTPScheme(t *testing.T) {
	stdout, _, code := runGrab(t, []string{"ftp://example.com"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage error)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
}

func TestHTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	stdout, stderr, code := runGrab(t, []string{srv.URL}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "404") {
		t.Errorf("stderr = %q, want it to contain '404'", stderr)
	}
}

func TestHTTP500(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
}

func TestNoURLEmptyStdin(t *testing.T) {
	stdout, _, code := runGrab(t, nil, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (no URL provided)", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
}

func TestUnknownFlag(t *testing.T) {
	stdout, _, code := runGrab(t, []string{"--unknown-flag"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
}

func TestMutuallyExclusiveFlags(t *testing.T) {
	stdout, _, code := runGrab(t, []string{"--text", "--raw", "http://example.com"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on usage error, got %q", stdout)
	}
}

func TestHelp(t *testing.T) {
	stdout, _, code := runGrab(t, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "grab") {
		t.Errorf("--help stdout = %q, want it to contain 'grab'", stdout)
	}
}

func TestRedirectCapExceeded(t *testing.T) {
	hop := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hop++
		http.Redirect(w, r, fmt.Sprintf("/?n=%d", hop), http.StatusFound)
	}))
	defer srv.Close()

	stdout, stderr, code := runGrab(t, []string{srv.URL}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "too many redirects") {
		t.Errorf("stderr = %q, want it to contain 'too many redirects'", stderr)
	}
}

// --- --json error routing ---

func TestJSONModeHTTP404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	stdout, stderr, code := runGrab(t, []string{srv.URL, "--json"}, "")
	if code != 1 {
		t.Fatalf("--json + 404: exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("--json + 404: stderr must be empty, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("--json + 404: stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Errorf("--json + 404: JSON missing 'error' field, got: %v", obj)
	}
}

func TestJSONModeInvalidURL(t *testing.T) {
	stdout, stderr, code := runGrab(t, []string{"ftp://example.com", "--json"}, "")
	if code != 2 {
		t.Fatalf("--json + ftp://: exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("--json + ftp://: stderr must be empty, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("--json + ftp://: stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Errorf("--json + ftp://: JSON missing 'error' field, got: %v", obj)
	}
	if int(obj["code"].(float64)) != 2 {
		t.Errorf("--json + ftp://: code = %v, want 2", obj["code"])
	}
}

func TestJSONModeNoScheme(t *testing.T) {
	stdout, stderr, code := runGrab(t, []string{"not-a-url", "--json"}, "")
	if code != 2 {
		t.Fatalf("--json + no-scheme: exit code = %d, want 2", code)
	}
	if stderr != "" {
		t.Errorf("--json + no-scheme: stderr must be empty, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("--json + no-scheme: stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Errorf("--json + no-scheme: JSON missing 'error' field, got: %v", obj)
	}
}

func TestJSONModeUnreachableHost(t *testing.T) {
	// Use a local address that no server is listening on.
	stdout, stderr, code := runGrab(t, []string{"http://127.0.0.1:1", "--json"}, "")
	if code != 1 {
		t.Fatalf("--json + unreachable: exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("--json + unreachable: stderr must be empty, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("--json + unreachable: stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Errorf("--json + unreachable: JSON missing 'error' field, got: %v", obj)
	}
}

// --- property test ---

func TestPropertyExitCodesOnly(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	cases := [][]string{
		{srv.URL},
		{srv.URL, "--raw"},
		{srv.URL, "--text"},
		{srv.URL, "--json"},
		{"not-a-url"},
		{"ftp://example.com"},
		{"--unknown"},
	}
	for _, args := range cases {
		_, _, code := runGrab(t, args, "")
		if code != 0 && code != 1 && code != 2 {
			t.Errorf("args=%v: exit code = %d, want 0, 1, or 2", args, code)
		}
	}
}
