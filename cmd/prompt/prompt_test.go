package prompt

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
	"time"
)

// runPrompt sets os.Args, captures stdout/stderr, overrides environment
// variables, and calls Run(). It restores all state via t.Cleanup.
// Do not call t.Parallel() — tests share global state (os.Args, os.Stdin, etc).
func runPrompt(t *testing.T, env map[string]string, args []string, stdin string) (stdout, stderr string, code int) {
	t.Helper()

	origStdin := os.Stdin
	origStdout := os.Stdout
	origStderr := os.Stderr
	origArgs := os.Args
	origIsTerminal := isTerminal

	// Save and restore env vars that the test may override.
	envKeys := []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "VRK_DEFAULT_MODEL", "VRK_LLM_KEY", "VRK_LLM_URL"}
	origEnv := make(map[string]string, len(envKeys))
	for _, k := range envKeys {
		origEnv[k] = os.Getenv(k)
	}

	t.Cleanup(func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
		os.Stderr = origStderr
		os.Args = origArgs
		isTerminal = origIsTerminal
		for k, v := range origEnv {
			if v == "" {
				_ = os.Unsetenv(k)
			} else {
				_ = os.Setenv(k, v)
			}
		}
	})

	// Unset all tracked env keys first so tests start clean.
	for _, k := range envKeys {
		_ = os.Unsetenv(k)
	}
	// Apply test env overrides.
	for k, v := range env {
		if v == "" {
			_ = os.Unsetenv(k)
		} else {
			_ = os.Setenv(k, v)
		}
	}

	// stdin: always a pipe so Run() sees a non-TTY fd.
	stdinR, stdinW, err := os.Pipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}
	if _, err := io.WriteString(stdinW, stdin); err != nil {
		t.Fatalf("write stdin: %v", err)
	}
	if err := stdinW.Close(); err != nil {
		t.Fatalf("close stdin write: %v", err)
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

	os.Args = append([]string{"prompt"}, args...)

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

// --- Unit tests (no API calls) ---

// TestExplainExitsZero checks that --explain exits 0 and stdout contains the
// prompt text, model name, and max_tokens value.
func TestExplainExitsZero(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "hello") {
		t.Errorf("stdout does not contain prompt text 'hello':\n%s", stdout)
	}
	if !strings.Contains(stdout, "claude") {
		t.Errorf("stdout does not contain model name:\n%s", stdout)
	}
	if !strings.Contains(stdout, "4096") {
		t.Errorf("stdout does not contain max_tokens 4096:\n%s", stdout)
	}
}

// TestExplainUnknownModel checks that --explain passes unknown model names through unchanged.
func TestExplainUnknownModel(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--model", "unknown-model-xyz", "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "unknown-model-xyz") {
		t.Errorf("stdout does not contain model name 'unknown-model-xyz':\n%s", stdout)
	}
}

// TestExplainWithSchema checks that --explain shows schema content in the curl output.
func TestExplainWithSchema(t *testing.T) {
	schema := `{"name":"string"}`
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--schema", schema, "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "name") {
		t.Errorf("stdout does not contain schema content:\n%s", stdout)
	}
}

// TestExplainKeyNotLeaked checks that the literal API key value never appears
// in --explain output.
func TestExplainKeyNotLeaked(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "supersecretkey123"}
	stdout, stderr, code := runPrompt(t, env, []string{"--explain"}, "hi")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "supersecretkey123") {
		t.Errorf("--explain stdout leaked API key: %s", stdout)
	}
	if strings.Contains(stderr, "supersecretkey123") {
		t.Errorf("--explain stderr leaked API key: %s", stderr)
	}
}

// TestBudgetHardGate checks that --budget 1 exits 1 (token count exceeds budget)
// both with and without --fail. Budget check fires before any API call or key check.
// "hello world" tokenizes to 2 tokens (cl100k_base), so budget 1 is exceeded.
func TestBudgetHardGate(t *testing.T) {
	// (a) with --fail
	_, stderr, code := runPrompt(t, map[string]string{}, []string{"--budget", "1", "--fail"}, "hello world")
	if code != 1 {
		t.Errorf("(a) with --fail: exit code = %d, want 1", code)
	}
	lowerStderr := strings.ToLower(stderr)
	if !strings.Contains(lowerStderr, "token") && !strings.Contains(lowerStderr, "budget") {
		t.Errorf("(a) stderr does not mention token count or budget: %q", stderr)
	}

	// (b) without --fail
	_, stderr2, code2 := runPrompt(t, map[string]string{}, []string{"--budget", "1"}, "hello world")
	if code2 != 1 {
		t.Errorf("(b) without --fail: exit code = %d, want 1", code2)
	}
	lowerStderr2 := strings.ToLower(stderr2)
	if !strings.Contains(lowerStderr2, "token") && !strings.Contains(lowerStderr2, "budget") {
		t.Errorf("(b) stderr does not mention token count or budget: %q", stderr2)
	}
}

