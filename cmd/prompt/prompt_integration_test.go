//go:build integration

package prompt

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Integration tests make real API calls. Run with:
//
//	go test -tags integration ./cmd/prompt/...
//
// Anthropic tests require ANTHROPIC_API_KEY.
// OpenAI tests require OPENAI_API_KEY.
// A test is skipped when its required key is absent.
// Tests are cheap: short prompts, minimal token counts.

// --- helpers ---

// anthropicEnv returns an env map with only ANTHROPIC_API_KEY set, skipping
// the test if the key is not present in the environment.
func anthropicEnv(t *testing.T) map[string]string {
	t.Helper()
	key := os.Getenv("ANTHROPIC_API_KEY")
	if key == "" {
		t.Skip("ANTHROPIC_API_KEY not set")
	}
	return map[string]string{"ANTHROPIC_API_KEY": key}
}

// openaiEnv returns an env map with only OPENAI_API_KEY set, skipping the
// test if the key is not present in the environment.
func openaiEnv(t *testing.T) map[string]string {
	t.Helper()
	key := os.Getenv("OPENAI_API_KEY")
	if key == "" {
		t.Skip("OPENAI_API_KEY not set")
	}
	return map[string]string{"OPENAI_API_KEY": key}
}

// checkJSONEnvelope validates the shape of a --json response envelope.
func checkJSONEnvelope(t *testing.T, stdout string) {
	t.Helper()
	var out struct {
		Response       string `json:"response"`
		Model          string `json:"model"`
		PromptTokens   int    `json:"prompt_tokens"`
		ResponseTokens int    `json:"response_tokens"`
		ElapsedMs      int64  `json:"elapsed_ms"`
	}
	if err := json.Unmarshal([]byte(stdout), &out); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw: %s", err, stdout)
	}
	if out.Response == "" {
		t.Error("response field is empty")
	}
	if out.Model == "" {
		t.Error("model field is empty")
	}
	if out.PromptTokens <= 0 {
		t.Errorf("prompt_tokens = %d, want > 0", out.PromptTokens)
	}
	if out.ResponseTokens <= 0 {
		t.Errorf("response_tokens = %d, want > 0", out.ResponseTokens)
	}
	if out.ElapsedMs <= 0 {
		t.Errorf("elapsed_ms = %d, want > 0", out.ElapsedMs)
	}
}

// checkSchemaResponse validates a JSON response against a simple name/age schema.
func checkSchemaResponse(t *testing.T, stdout string) {
	t.Helper()
	var obj map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &obj); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\nraw: %s", err, stdout)
	}
	if _, ok := obj["name"].(string); !ok {
		t.Errorf("response missing or wrong type for 'name': %v", obj)
	}
	if _, ok := obj["age"]; !ok {
		t.Errorf("response missing 'age' field: %v", obj)
	}
}

// --- Anthropic tests ---

func TestAnthropicPingPong(t *testing.T) {
	stdout, stderr, code := runPrompt(t, anthropicEnv(t), []string{},
		"Reply with exactly the word: pong")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(stdout), "pong") {
		t.Errorf("stdout does not contain 'pong':\n%s", stdout)
	}
}

func TestAnthropicJSONOutput(t *testing.T) {
	stdout, stderr, code := runPrompt(t, anthropicEnv(t), []string{"--json"},
		"Reply with exactly the word: pong")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}
	checkJSONEnvelope(t, stdout)
}

func TestAnthropicSchemaMatch(t *testing.T) {
	schema := `{"name":"string","age":"number"}`
	stdout, stderr, code := runPrompt(t, anthropicEnv(t),
		[]string{"--schema", schema},
		`Reply with: {"name":"Alice","age":30}`)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}
	checkSchemaResponse(t, stdout)
}

func TestAnthropicSchemaRetry(t *testing.T) {
	schema := `{"name":"string","age":"number"}`
	_, stderr, code := runPrompt(t, anthropicEnv(t),
		[]string{"--schema", schema, "--retry", "2"},
		`Reply with: {"name":"Alice","age":"wrong"}`)
	if code == 2 {
		t.Fatalf("exit code = 2 (usage error), want 0 or 1\nstderr: %s", stderr)
	}
	if strings.Contains(stderr, "retrying") {
		t.Logf("retries occurred: %s", stderr)
	}
}

// --- OpenAI tests ---

func TestOpenAIPingPong(t *testing.T) {
	stdout, stderr, code := runPrompt(t, openaiEnv(t),
		[]string{"--model", "gpt-4o-mini"},
		"Reply with exactly the word: pong")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}
	if !strings.Contains(strings.ToLower(stdout), "pong") {
		t.Errorf("stdout does not contain 'pong':\n%s", stdout)
	}
}

func TestOpenAIJSONOutput(t *testing.T) {
	stdout, stderr, code := runPrompt(t, openaiEnv(t),
		[]string{"--model", "gpt-4o-mini", "--json"},
		"Reply with exactly the word: pong")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}
	checkJSONEnvelope(t, stdout)
}

func TestOpenAISchemaMatch(t *testing.T) {
	schema := `{"name":"string","age":"number"}`
	stdout, stderr, code := runPrompt(t, openaiEnv(t),
		[]string{"--model", "gpt-4o-mini", "--schema", schema},
		`Reply with: {"name":"Alice","age":30}`)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}
	checkSchemaResponse(t, stdout)
}

func TestOpenAISchemaRetry(t *testing.T) {
	schema := `{"name":"string","age":"number"}`
	_, stderr, code := runPrompt(t, openaiEnv(t),
		[]string{"--model", "gpt-4o-mini", "--schema", schema, "--retry", "2"},
		`Reply with: {"name":"Alice","age":"wrong"}`)
	if code == 2 {
		t.Fatalf("exit code = 2 (usage error), want 0 or 1\nstderr: %s", stderr)
	}
	if strings.Contains(stderr, "retrying") {
		t.Logf("retries occurred: %s", stderr)
	}
}
