package prompt

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
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
		Response   string `json:"response"`
		Model      string `json:"model"`
		TokensUsed int    `json:"tokens_used"`
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
	if out.TokensUsed != 6 {
		t.Errorf("tokens_used = %d, want 6", out.TokensUsed)
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