// TestNoStdinInteractive checks that when stdin is a terminal (simulated) and
// no positional arg and no --explain, Run() returns exit 2.
func TestNoStdinInteractive(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	_, _, code := runPrompt(t, map[string]string{}, []string{}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (interactive TTY with no input)", code)
	}
}

// TestNoAPIKey checks that when neither key is set, Run() exits 1 with the
// expected message on stderr and nothing on stdout.
func TestNoAPIKey(t *testing.T) {
	stdout, stderr, code := runPrompt(t, map[string]string{}, []string{}, "hello")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got: %q", stdout)
	}
	if !strings.Contains(stderr, "no API key found") {
		t.Errorf("stderr does not contain 'no API key found': %q", stderr)
	}
	if !strings.Contains(stderr, "ANTHROPIC_API_KEY") {
		t.Errorf("stderr does not mention ANTHROPIC_API_KEY: %q", stderr)
	}
}

// TestKeyValueNotInOutput checks that a fake API key value never appears in
// stdout or stderr. Uses --explain to avoid any real network call — the key
// safety guarantee must hold before a request is ever made.
func TestKeyValueNotInOutput(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "VERYSECRETAPIKEY999"}
	stdout, stderr, code := runPrompt(t, env, []string{"--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "VERYSECRETAPIKEY999") {
		t.Errorf("stdout contains key value: %s", stdout)
	}
	if strings.Contains(stderr, "VERYSECRETAPIKEY999") {
		t.Errorf("stderr contains key value: %s", stderr)
	}
}

// TestPositionalArg checks that a positional argument works like stdin when
// combined with --explain.
func TestPositionalArg(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--explain", "hello world"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "hello world") {
		t.Errorf("stdout does not contain positional arg text:\n%s", stdout)
	}
}

// TestHelpExitsZero checks that --help exits 0.
func TestHelpExitsZero(t *testing.T) {
	stdout, _, code := runPrompt(t, map[string]string{}, []string{"--help"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout == "" {
		t.Error("--help: stdout is empty, want usage text")
	}
}

// TestExplainNeverLeaksKey is a property test: for various fake key values,
// --explain output must never contain the literal key.
func TestExplainNeverLeaksKey(t *testing.T) {
	keys := []string{
		"sk-ant-testkey001",
		"sk-testkey002abcdef",
		"supersecretvalue",
		"Bearer eyJtestkey",
		"verylongsecretkey1234567890",
	}
	for _, key := range keys {
		env := map[string]string{"ANTHROPIC_API_KEY": key}
		stdout, stderr, code := runPrompt(t, env, []string{"--explain"}, "test prompt")
		if code != 0 {
			t.Errorf("key=%q: exit code = %d, want 0", key, code)
			continue
		}
		if strings.Contains(stdout, key) {
			t.Errorf("key=%q: stdout contains the key value", key)
		}
		if strings.Contains(stderr, key) {
			t.Errorf("key=%q: stderr contains the key value", key)
		}
	}
}

// --- custom endpoint tests ---

// mockOpenAIResponse is a valid OpenAI chat completions response used across endpoint tests.
const mockOpenAIResponse = `{"choices":[{"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}],"model":"llama3.2","usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`

// newMockServer returns an httptest.Server that always responds with mockOpenAIResponse.
func newMockServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockOpenAIResponse))
	}))
}

// TestEndpointPathAppend is a unit test for resolveEndpoint covering the five
// cases from the spec. No server needed.
func TestEndpointPathAppend(t *testing.T) {
	cases := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"", "", false},
		{"http://localhost:11434", "http://localhost:11434/v1/chat/completions", false},
		{"http://localhost:11434/v1", "http://localhost:11434/v1/chat/completions", false},
		{"http://localhost:11434/v1/chat/completions", "http://localhost:11434/v1/chat/completions", false},
		{"not a url", "", true},
	}
	for _, c := range cases {
		got, err := resolveEndpoint(c.input)
		if c.wantErr {
			if err == nil {
				t.Errorf("resolveEndpoint(%q): want error, got nil (result=%q)", c.input, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("resolveEndpoint(%q): unexpected error: %v", c.input, err)
			continue
		}
		if got != c.want {
			t.Errorf("resolveEndpoint(%q) = %q, want %q", c.input, got, c.want)
		}
	}
}

// TestEndpointInvalidURL checks that a bad --endpoint value exits 2.
func TestEndpointInvalidURL(t *testing.T) {
	_, _, code := runPrompt(t, map[string]string{}, []string{"--endpoint", "not a url"}, "hello")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (invalid endpoint URL)", code)
	}
}

