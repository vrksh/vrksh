package grab

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net"
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

	os.Args = append([]string{"grab", "--allow-internal"}, args...)

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

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on error.
// Exit code is unaffected. An ftp:// URL is invalid (only http/https accepted),
// triggering a usage error after the defer is registered.
func TestQuietSuppressesStderr(t *testing.T) {
	_, stderr, code := runGrab(t, []string{"--quiet", "ftp://example.com"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage: invalid URL scheme)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, "<html><body><p>hello world</p></body></html>")
	}))
	defer srv.Close()

	stdout, stderr, code := runGrab(t, []string{"--quiet", srv.URL}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	if !strings.Contains(stdout, "hello world") {
		t.Errorf("stdout = %q, want content containing 'hello world'", stdout)
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

func TestJSONErrorToStdout(t *testing.T) {
	// grab with --json and a 404 must route the error to stdout as JSON; stderr empty.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	defer srv.Close()

	stdout, stderr, code := runGrab(t, []string{srv.URL, "--json"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing key \"error\"")
	}
}

// --- SSRF protection tests ---

// runGrabNoAllowInternal is like runGrab but does NOT prepend --allow-internal,
// so the SSRF guard is active. Use for testing SSRF blocking behaviour.
func runGrabNoAllowInternal(t *testing.T, args []string, stdinContent string) (stdout, stderr string, code int) {
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

// TestSSRFBlocksLoopback verifies that requests to 127.0.0.1 are blocked
// when --allow-internal is not set.
func TestSSRFBlocksLoopback(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	_, stderr, code := runGrabNoAllowInternal(t, []string{srv.URL}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (blocked by SSRF guard)", code)
	}
	if !strings.Contains(stderr, "internal network address") {
		t.Errorf("stderr = %q, want it to mention 'internal network address'", stderr)
	}
}

// TestSSRFAllowInternalBypasses verifies that --allow-internal lets the request
// through to a localhost server.
func TestSSRFAllowInternalBypasses(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	// runGrab already prepends --allow-internal
	stdout, _, code := runGrab(t, []string{srv.URL}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (--allow-internal should bypass SSRF guard)", code)
	}
	if !strings.Contains(stdout, "Hello World") {
		t.Errorf("stdout = %q, want it to contain 'Hello World'", stdout)
	}
}

// TestSSRFBlocksLoopbackJSON verifies that --json routes the SSRF error to
// stdout as JSON with stderr empty.
func TestSSRFBlocksLoopbackJSON(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	stdout, stderr, code := runGrabNoAllowInternal(t, []string{srv.URL, "--json"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	if _, ok := obj["error"]; !ok {
		t.Error("JSON missing 'error' field")
	}
	errMsg, _ := obj["error"].(string)
	if !strings.Contains(errMsg, "internal network address") {
		t.Errorf("error = %q, want it to mention 'internal network address'", errMsg)
	}
}

// TestSSRFBlocksRedirectToInternal verifies that a public-looking URL that
// redirects to a loopback address is also blocked. The SSRF guard runs in the
// DialContext hook, so it fires on every TCP connection including redirect hops.
func TestSSRFBlocksRedirectToInternal(t *testing.T) {
	// Internal target server on localhost.
	internal := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, "<html><body><p>secret</p></body></html>")
	}))
	defer internal.Close()

	// "Public" server that redirects to the internal server.
	// Both use 127.0.0.1 in tests, but the point is the redirect hop is checked.
	public := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, internal.URL, http.StatusFound)
	}))
	defer public.Close()

	_, stderr, code := runGrabNoAllowInternal(t, []string{public.URL}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (redirect to internal should be blocked)", code)
	}
	if !strings.Contains(stderr, "internal network address") {
		t.Errorf("stderr = %q, want it to mention 'internal network address'", stderr)
	}
}

// TestIsInternalIP verifies the isInternalIP function against known private,
// loopback, and link-local addresses, plus a public address.
func TestIsInternalIP(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.1.1", true},
		{"192.168.255.255", true},
		{"169.254.169.254", true},
		{"169.254.0.1", true},
		{"::1", true},
		{"fc00::1", true},
		{"fd12:3456::1", true},
		{"fe80::1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"93.184.216.34", false},
		{"2607:f8b0:4004:800::200e", false},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		if ip == nil {
			t.Fatalf("failed to parse IP %q", tc.ip)
		}
		got := isInternalIP(ip)
		if got != tc.want {
			t.Errorf("isInternalIP(%s) = %v, want %v", tc.ip, got, tc.want)
		}
	}
}

// --- max-size tests ---

// TestMaxSizeExceeded verifies that --max-size limits the response body and
// exits 1 when the limit is hit.
func TestMaxSizeExceeded(t *testing.T) {
	bigBody := strings.Repeat("x", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, bigBody)
	}))
	defer srv.Close()

	// Set --max-size to 512 bytes, smaller than the 1024 byte response.
	_, stderr, code := runGrab(t, []string{srv.URL, "--max-size", "512"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (response exceeds max-size)", code)
	}
	if !strings.Contains(stderr, "exceeded") {
		t.Errorf("stderr = %q, want it to mention 'exceeded'", stderr)
	}
}

// TestMaxSizeExceededJSON verifies --max-size error goes to stdout as JSON.
func TestMaxSizeExceededJSON(t *testing.T) {
	bigBody := strings.Repeat("x", 1024)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, bigBody)
	}))
	defer srv.Close()

	stdout, stderr, code := runGrab(t, []string{srv.URL, "--json", "--max-size", "512"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty when --json active, got %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %q", err, stdout)
	}
	errMsg, _ := obj["error"].(string)
	if !strings.Contains(errMsg, "exceeded") {
		t.Errorf("error = %q, want it to mention 'exceeded'", errMsg)
	}
}

// TestMaxSizeDefault verifies the default max-size allows a normal small response.
func TestMaxSizeDefault(t *testing.T) {
	srv := htmlServer()
	defer srv.Close()

	stdout, _, code := runGrab(t, []string{srv.URL}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (small response within default limit)", code)
	}
	if !strings.Contains(stdout, "Hello World") {
		t.Errorf("stdout = %q, want it to contain 'Hello World'", stdout)
	}
}
