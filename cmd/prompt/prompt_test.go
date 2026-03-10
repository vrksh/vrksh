package prompt

import (
	"bytes"
	"io"
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
	origIsTerminal := stdinIsTerminal

	// Save and restore env vars that the test may override.
	envKeys := []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "VRK_DEFAULT_MODEL"}
	origEnv := make(map[string]string, len(envKeys))
	for _, k := range envKeys {
		origEnv[k] = os.Getenv(k)
	}

	t.Cleanup(func() {
		os.Stdin = origStdin
		os.Stdout = origStdout
		os.Stderr = origStderr
		os.Args = origArgs
		stdinIsTerminal = origIsTerminal
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
	orig := stdinIsTerminal
	stdinIsTerminal = func() bool { return true }
	t.Cleanup(func() { stdinIsTerminal = orig })

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