// TestEndpointNoModel checks that --endpoint without --model (and no VRK_DEFAULT_MODEL) exits 2.
func TestEndpointNoModel(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	_, stderr, code := runPrompt(t, map[string]string{}, []string{"--endpoint", srv.URL}, "hello")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (--model required)", code)
	}
	if !strings.Contains(stderr, "--model") {
		t.Errorf("stderr does not mention --model: %q", stderr)
	}
}

// TestEndpointFlagExplain checks that --endpoint + --explain prints curl to the
// resolved URL and makes no HTTP request.
func TestEndpointFlagExplain(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", srv.URL + "/v1", "--model", "llama3.2", "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "/v1/chat/completions") {
		t.Errorf("curl output does not contain /v1/chat/completions:\n%s", stdout)
	}
	if called {
		t.Error("--explain made an HTTP request; it should not")
	}
}

// TestEndpointExplainNoModel checks that --endpoint + --explain without --model
// exits 0 (explain bypasses the model guard — it is for debugging, not execution).
func TestEndpointExplainNoModel(t *testing.T) {
	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", "http://localhost:11434/v1", "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("--explain without --model: exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "chat/completions") {
		t.Errorf("--explain stdout does not contain endpoint URL:\n%s", stdout)
	}
}

// TestEndpointPrecedence checks that --endpoint takes priority over both
// ANTHROPIC_API_KEY and OPENAI_API_KEY — the request must hit the mock server.
func TestEndpointPrecedence(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	env := map[string]string{
		"ANTHROPIC_API_KEY": "fake-anthropic",
		"OPENAI_API_KEY":    "fake-openai",
	}
	stdout, _, code := runPrompt(t, env, []string{"--endpoint", srv.URL, "--model", "llama3.2"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "pong") {
		t.Errorf("stdout does not contain mock response 'pong': %q", stdout)
	}
}

// TestEndpointNoAuthHeader checks that when VRK_LLM_KEY is not set, no
// Authorization header is sent to the endpoint.
func TestEndpointNoAuthHeader(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockOpenAIResponse))
	}))
	defer srv.Close()

	_, _, code := runPrompt(t, map[string]string{}, []string{"--endpoint", srv.URL, "--model", "llama3.2"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if gotAuth != "" {
		t.Errorf("Authorization header present, want none: %q", gotAuth)
	}
}

// TestEndpointWithAPIKey checks that VRK_LLM_KEY is sent as the Bearer token.
func TestEndpointWithAPIKey(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(mockOpenAIResponse))
	}))
	defer srv.Close()

	_, _, code := runPrompt(t, map[string]string{"VRK_LLM_KEY": "testkey"},
		[]string{"--endpoint", srv.URL, "--model", "llama3.2"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if gotAuth != "Bearer testkey" {
		t.Errorf("Authorization = %q, want %q", gotAuth, "Bearer testkey")
	}
}

// TestVRKLLMURLEnv checks that VRK_LLM_URL (without --endpoint flag) routes the
// request to the given server.
func TestVRKLLMURLEnv(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdout, _, code := runPrompt(t, map[string]string{"VRK_LLM_URL": srv.URL},
		[]string{"--model", "llama3.2"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "pong") {
		t.Errorf("stdout does not contain mock response 'pong': %q", stdout)
	}
}

// TestEndpointRealCall checks that a successful endpoint call returns exit 0 and
// the response text on stdout.
func TestEndpointRealCall(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", srv.URL, "--model", "llama3.2"}, "Reply with: pong")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "pong") {
		t.Errorf("stdout does not contain 'pong': %q", stdout)
	}
}

// TestEndpointRealCallJSON checks the --json envelope shape for endpoint calls.
func TestEndpointRealCallJSON(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", srv.URL, "--model", "llama3.2", "--json"}, "Reply with: pong")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var out struct {
		Response       string `json:"response"`
		Model          string `json:"model"`
		PromptTokens   int    `json:"prompt_tokens"`
		ResponseTokens int    `json:"response_tokens"`
		ElapsedMs      int64  `json:"elapsed_ms"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &out); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %s", err, stdout)
	}
	if out.Response != "pong" {
		t.Errorf("response = %q, want %q", out.Response, "pong")
	}
	if out.Model != "llama3.2" {
		t.Errorf("model = %q, want %q", out.Model, "llama3.2")
	}
	if out.PromptTokens != 5 {
		t.Errorf("prompt_tokens = %d, want 5", out.PromptTokens)
	}
	if out.ResponseTokens != 1 {
		t.Errorf("response_tokens = %d, want 1", out.ResponseTokens)
	}
}

// TestEndpointUnreachable checks that an endpoint with nothing listening exits 1
// with an error on stderr.
func TestEndpointUnreachable(t *testing.T) {
	_, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", "http://localhost:9", "--model", "llama3.2"}, "hello")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	lower := strings.ToLower(stderr)
	if !strings.Contains(lower, "connection refused") && !strings.Contains(lower, "request failed") && !strings.Contains(lower, "failed") {
		t.Errorf("stderr does not mention connection failure: %q", stderr)
	}
}

func TestJSONErrorToStdout(t *testing.T) {
	// No API key with --json must route the error to stdout as JSON; stderr empty.
	stdout, stderr, code := runPrompt(t, map[string]string{}, []string{"--json"}, "hello")
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
	if c, _ := obj["code"].(float64); int(c) != 1 {
		t.Errorf("code = %v, want 1", obj["code"])
	}
}

// TestJSONUsageErrorsToStdout verifies that usage errors (exit 2) also route to
// stdout as JSON when --json is set, leaving stderr empty.
func TestJSONUsageErrorsToStdout(t *testing.T) {
	assertJSONUsageError := func(t *testing.T, label, stdout, stderr string, code int) {
		t.Helper()
		if code != 2 {
			t.Fatalf("%s: exit code = %d, want 2", label, code)
		}
		if stderr != "" {
			t.Errorf("%s: stderr must be empty when --json active, got %q", label, stderr)
		}
		var obj map[string]any
		if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
			t.Fatalf("%s: stdout is not valid JSON: %v\ngot: %q", label, err, stdout)
		}
		if _, ok := obj["error"]; !ok {
			t.Errorf("%s: JSON missing key \"error\"", label)
		}
		if c, _ := obj["code"].(float64); int(c) != 2 {
			t.Errorf("%s: code = %v, want 2", label, obj["code"])
		}
	}

	t.Run("invalid endpoint URL", func(t *testing.T) {
		stdout, stderr, code := runPrompt(t, map[string]string{},
			[]string{"--json", "--endpoint", "not a url"}, "hello")
		assertJSONUsageError(t, "invalid endpoint URL", stdout, stderr, code)
	})

	t.Run("missing --model with endpoint", func(t *testing.T) {
		srv := newMockServer(t)
		defer srv.Close()
		stdout, stderr, code := runPrompt(t, map[string]string{},
			[]string{"--json", "--endpoint", srv.URL}, "hello")
		assertJSONUsageError(t, "missing --model", stdout, stderr, code)
	})

	t.Run("no input on TTY", func(t *testing.T) {
		orig := isTerminal
		isTerminal = func(int) bool { return true }
		t.Cleanup(func() { isTerminal = orig })
		stdout, stderr, code := runPrompt(t, map[string]string{}, []string{"--json"}, "")
		assertJSONUsageError(t, "no input on TTY", stdout, stderr, code)
	})
}

// --- --system flag tests ---

// TestPromptSystemBasic checks that --system 'text' + --explain produces Anthropic
// curl output with a "system" field containing the provided text.
func TestPromptSystemBasic(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--system", "You are a classifier.", "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "You are a classifier.") {
		t.Errorf("stdout does not contain system prompt text:\n%s", stdout)
	}
	if !strings.Contains(stdout, `"system"`) {
		t.Errorf("stdout does not contain \"system\" key:\n%s", stdout)
	}
}

// TestPromptSystemFromFile checks that --system @tmpfile reads the file and
// includes its content in the explain output.
func TestPromptSystemFromFile(t *testing.T) {
	tmp, err := os.CreateTemp("", "vrk-sys-*.txt")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck
	if _, err := tmp.WriteString("You are a summariser."); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = tmp.Close()

	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--system", "@" + tmp.Name(), "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "You are a summariser.") {
		t.Errorf("stdout does not contain file content:\n%s", stdout)
	}
}

// TestPromptSystemFromAbsolutePath confirms that @/absolute/path works correctly
// and the @ stripping doesn't break absolute paths (e.g. via naive filepath.Join).
func TestPromptSystemFromAbsolutePath(t *testing.T) {
	tmp, err := os.CreateTemp("", "vrk-sys-abs-*.txt")
	if err != nil {
		t.Fatalf("creating temp file: %v", err)
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck
	if _, err := tmp.WriteString("Absolute path content."); err != nil {
		t.Fatalf("writing temp file: %v", err)
	}
	_ = tmp.Close()

	// tmp.Name() is already an absolute path
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--system", "@" + tmp.Name(), "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "Absolute path content.") {
		t.Errorf("stdout does not contain file content:\n%s", stdout)
	}
}

// TestPromptSystemFileNotFound checks that --system @missing.txt exits 1 with
// the exact error format: "prompt: system prompt file not found: missing.txt"
func TestPromptSystemFileNotFound(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	_, stderr, code := runPrompt(t, env, []string{"--system", "@missing.txt"}, "hello")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "prompt: system prompt file not found: missing.txt") {
		t.Errorf("stderr does not contain expected message, got: %q", stderr)
	}
}

// TestPromptSystemEmptyValue checks that --system with an empty string exits 2
// with the message: "prompt: --system value cannot be empty"
func TestPromptSystemEmptyValue(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	_, stderr, code := runPrompt(t, env, []string{"--system", ""}, "hello")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "prompt: --system value cannot be empty") {
		t.Errorf("stderr does not contain expected message, got: %q", stderr)
	}
}

// TestPromptSystemWithPositional checks that --system and a positional arg both
// appear in the explain output.
func TestPromptSystemWithPositional(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--system", "You are a reviewer.", "--explain", "Review this code"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "You are a reviewer.") {
		t.Errorf("stdout missing system prompt:\n%s", stdout)
	}
	if !strings.Contains(stdout, "Review this code") {
		t.Errorf("stdout missing positional arg:\n%s", stdout)
	}
}

// TestPromptSystemWithStdin checks that --system and stdin input both appear in
// the explain output.
func TestPromptSystemWithStdin(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--system", "You are a support agent.", "--explain"}, "My app is crashing")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "You are a support agent.") {
		t.Errorf("stdout missing system prompt:\n%s", stdout)
	}
	if !strings.Contains(stdout, "My app is crashing") {
		t.Errorf("stdout missing stdin content:\n%s", stdout)
	}
}

// TestPromptSystemAbsent verifies that when --system is not set, the Anthropic
// --explain output does not contain a "system" key in the JSON body.
func TestPromptSystemAbsent(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// The curl body should not have a "system" field when --system is absent
	// and --schema is also absent.
	if strings.Contains(stdout, `"system"`) {
		t.Errorf("stdout contains \"system\" key when --system not set:\n%s", stdout)
	}
}

// TestPromptSystemJSONOutput checks that --system + --json includes the
// system_prompt field in the JSON output with the correct value.
func TestPromptSystemJSONOutput(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--system", "You are helpful.", "--endpoint", srv.URL, "--model", "llama3.2", "--json"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %s", err, stdout)
	}
	sp, ok := obj["system_prompt"]
	if !ok {
		t.Fatal("system_prompt key missing from JSON output")
	}
	if sp != "You are helpful." {
		t.Errorf("system_prompt = %q, want %q", sp, "You are helpful.")
	}
}

// TestPromptSystemJSONOutputAbsent checks that when --system is not set, the
// system_prompt key is absent from --json output.
func TestPromptSystemJSONOutputAbsent(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", srv.URL, "--model", "llama3.2", "--json"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %s", err, stdout)
	}
	if _, ok := obj["system_prompt"]; ok {
		t.Error("system_prompt key present in JSON output when --system not set")
	}
}

// TestPromptSystemExplain checks that --system text appears verbatim in the
// --explain curl output.
func TestPromptSystemExplain(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--system", "You are helpful.", "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "You are helpful.") {
		t.Errorf("system prompt text not found in --explain output:\n%s", stdout)
	}
}

// TestPromptSystemOpenAI checks that --system with an OpenAI model produces a
// system role message in the --explain output's messages array.
func TestPromptSystemOpenAI(t *testing.T) {
	env := map[string]string{"OPENAI_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--system", "You are a translator.", "--model", "gpt-4o", "--explain"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, `"role":"system"`) {
		t.Errorf("stdout missing system role message:\n%s", stdout)
	}
	if !strings.Contains(stdout, "You are a translator.") {
		t.Errorf("stdout missing system prompt text:\n%s", stdout)
	}
}

// --- --quiet flag tests ---

// TestQuietSuppressesStderr verifies that --quiet suppresses stderr on error.
// Exit code is unaffected. TTY with no input triggers the usage error after
// the defer is registered.
func TestQuietSuppressesStderr(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	_, stderr, code := runPrompt(t, map[string]string{}, []string{"--quiet"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2 (usage: no input)", code)
	}
	if stderr != "" {
		t.Errorf("--quiet: stderr = %q, want empty", stderr)
	}
}

// TestQuietDoesNotAffectStdout verifies that --quiet does not suppress stdout
// on success. --explain is used so no API call is made.
func TestQuietDoesNotAffectStdout(t *testing.T) {
	stdout, stderr, code := runPrompt(t, map[string]string{}, []string{"--explain", "--quiet"}, "what is 2+2")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty on success, got %q", stderr)
	}
	if !strings.Contains(stdout, "curl") {
		t.Errorf("stdout = %q, want --explain curl output", stdout)
	}
}

// --- --field tests ---

// newCountingMockServer returns a mock OpenAI-compatible server that returns
// a different response content for each sequential call. Once all contents
// are exhausted, it repeats the last one.
func newCountingMockServer(t *testing.T, contents []string) *httptest.Server {
	t.Helper()
	callCount := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := callCount
		if idx >= len(contents) {
			idx = len(contents) - 1
		}
		callCount++
		contentJSON, _ := json.Marshal(contents[idx])
		resp := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":%s},"finish_reason":"stop"}],"model":"test","usage":{"prompt_tokens":5,"completion_tokens":1,"total_tokens":6}}`, string(contentJSON))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
}

// TestPromptJSONFieldNames verifies that --json output uses the normalized
// field names (prompt_tokens, response_tokens, elapsed_ms) and not the old
// names (tokens_used, latency_ms, request_hash).
func TestPromptJSONFieldNames(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", srv.URL, "--model", "llama3.2", "--json"}, "hello")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\ngot: %s", err, stdout)
	}
	// New field names must be present.
	for _, key := range []string{"response", "model", "prompt_tokens", "response_tokens", "elapsed_ms"} {
		if _, ok := obj[key]; !ok {
			t.Errorf("missing key %q in JSON output", key)
		}
	}
	// Old field names must be absent.
	for _, key := range []string{"tokens_used", "latency_ms", "request_hash"} {
		if _, ok := obj[key]; ok {
			t.Errorf("deprecated key %q still present in JSON output", key)
		}
	}
}

// TestFieldBasic checks that --field reads JSONL line by line and produces
// one output line per input record.
func TestFieldBasic(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdin := "{\"text\":\"hello\"}\n{\"text\":\"world\"}\n"
	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--endpoint", srv.URL, "--model", "test"}, stdin)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d output lines, want 2:\n%s", len(lines), stdout)
	}
	for i, line := range lines {
		if !strings.Contains(line, "pong") {
			t.Errorf("line %d does not contain 'pong': %q", i+1, line)
		}
	}
}

// TestFieldWithJSON checks that --field --json merges input record fields
// with response metadata fields in the output.
func TestFieldWithJSON(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdin := "{\"index\":0,\"text\":\"hello\",\"tokens\":5}\n{\"index\":1,\"text\":\"world\",\"tokens\":5}\n"
	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--json", "--endpoint", srv.URL, "--model", "test"}, stdin)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d output lines, want 2:\n%s", len(lines), stdout)
	}
	for i, line := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(line), &obj); err != nil {
			t.Fatalf("line %d: invalid JSON: %v", i+1, err)
		}
		// Input fields preserved.
		if _, ok := obj["index"]; !ok {
			t.Errorf("line %d: missing input field 'index'", i+1)
		}
		if _, ok := obj["tokens"]; !ok {
			t.Errorf("line %d: missing input field 'tokens'", i+1)
		}
		// Response fields added.
		for _, key := range []string{"response", "model", "prompt_tokens", "response_tokens", "elapsed_ms"} {
			if _, ok := obj[key]; !ok {
				t.Errorf("line %d: missing response field %q", i+1, key)
			}
		}
	}
}

// TestFieldMissingField checks that a record missing the named field exits 1
// with an error mentioning the field name and line number.
func TestFieldMissingField(t *testing.T) {
	stdin := "{\"other\":\"hello\"}\n"
	_, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--endpoint", "http://localhost:1", "--model", "test"}, stdin)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "line 1") {
		t.Errorf("stderr does not mention line number: %q", stderr)
	}
	if !strings.Contains(stderr, `"text"`) {
		t.Errorf("stderr does not mention field name: %q", stderr)
	}
}

// TestFieldInvalidJSON checks that invalid JSON on line 2 exits 1 after
// line 1 succeeds. Requires a mock server because line 1 makes an API call.
func TestFieldInvalidJSON(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdin := "{\"text\":\"hello\"}\nnot json\n"
	stdout, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--endpoint", srv.URL, "--model", "test"}, stdin)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	// Line 1 should have succeeded - stdout should have one record.
	trimmed := strings.TrimSpace(stdout)
	if trimmed == "" {
		t.Error("stdout is empty, expected one record from line 1")
	} else {
		lines := strings.Split(trimmed, "\n")
		if len(lines) != 1 {
			t.Errorf("expected 1 output line from successful line 1, got %d: %q", len(lines), stdout)
		}
	}
	if !strings.Contains(stderr, "line 2") {
		t.Errorf("stderr does not mention line 2: %q", stderr)
	}
	if !strings.Contains(stderr, "invalid JSON") {
		t.Errorf("stderr does not mention 'invalid JSON': %q", stderr)
	}
}

// TestFieldNonStringValue checks that a non-string field value exits 1.
func TestFieldNonStringValue(t *testing.T) {
	stdin := "{\"text\":42}\n"
	_, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--endpoint", "http://localhost:1", "--model", "test"}, stdin)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "not a string") {
		t.Errorf("stderr does not mention 'not a string': %q", stderr)
	}
}

// TestFieldEmptyInput checks that --field with empty stdin exits 0 with no output.
func TestFieldEmptyInput(t *testing.T) {
	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--endpoint", "http://localhost:1", "--model", "test"}, "")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
}

// TestFieldBudgetExceeded checks that --budget is enforced per record and
// exits 1 before making an API call.
func TestFieldBudgetExceeded(t *testing.T) {
	// "hello world" is ~2 tokens (cl100k_base), budget of 1 exceeds.
	stdin := "{\"text\":\"hello world\"}\n"
	_, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--budget", "1", "--endpoint", "http://localhost:1", "--model", "test"}, stdin)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(strings.ToLower(stderr), "budget") {
		t.Errorf("stderr does not mention budget: %q", stderr)
	}
}

// TestFieldSchemaRetry checks that --schema with --retry retries on schema
// mismatch and succeeds when the response eventually matches.
func TestFieldSchemaRetry(t *testing.T) {
	srv := newCountingMockServer(t, []string{
		"not json at all",   // attempt 1: fails schema
		`{"wrong":"field"}`, // attempt 2: fails schema (missing "name")
		`{"name":"Alice"}`,  // attempt 3: passes schema
	})
	defer srv.Close()

	stdin := "{\"text\":\"test\"}\n"
	stdout, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--schema", `{"name":"string"}`, "--retry", "2",
			"--endpoint", srv.URL, "--model", "test"}, stdin)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s\nstdout: %s", code, stderr, stdout)
	}
	if !strings.Contains(stdout, "Alice") {
		t.Errorf("stdout does not contain final successful response: %q", stdout)
	}
}

// TestFieldWithExplain checks that --field and --explain are mutually exclusive (exit 2).
func TestFieldWithExplain(t *testing.T) {
	_, stderr, code := runPrompt(t, map[string]string{"ANTHROPIC_API_KEY": "fake"},
		[]string{"--field", "text", "--explain"}, "{\"text\":\"hello\"}\n")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Errorf("stderr does not mention 'mutually exclusive': %q", stderr)
	}
}

// TestFieldTTY checks that --field with TTY stdin exits 2.
func TestFieldTTY(t *testing.T) {
	orig := isTerminal
	isTerminal = func(int) bool { return true }
	t.Cleanup(func() { isTerminal = orig })

	_, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text"}, "")
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "--field requires piped input") {
		t.Errorf("stderr does not mention piped input: %q", stderr)
	}
}

// TestFieldResponseOverwrite checks that when an input record already has a
// "response" field, it is overwritten and a warning is emitted to stderr.
func TestFieldResponseOverwrite(t *testing.T) {
	srv := newMockServer(t)
	defer srv.Close()

	stdin := "{\"text\":\"hello\",\"response\":\"old\"}\n"
	stdout, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--json", "--endpoint", srv.URL, "--model", "test"}, stdin)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stderr, "response") {
		t.Errorf("stderr does not warn about 'response' field overwrite: %q", stderr)
	}
	var obj map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if obj["response"] != "pong" {
		t.Errorf("response = %v, want 'pong'", obj["response"])
	}
}

// TestFieldNewlineEscape checks that newlines within LLM responses are escaped
// as literal \n in non-JSON output so each response stays on one line.
func TestFieldNewlineEscape(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := "line1\nline2\nline3"
		contentJSON, _ := json.Marshal(content)
		resp := fmt.Sprintf(`{"choices":[{"message":{"role":"assistant","content":%s},"finish_reason":"stop"}],"model":"test","usage":{"prompt_tokens":5,"completion_tokens":3,"total_tokens":8}}`, string(contentJSON))
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	stdin := "{\"text\":\"hello\"}\n"
	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--field", "text", "--endpoint", srv.URL, "--model", "test"}, stdin)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected 1 output line (newlines escaped), got %d:\n%s", len(lines), stdout)
	}
	if !strings.Contains(lines[0], `\n`) {
		t.Errorf("output does not contain escaped newlines: %q", lines[0])
	}
}

// --- M3: HTTP client timeout ---

// TestAPIClientHasTimeout verifies that the package-level apiClient has a
// non-zero timeout set. This is a unit test - it reads the Timeout field
// directly rather than waiting for an actual timeout.
func TestAPIClientHasTimeout(t *testing.T) {
	if apiClient == nil {
		t.Fatal("apiClient is nil")
	}
	if apiClient.Timeout == 0 {
		t.Error("apiClient.Timeout is 0, want non-zero")
	}
	if apiClient.Timeout != 120*time.Second {
		t.Errorf("apiClient.Timeout = %v, want %v", apiClient.Timeout, 120*time.Second)
	}
}

// TestHTTPTimeoutConstant verifies the named constant matches the expected value.
func TestHTTPTimeoutConstant(t *testing.T) {
	if httpTimeout != 120*time.Second {
		t.Errorf("httpTimeout = %v, want 120s", httpTimeout)
	}
}

// --- L1: single quote escaping in --explain ---

// TestExplainSingleQuoteEscaping verifies that a prompt containing single
// quotes produces valid, copy-pasteable curl output. The shell pattern for
// embedding a single quote inside a single-quoted string is: '\”
func TestExplainSingleQuoteEscaping(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--explain"}, "it's a test")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	// The raw single quote from "it's" must not appear unescaped inside the
	// -d '...' argument. It should be escaped as '\''
	if strings.Contains(stdout, "it's a test") {
		t.Errorf("stdout contains unescaped single quote in curl body:\n%s", stdout)
	}
	if !strings.Contains(stdout, `'\''`) {
		t.Errorf("stdout does not contain escaped single quote sequence '\\'':\n%s", stdout)
	}
}

// TestExplainSingleQuoteOpenAI tests single quote escaping with the OpenAI provider.
func TestExplainSingleQuoteOpenAI(t *testing.T) {
	env := map[string]string{"OPENAI_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--explain"}, "don't stop")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "don't stop") {
		t.Errorf("stdout contains unescaped single quote:\n%s", stdout)
	}
	if !strings.Contains(stdout, `'\''`) {
		t.Errorf("stdout missing escaped single quote sequence:\n%s", stdout)
	}
}

// TestExplainSingleQuoteEndpoint tests single quote escaping with a custom endpoint.
func TestExplainSingleQuoteEndpoint(t *testing.T) {
	stdout, _, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", "http://localhost:11434/v1", "--explain"}, "it's fine")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if strings.Contains(stdout, "it's fine") {
		t.Errorf("stdout contains unescaped single quote:\n%s", stdout)
	}
	if !strings.Contains(stdout, `'\''`) {
		t.Errorf("stdout missing escaped single quote sequence:\n%s", stdout)
	}
}

// TestExplainNoQuotesUnchanged verifies that prompts without single quotes
// still render correctly in --explain output.
func TestExplainNoQuotesUnchanged(t *testing.T) {
	env := map[string]string{"ANTHROPIC_API_KEY": "fake"}
	stdout, _, code := runPrompt(t, env, []string{"--explain"}, "hello world")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, "hello world") {
		t.Errorf("stdout does not contain prompt text 'hello world':\n%s", stdout)
	}
}

// --- L2: response body size limit ---

// TestMaxResponseBytesConstant verifies the limit constant is 50MB.
func TestMaxResponseBytesConstant(t *testing.T) {
	want := int64(50 * 1024 * 1024)
	if maxResponseBytes != want {
		t.Errorf("maxResponseBytes = %d, want %d (50MB)", maxResponseBytes, want)
	}
}

// TestResponseBodyLimit verifies that an API response larger than
// maxResponseBytes results in exit 1 with a clear error message.
func TestResponseBodyLimit(t *testing.T) {
	// Save and restore the original maxResponseBytes so we can test with a
	// tiny limit without needing a 50MB response in the test.
	origMax := maxResponseBytes
	maxResponseBytes = 64 // 64 bytes
	t.Cleanup(func() { maxResponseBytes = origMax })

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Write a response larger than 64 bytes.
		_, _ = w.Write([]byte(strings.Repeat("x", 128)))
	}))
	defer srv.Close()

	_, stderr, code := runPrompt(t, map[string]string{},
		[]string{"--endpoint", srv.URL, "--model", "test"}, "hello")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "exceeded") {
		t.Errorf("stderr = %q, want it to mention 'exceeded'", stderr)
	}
}
