package prompt

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/tokcount"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// httpTimeout is the timeout for all LLM API calls. LLM responses can be slow
// (large completions, busy endpoints), so 120s is generous but bounded.
const httpTimeout = 120 * time.Second

// maxResponseBytes is the maximum API response body size (50MB). Protects
// against a malicious or misconfigured endpoint returning unbounded data.
// Var so tests can override without allocating a 50MB response.
var maxResponseBytes int64 = 50 * 1024 * 1024

// apiClient is the shared HTTP client used for all LLM API calls.
// Redirects are never followed: a 307 re-sends the POST body (the prompt) and,
// on the same host, the Authorization header to wherever the server points.
var apiClient = &http.Client{
	Timeout: httpTimeout,
	CheckRedirect: func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	},
}

// maxErrBody caps how much of an API error body is echoed to stderr.
const maxErrBody = 2048

// errBody makes an API error body safe to print: key scrubbed, cut to
// maxErrBody bytes, terminal control characters dropped.
func errBody(b []byte, key string) string {
	s := scrubKey(string(b), key)
	if len(s) > maxErrBody {
		s = strings.ToValidUTF8(s[:maxErrBody], "") + "...(truncated)"
	}
	return shared.StripControls(s)
}

type provider int

const (
	providerAnthropic provider = iota
	providerOpenAI
)

// callResult holds the parsed response from any LLM API call.
type callResult struct {
	text           string
	promptTokens   int
	responseTokens int
}

// promptOutput is the shape emitted by --json.
type promptOutput struct {
	Response       string `json:"response"`
	Model          string `json:"model"`
	PromptTokens   int    `json:"prompt_tokens"`
	ResponseTokens int    `json:"response_tokens"`
	ElapsedMs      int64  `json:"elapsed_ms"`
	SystemPrompt   string `json:"system_prompt,omitempty"`
}

// resolveModel returns what the user explicitly asked for via --model or
// VRK_DEFAULT_MODEL. Returns "" when neither is set — the caller must apply
// a provider-appropriate default once the provider is known.
func resolveModel(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	return os.Getenv("VRK_DEFAULT_MODEL")
}

// defaultModel returns the built-in default model name for a provider.
func defaultModel(prov provider) string {
	if prov == providerOpenAI {
		return "gpt-4o-mini"
	}
	return "claude-sonnet-4-6"
}

// selectProvider picks a provider based on the model name and available keys.
// Returns an error if neither key is set.
func selectProvider(model, anthropicKey, openaiKey string) (provider, string, error) {
	openaiPrefixes := []string{"gpt-", "o1", "o3", "o4"}
	isOpenAI := func(m string) bool {
		for _, p := range openaiPrefixes {
			if strings.HasPrefix(m, p) {
				return true
			}
		}
		return false
	}
	if anthropicKey != "" && openaiKey != "" {
		if isOpenAI(model) {
			return providerOpenAI, openaiKey, nil
		}
		return providerAnthropic, anthropicKey, nil
	}
	if anthropicKey != "" {
		return providerAnthropic, anthropicKey, nil
	}
	if openaiKey != "" {
		return providerOpenAI, openaiKey, nil
	}
	return 0, "", fmt.Errorf("no API key found: set ANTHROPIC_API_KEY or OPENAI_API_KEY")
}

// scrubKey replaces occurrences of key in s with [REDACTED].
// Safe to call with an empty key — returns s unchanged.
func scrubKey(s, key string) string {
	if key == "" {
		return s
	}
	return strings.ReplaceAll(s, key, "[REDACTED]")
}

// resolveSystemPrompt resolves the --system flag value. Empty → error (exit 2).
// @path → read file. Literal text → return as-is.
func resolveSystemPrompt(val string) (string, error) {
	if val == "" {
		return "", fmt.Errorf("prompt: --system value cannot be empty")
	}
	if strings.HasPrefix(val, "@") {
		path := val[1:]
		b, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("prompt: system prompt file not found: %s", path)
			}
			return "", fmt.Errorf("prompt: cannot read system prompt file: %v", err)
		}
		return strings.TrimSuffix(string(b), "\n"), nil
	}
	return val, nil
}

// buildSystemPrompt returns a system prompt string that instructs the model to
// respond with JSON matching the schema.
func buildSystemPrompt(schema map[string]string) string {
	var sb strings.Builder
	sb.WriteString("You must respond with a valid JSON object. ")
	sb.WriteString("The JSON object must have exactly these keys with the specified types:\n")
	for _, k := range sortedKeys(schema) {
		fmt.Fprintf(&sb, "  %q: %s\n", k, schema[k])
	}
	sb.WriteString("Respond with ONLY the JSON object, no other text.")
	return sb.String()
}

// withSchemaInstructions appends the schema instructions to the user's system
// prompt, separated by a blank line. Used for Anthropic and --endpoint, which
// have no native schema enforcement.
func withSchemaInstructions(systemPrompt string, schema map[string]string) string {
	if schema == nil {
		return systemPrompt
	}
	if systemPrompt == "" {
		return buildSystemPrompt(schema)
	}
	return systemPrompt + "\n\n" + buildSystemPrompt(schema)
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// openAIJSONSchemaFor turns the flat {"key":"type"} schema into a JSON Schema
// object for OpenAI response_format. strict is true only when every type is a
// scalar, because OpenAI strict mode needs full nested schemas for arrays and
// objects.
func openAIJSONSchemaFor(schema map[string]string) (s map[string]any, strict bool) {
	props := make(map[string]any, len(schema))
	strict = true
	for k, t := range schema {
		switch t {
		case "array":
			props[k] = map[string]any{"type": "array", "items": map[string]any{}}
			strict = false
		case "object":
			props[k] = map[string]any{"type": "object"}
			strict = false
		default:
			props[k] = map[string]any{"type": t}
		}
	}
	return map[string]any{
		"type":                 "object",
		"properties":           props,
		"required":             sortedKeys(schema),
		"additionalProperties": false,
	}, strict
}

// parseSchema parses schema string (inline JSON or file path) into a map.
func parseSchema(schemaArg string) (map[string]string, error) {
	var raw string
	if strings.HasPrefix(strings.TrimSpace(schemaArg), "{") {
		raw = schemaArg
	} else {
		b, err := os.ReadFile(schemaArg)
		if err != nil {
			return nil, fmt.Errorf("prompt: cannot read schema file: %s: %w", schemaArg, err)
		}
		raw = string(b)
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil, fmt.Errorf("prompt: invalid schema: %w", err)
	}
	return m, nil
}

// validateSchema checks that all keys in schema exist in data with the right type.
// data is expected to be a JSON object string.
func validateSchema(responseText string, schema map[string]string) bool {
	var obj map[string]interface{}
	d := json.NewDecoder(strings.NewReader(responseText))
	d.UseNumber()
	if err := d.Decode(&obj); err != nil {
		return false
	}
	for k, wantType := range schema {
		val, ok := obj[k]
		if !ok {
			return false
		}
		switch wantType {
		case "string":
			if _, ok := val.(string); !ok {
				return false
			}
		case "number":
			if _, ok := val.(json.Number); !ok {
				return false
			}
		case "boolean":
			if _, ok := val.(bool); !ok {
				return false
			}
		case "array":
			if _, ok := val.([]interface{}); !ok {
				return false
			}
		case "object":
			if _, ok := val.(map[string]interface{}); !ok {
				return false
			}
		}
	}
	return true
}

// anthropicRequest is the JSON body sent to the Anthropic messages API.
type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	System    string             `json:"system,omitempty"`
	Messages  []anthropicMessage `json:"messages"`
	Temp      float64            `json:"temperature"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func anthropicBody(model, prompt, systemPrompt string, temp float64) anthropicRequest {
	return anthropicRequest{
		Model:     model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Temp:      temp,
		Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
	}
}

// callAnthropic makes a single call to the Anthropic messages API.
func callAnthropic(model, key, prompt, systemPrompt string, temp float64) (callResult, error) {
	body, err := json.Marshal(anthropicBody(model, prompt, systemPrompt, temp))
	if err != nil {
		return callResult{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return callResult{}, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := apiClient.Do(httpReq)
	if err != nil {
		return callResult{}, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return callResult{}, fmt.Errorf("reading response body: %w", err)
	}
	if int64(len(respBody)) >= maxResponseBytes {
		return callResult{}, fmt.Errorf("API response exceeded %dMB limit", maxResponseBytes/(1024*1024))
	}

	if resp.StatusCode != http.StatusOK {
		safeBody := errBody(respBody, key)
		return callResult{}, fmt.Errorf("API error %d: %s", resp.StatusCode, safeBody)
	}

	var result struct {
		Content []struct {
			Text string `json:"text"`
		} `json:"content"`
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	d := json.NewDecoder(bytes.NewReader(respBody))
	d.UseNumber()
	if err := d.Decode(&result); err != nil {
		return callResult{}, fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Content) == 0 {
		return callResult{}, fmt.Errorf("empty content in response")
	}
	return callResult{
		text:           result.Content[0].Text,
		promptTokens:   result.Usage.InputTokens,
		responseTokens: result.Usage.OutputTokens,
	}, nil
}

// openAIRequest is the JSON body sent to the OpenAI chat completions API.
type openAIRequest struct {
	Model          string          `json:"model"`
	MaxTokens      int             `json:"max_tokens"`
	Temp           float64         `json:"temperature"`
	Messages       []openAIMessage `json:"messages"`
	ResponseFormat *openAIRespFmt  `json:"response_format,omitempty"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIRespFmt struct {
	Type       string            `json:"type"`
	JSONSchema *openAIJSONSchema `json:"json_schema,omitempty"`
}

type openAIJSONSchema struct {
	Strict bool                   `json:"strict"`
	Name   string                 `json:"name"`
	Schema map[string]interface{} `json:"schema"`
}

func chatMessages(prompt, systemPrompt string) []openAIMessage {
	var msgs []openAIMessage
	if systemPrompt != "" {
		msgs = append(msgs, openAIMessage{Role: "system", Content: systemPrompt})
	}
	return append(msgs, openAIMessage{Role: "user", Content: prompt})
}

func openAIBody(model, prompt string, temp float64, schema map[string]string, systemPrompt string) openAIRequest {
	req := openAIRequest{
		Model:     model,
		MaxTokens: 4096,
		Temp:      temp,
		Messages:  chatMessages(prompt, systemPrompt),
	}
	if schema != nil {
		s, strict := openAIJSONSchemaFor(schema)
		req.ResponseFormat = &openAIRespFmt{
			Type:       "json_schema",
			JSONSchema: &openAIJSONSchema{Strict: strict, Name: "output", Schema: s},
		}
	}
	return req
}

// callOpenAI makes a single call to the OpenAI chat completions API.
func callOpenAI(model, key, prompt string, temp float64, schema map[string]string, systemPrompt string) (callResult, error) {
	body, err := json.Marshal(openAIBody(model, prompt, temp, schema, systemPrompt))
	if err != nil {
		return callResult{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return callResult{}, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("content-type", "application/json")

	resp, err := apiClient.Do(httpReq)
	if err != nil {
		return callResult{}, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return callResult{}, fmt.Errorf("reading response body: %w", err)
	}
	if int64(len(respBody)) >= maxResponseBytes {
		return callResult{}, fmt.Errorf("API response exceeded %dMB limit", maxResponseBytes/(1024*1024))
	}

	if resp.StatusCode != http.StatusOK {
		safeBody := errBody(respBody, key)
		return callResult{}, fmt.Errorf("API error %d: %s", resp.StatusCode, safeBody)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	d := json.NewDecoder(bytes.NewReader(respBody))
	d.UseNumber()
	if err := d.Decode(&result); err != nil {
		return callResult{}, fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Choices) == 0 {
		return callResult{}, fmt.Errorf("empty choices in response")
	}
	return callResult{
		text:           result.Choices[0].Message.Content,
		promptTokens:   result.Usage.PromptTokens,
		responseTokens: result.Usage.CompletionTokens,
	}, nil
}

// shellEscapeSingleQuote escapes s for use inside a single-quoted shell string.
// Any embedded single quote is replaced with the sequence '\” which ends the
// current single-quoted segment, adds a backslash-escaped literal quote, and
// reopens the single-quoted segment.
func shellEscapeSingleQuote(s string) string {
	return strings.ReplaceAll(s, "'", `'\''`)
}

// printExplain writes a curl equivalent command to stdout and returns 0.
// The API key is always shown as an env var reference, never its value.
func printExplain(w io.Writer, prov provider, model, prompt string, schema map[string]string, systemPrompt string) int {
	bw := bufio.NewWriter(w)

	switch prov {
	case providerOpenAI:
		bodyJSON, _ := json.Marshal(openAIBody(model, prompt, 0, schema, systemPrompt))
		_, _ = fmt.Fprintf(bw, "curl 'https://api.openai.com/v1/chat/completions' \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"Authorization: Bearer $OPENAI_API_KEY\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"content-type: application/json\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -d '%s'\n", shellEscapeSingleQuote(string(bodyJSON)))

	default: // providerAnthropic; systemPrompt already includes schema instructions.
		bodyJSON, _ := json.Marshal(anthropicBody(model, prompt, systemPrompt, 0))
		_, _ = fmt.Fprintf(bw, "curl 'https://api.anthropic.com/v1/messages' \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"x-api-key: $ANTHROPIC_API_KEY\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"anthropic-version: 2023-06-01\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"content-type: application/json\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -d '%s'\n", shellEscapeSingleQuote(string(bodyJSON)))
	}

	if err := bw.Flush(); err != nil {
		return shared.Errorf("prompt: flushing output: %v", err)
	}
	return 0
}

// resolveEndpoint normalises a raw endpoint string into a full chat completions URL.
// Empty input is a no-op (returns "", nil). Malformed URLs return an error.
// Path rules:
//   - empty or "/" → append /v1/chat/completions
//   - already ends with /chat/completions → use as-is
//   - anything else (e.g. /v1) → append /chat/completions
func resolveEndpoint(raw string) (string, error) {
	if raw == "" {
		return "", nil
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", fmt.Errorf("invalid endpoint URL")
	}
	p := u.Path
	switch {
	case p == "" || p == "/":
		u.Path = "/v1/chat/completions"
	case strings.HasSuffix(p, "/chat/completions"):
		// already correct — use as-is
	default:
		u.Path = strings.TrimRight(p, "/") + "/chat/completions"
	}
	return u.String(), nil
}

// openAICompatRequest is the body sent to any OpenAI-compatible chat completions endpoint.
type openAICompatRequest struct {
	Model          string          `json:"model"`
	MaxTokens      int             `json:"max_tokens"`
	Temp           float64         `json:"temperature"`
	Stream         bool            `json:"stream"`
	Messages       []openAIMessage `json:"messages"`
	ResponseFormat *openAIRespFmt  `json:"response_format,omitempty"`
}

// endpointBody builds the request for an OpenAI-compatible endpoint. The schema
// itself travels in systemPrompt (see withSchemaInstructions); here it only
// switches on json_object mode.
func endpointBody(model, prompt string, temp float64, schema map[string]string, systemPrompt string) openAICompatRequest {
	req := openAICompatRequest{
		Model:     model,
		MaxTokens: 4096,
		Temp:      temp,
		Messages:  chatMessages(prompt, systemPrompt),
	}
	if schema != nil {
		req.ResponseFormat = &openAIRespFmt{Type: "json_object"}
	}
	return req
}

// endpointKey returns the VRK_LLM_KEY to send to endpoint, or "" when it must
// not go there. The key belongs to VRK_LLM_URL: an --endpoint on any other
// origin (scheme, host, port) gets no key, so a caller that can pick the
// endpoint can't collect it. warn says why a set key was held back. Plain http
// to a host that isn't loopback is an error, since the key would cross the
// network in clear.
func endpointKey(endpoint string) (key, warn string, err error) {
	key = os.Getenv("VRK_LLM_KEY")
	if key == "" {
		return "", "", nil
	}
	ep, e1 := url.Parse(endpoint)
	home, e2 := url.Parse(os.Getenv("VRK_LLM_URL"))
	if e1 != nil || e2 != nil || origin(ep) != origin(home) || origin(ep) == "" {
		return "", fmt.Sprintf("prompt: VRK_LLM_KEY not sent to %s (not the VRK_LLM_URL origin)", ep.Redacted()), nil
	}
	if ep.Scheme == "http" && !isLoopback(ep.Hostname()) {
		return "", "", errors.New("prompt: refusing to send VRK_LLM_KEY over plain http to non-loopback host " + ep.Hostname())
	}
	return key, "", nil
}

// origin is scheme://host:port with the default port filled in, or "".
func origin(u *url.URL) string {
	if u == nil || u.Host == "" {
		return ""
	}
	port := u.Port()
	if port == "" {
		port = map[string]string{"http": "80", "https": "443"}[u.Scheme]
	}
	return strings.ToLower(u.Scheme + "://" + u.Hostname() + ":" + port)
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	a, err := netip.ParseAddr(host)
	return err == nil && a.Unmap().IsLoopback()
}

// callOpenAICompatible sends a request to an OpenAI-compatible endpoint and returns
// a callResult with the assistant reply and token counts.
// Auth: sets Authorization: Bearer only if key is non-empty (see endpointKey).
// Never uses OPENAI_API_KEY or ANTHROPIC_API_KEY.
func callOpenAICompatible(endpointURL, key, model, prompt string, temp float64, schema map[string]string, systemPrompt string) (callResult, error) {
	body, err := json.Marshal(endpointBody(model, prompt, temp, schema, systemPrompt))
	if err != nil {
		return callResult{}, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", endpointURL, bytes.NewReader(body))
	if err != nil {
		return callResult{}, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	if key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := apiClient.Do(httpReq)
	if err != nil {
		return callResult{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return callResult{}, fmt.Errorf("reading response body: %w", err)
	}
	if int64(len(respBody)) >= maxResponseBytes {
		return callResult{}, fmt.Errorf("API response exceeded %dMB limit", maxResponseBytes/(1024*1024))
	}

	if resp.StatusCode != http.StatusOK {
		safeBody := errBody(respBody, key)
		return callResult{}, fmt.Errorf("API error %d: %s", resp.StatusCode, safeBody)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	d := json.NewDecoder(bytes.NewReader(respBody))
	d.UseNumber()
	if err := d.Decode(&result); err != nil {
		return callResult{}, fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Choices) == 0 {
		return callResult{}, fmt.Errorf("empty choices in response")
	}
	return callResult{
		text:           result.Choices[0].Message.Content,
		promptTokens:   result.Usage.PromptTokens,
		responseTokens: result.Usage.CompletionTokens,
	}, nil
}

// printExplainEndpoint writes a curl command for a custom endpoint to w.
// The Authorization header line is included only when the key would be sent,
// and is shown as $VRK_LLM_KEY - the actual value is never printed.
func printExplainEndpoint(w io.Writer, endpointURL string, withKey bool, model, prompt string, schema map[string]string, systemPrompt string) int {
	bw := bufio.NewWriter(w)

	bodyJSON, _ := json.Marshal(endpointBody(model, prompt, 0, schema, systemPrompt))

	_, _ = fmt.Fprintf(bw, "curl '%s' \\\n", shellEscapeSingleQuote(endpointURL))
	if withKey {
		_, _ = fmt.Fprintf(bw, "  -H \"Authorization: Bearer $VRK_LLM_KEY\" \\\n")
	}
	_, _ = fmt.Fprintf(bw, "  -H \"content-type: application/json\" \\\n")
	_, _ = fmt.Fprintf(bw, "  -d '%s'\n", shellEscapeSingleQuote(string(bodyJSON)))

	if err := bw.Flush(); err != nil {
		return shared.Errorf("prompt: flushing output: %v", err)
	}
	return 0
}

// dotGet extracts a value from a nested map using a dot-separated path.
// Same pattern as cmd/throttle/throttle.go:dotGet.
func dotGet(obj map[string]any, path string) (any, error) {
	parts := strings.SplitN(path, ".", 2)
	val, ok := obj[parts[0]]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", parts[0])
	}
	if len(parts) == 1 {
		return val, nil
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not an object at: %s", parts[0])
	}
	return dotGet(nested, parts[1])
}

// sender makes one request at the given temperature.
type sender func(prompt string, temp float64) (callResult, error)

// resolveEndpointKey wraps endpointKey for Run and runFieldMode: prints the
// warning (stderr, not under --json) and turns the error into exit 2.
func resolveEndpointKey(endpoint string, jsonFlag bool) (string, int) {
	key, warn, err := endpointKey(endpoint)
	if err != nil {
		if jsonFlag {
			return "", shared.PrintJSONError(map[string]any{"error": err.Error(), "code": 2})
		}
		return "", shared.UsageErrorf("%s", err.Error())
	}
	if warn != "" && !jsonFlag {
		shared.Warn("%s", warn)
	}
	return key, 0
}

// newSender decides how a request is built and sent. Validation, retry,
// --fail, and output are the same for every provider (see callWithSchema).
// Anthropic and --endpoint get the schema as system prompt instructions;
// OpenAI gets it as response_format.
func newSender(endpoint string, prov provider, model, apiKey, systemPrompt string, schema map[string]string) sender {
	switch {
	case endpoint != "":
		sys := withSchemaInstructions(systemPrompt, schema)
		return func(p string, t float64) (callResult, error) {
			return callOpenAICompatible(endpoint, apiKey, model, p, t, schema, sys)
		}
	case prov == providerOpenAI:
		return func(p string, t float64) (callResult, error) {
			return callOpenAI(model, apiKey, p, t, schema, systemPrompt)
		}
	default:
		sys := withSchemaInstructions(systemPrompt, schema)
		return func(p string, t float64) (callResult, error) {
			return callAnthropic(model, apiKey, p, sys, t)
		}
	}
}

// callWithSchema sends prompt and, when a schema is set, validates the reply
// and retries with rising temperature (0, 0.1, 0.2, ... capped at 1). A final
// mismatch is an error only with --fail or --retry. Without them it warns and
// returns the response, so the caller prints it and exits 0.
func callWithSchema(send sender, prompt string, schema map[string]string, retryCount int, failFlag bool, linePrefix string) (callResult, error) {
	for attempt := 0; ; attempt++ {
		cr, err := send(prompt, min(float64(attempt)/10, 1))
		if err != nil || schema == nil || validateSchema(cr.text, schema) {
			return cr, err
		}
		if attempt < retryCount {
			fmt.Fprintf(os.Stderr, "prompt: %sattempt %d failed schema validation, retrying...\n", linePrefix, attempt+1)
			continue
		}
		if !failFlag && retryCount <= 0 {
			fmt.Fprintf(os.Stderr, "prompt: %swarning: response does not match schema\n", linePrefix)
			return cr, nil
		}
		if attempt == 0 {
			return cr, errors.New("response does not match schema")
		}
		return cr, fmt.Errorf("response does not match schema after %d attempts", attempt+1)
	}
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "prompt",
		Short: "Send a prompt to an LLM and print the response",
		Flags: []shared.FlagMeta{
			{Name: "model", Shorthand: "m", Usage: "LLM model (default: claude-sonnet-4-6 or VRK_DEFAULT_MODEL)"},
			{Name: "field", Usage: "dot-path field in each JSONL line to use as prompt text"},
			{Name: "budget", Usage: "exit 1 if prompt exceeds N tokens (0 = disabled)"},
			{Name: "fail", Shorthand: "f", Usage: "exit 1 on schema mismatch (API errors always exit 1)"},
			{Name: "json", Shorthand: "j", Usage: "emit response as JSON envelope with metadata"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
			{Name: "schema", Shorthand: "s", Usage: "JSON schema string or file path for response validation"},
			{Name: "explain", Usage: "print equivalent curl command and exit, no API call"},
			{Name: "retry", Usage: "retry N times on schema mismatch with temperature escalation"},
			{Name: "endpoint", Usage: "OpenAI-compatible API base URL (e.g. http://localhost:11434/v1)"},
			{Name: "system", Usage: "system prompt text, or @path to read from file"},
		},
	})
}

// Run is the entry point for vrk prompt. Returns 0 (success), 1 (runtime
// error), or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("prompt", pflag.ContinueOnError)

	var (
		modelFlag   string
		fieldFlag   string
		budget      int
		failFlag    bool
		jsonFlag    bool
		quietFlag   bool
		schemaArg   string
		explainFlag bool
		retryCount  int
		endpoint    string
		systemArg   string
	)

	fs.StringVarP(&modelFlag, "model", "m", "", "LLM model (default: claude-sonnet-4-6 or VRK_DEFAULT_MODEL)")
	fs.StringVar(&fieldFlag, "field", "", "dot-path field in each JSONL line to use as prompt text")
	fs.IntVar(&budget, "budget", 0, "exit 1 if prompt exceeds N tokens (0 = disabled)")
	fs.BoolVarP(&failFlag, "fail", "f", false, "exit 1 on schema mismatch (API errors always exit 1)")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit response as JSON envelope with metadata")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")
	fs.StringVarP(&schemaArg, "schema", "s", "", "JSON schema string or file path for response validation")
	fs.BoolVar(&explainFlag, "explain", false, "print equivalent curl command and exit, no API call")
	fs.IntVar(&retryCount, "retry", 0, "retry N times on schema mismatch with temperature escalation")
	fs.StringVar(&endpoint, "endpoint", "", "OpenAI-compatible API base URL (e.g. http://localhost:11434/v1)")
	fs.StringVar(&systemArg, "system", "", "system prompt text, or @path to read from file")

	// Suppress pflag's auto-printing so all output goes through shared helpers.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		// jsonFlag is not yet populated — pre-scan raw args so flag-parse errors
		// still route to stdout as JSON when --json/-j was requested.
		for _, a := range os.Args[1:] {
			if a == "--json" || a == "-j" {
				return shared.PrintJSONError(map[string]any{"error": err.Error(), "code": 2})
			}
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	// VRK_LLM_URL is the env-var alternative to --endpoint; flag takes precedence.
	if endpoint == "" {
		endpoint = os.Getenv("VRK_LLM_URL")
	}

	// Validate and normalise the endpoint URL before doing anything else.
	resolvedEndpoint, err := resolveEndpoint(endpoint)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "invalid endpoint URL", "code": 2})
		}
		return shared.UsageErrorf("invalid endpoint URL")
	}
	endpoint = resolvedEndpoint

	// Resolve --system flag before anything else that might use it.
	var systemPrompt string
	systemSet := fs.Changed("system")
	if systemSet {
		var sysErr error
		systemPrompt, sysErr = resolveSystemPrompt(systemArg)
		if sysErr != nil {
			msg := sysErr.Error()
			// Empty value is a usage error (exit 2); file errors are runtime (exit 1).
			if strings.Contains(msg, "cannot be empty") {
				if jsonFlag {
					return shared.PrintJSONError(map[string]any{"error": msg, "code": 2})
				}
				return shared.UsageErrorf("%s", msg)
			}
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{"error": msg, "code": 1})
			}
			return shared.Errorf("%s", msg)
		}
	}

	model := resolveModel(modelFlag)

	// --field + --explain mutual exclusion.
	if fieldFlag != "" && explainFlag {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "prompt: --field and --explain are mutually exclusive",
				"code":  2,
			})
		}
		return shared.UsageErrorf("prompt: --field and --explain are mutually exclusive")
	}

	// --field TTY guard: streaming mode requires piped input.
	if fieldFlag != "" && isTerminal(int(os.Stdin.Fd())) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "prompt: --field requires piped input",
				"code":  2,
			})
		}
		return shared.UsageErrorf("prompt: --field requires piped input")
	}

	// TTY detection: if stdin is a terminal and no positional arg and no --explain,
	// there is no input - that is a usage error.
	args := fs.Args()
	if isTerminal(int(os.Stdin.Fd())) && len(args) == 0 && !explainFlag {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "prompt: no input: pipe text to stdin or pass as argument", "code": 2})
		}
		return shared.UsageErrorf("prompt: no input: pipe text to stdin or pass as argument")
	}

	// Parse schema if provided.
	var schema map[string]string
	if schemaArg != "" {
		var err error
		schema, err = parseSchema(schemaArg)
		if err != nil {
			return shared.Errorf("%v", err)
		}
	}

	// --field mode: JSONL streaming. Handles everything and returns.
	if fieldFlag != "" {
		return runFieldMode(fieldFlag, model, endpoint, budget, jsonFlag, quietFlag, failFlag, schema, retryCount, systemPrompt)
	}

	// Read input: positional arg wins over stdin.
	// For --explain with no positional arg and TTY stdin, use empty prompt.
	var promptText string
	if len(args) > 0 {
		promptText = strings.Join(args, " ")
	} else if !isTerminal(int(os.Stdin.Fd())) {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return shared.Errorf("prompt: reading stdin: %v", err)
		}
		promptText = strings.TrimSuffix(string(b), "\n")
	}
	// If --explain and no prompt, use empty string - that is fine.

	// D8: an empty prompt is a mistake upstream (a failed step, an unset
	// variable). Never send it.
	if strings.TrimSpace(promptText) == "" && !explainFlag {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "prompt: empty prompt, nothing sent", "code": 1})
		}
		return shared.Errorf("prompt: empty prompt, nothing sent")
	}

	// Budget check: count tokens and gate before any API call or key check.
	if budget > 0 {
		count, err := tokcount.CountTokens(promptText)
		if err != nil {
			return shared.Errorf("prompt: counting tokens: %v", err)
		}
		if count > budget {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("prompt: %d tokens exceeds budget of %d", count, budget),
					"code":  1,
				})
			}
			return shared.Errorf("prompt: %d tokens exceeds budget of %d", count, budget)
		}
	}

	var prov provider
	var apiKey string
	if endpoint != "" {
		// Custom endpoint: always OpenAI chat completions format, no provider detection.
		var keyCode int
		if apiKey, keyCode = resolveEndpointKey(endpoint, jsonFlag); keyCode != 0 {
			return keyCode
		}
		if explainFlag {
			return printExplainEndpoint(os.Stdout, endpoint, apiKey != "", model, promptText, schema, withSchemaInstructions(systemPrompt, schema))
		}
		// Local model names cannot be guessed - require an explicit --model.
		if model == "" {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{"error": "prompt: --model is required when using --endpoint", "code": 2})
			}
			return shared.UsageErrorf("prompt: --model is required when using --endpoint")
		}
	} else {
		// Read API keys so we can select provider.
		anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
		openaiKey := os.Getenv("OPENAI_API_KEY")

		// Determine provider. For --explain with no keys, default to Anthropic
		// (user still gets a useful curl template).
		if !explainFlag {
			var err error
			prov, apiKey, err = selectProvider(model, anthropicKey, openaiKey)
			if err != nil {
				if jsonFlag {
					return shared.PrintJSONError(map[string]any{
						"error": fmt.Sprintf("prompt: %v", err),
						"code":  1,
					})
				}
				return shared.Errorf("prompt: %v", err)
			}
		} else if openaiKey != "" && anthropicKey == "" {
			prov = providerOpenAI
		}

		// Apply provider-appropriate default when no explicit model was given.
		if model == "" {
			model = defaultModel(prov)
		}

		// --explain: print curl equivalent and exit 0.
		if explainFlag {
			explainSystem := systemPrompt
			if prov == providerAnthropic {
				explainSystem = withSchemaInstructions(systemPrompt, schema)
			}
			return printExplain(os.Stdout, prov, model, promptText, schema, explainSystem)
		}
	}

	startTime := time.Now()
	send := newSender(endpoint, prov, model, apiKey, systemPrompt, schema)
	cr, callErr := callWithSchema(send, promptText, schema, retryCount, failFlag, "")
	if callErr != nil {
		safeErr := scrubKey(callErr.Error(), apiKey)
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("prompt: %s", safeErr),
				"code":  1,
			})
		}
		return shared.Errorf("prompt: %s", safeErr)
	}

	elapsedMs := time.Since(startTime).Milliseconds()

	// Output.
	bw := bufio.NewWriter(os.Stdout)

	if jsonFlag {
		out := promptOutput{
			Response:       cr.text,
			Model:          model,
			PromptTokens:   cr.promptTokens,
			ResponseTokens: cr.responseTokens,
			ElapsedMs:      elapsedMs,
		}
		// A prompt read from @path is not echoed: that would turn --json into
		// a file reader for whoever controls the flags.
		if systemSet && !strings.HasPrefix(systemArg, "@") {
			out.SystemPrompt = systemPrompt
		}
		enc := json.NewEncoder(bw)
		if err := enc.Encode(out); err != nil {
			_ = bw.Flush()
			return shared.Errorf("prompt: encoding JSON output: %v", err)
		}
	} else {
		if _, err := fmt.Fprint(bw, cr.text); err != nil {
			_ = bw.Flush()
			return shared.Errorf("prompt: writing output: %v", err)
		}
		if len(cr.text) == 0 || cr.text[len(cr.text)-1] != '\n' {
			_, _ = fmt.Fprintln(bw)
		}
	}

	if err := bw.Flush(); err != nil {
		return shared.Errorf("prompt: flushing output: %v", err)
	}
	return 0
}

// runFieldMode processes JSONL stdin line by line, extracting the named field
// from each record as prompt text and making one API call per line.
func runFieldMode(fieldPath, model, endpoint string, budget int, jsonFlag, quietFlag, failFlag bool, schema map[string]string, retryCount int, systemPrompt string) int {
	// Set up scanner early so we can exit 0 on empty stdin before provider resolution.
	scanner := shared.ScanLines(os.Stdin)

	// Peek: if there's no first line, exit 0 immediately (empty stdin).
	if !scanner.Scan() {
		if err := scanner.Err(); err != nil {
			return shared.Errorf("prompt: reading stdin: %v", err)
		}
		return 0
	}
	firstLine := scanner.Text()

	// Resolve provider once before the loop.
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	var prov provider
	var apiKey string

	if endpoint == "" {
		var err error
		prov, apiKey, err = selectProvider(model, anthropicKey, openaiKey)
		if err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("prompt: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("prompt: %v", err)
		}
		if model == "" {
			model = defaultModel(prov)
		}
	} else {
		// Endpoint path requires explicit model.
		if model == "" {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": "prompt: --model is required when using --endpoint",
					"code":  2,
				})
			}
			return shared.UsageErrorf("prompt: --model is required when using --endpoint")
		}
		var keyCode int
		if apiKey, keyCode = resolveEndpointKey(endpoint, jsonFlag); keyCode != 0 {
			return keyCode
		}
	}

	send := newSender(endpoint, prov, model, apiKey, systemPrompt, schema)

	bw := bufio.NewWriter(os.Stdout)

	// Process firstLine (already scanned above), then continue with remaining lines.
	lineNum := 0
	line := firstLine
	for {
		lineNum++

		// Parse JSONL record.
		var record map[string]any
		d := json.NewDecoder(strings.NewReader(line))
		d.UseNumber()
		if err := d.Decode(&record); err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("prompt: line %d: invalid JSON", lineNum),
					"code":  1,
				})
			}
			return shared.Errorf("prompt: line %d: invalid JSON", lineNum)
		}

		// Extract field value.
		val, err := dotGet(record, fieldPath)
		if err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("prompt: line %d: field %q not found", lineNum, fieldPath),
					"code":  1,
				})
			}
			return shared.Errorf("prompt: line %d: field %q not found", lineNum, fieldPath)
		}
		promptText, ok := val.(string)
		if !ok {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("prompt: line %d: field %q is not a string", lineNum, fieldPath),
					"code":  1,
				})
			}
			return shared.Errorf("prompt: line %d: field %q is not a string", lineNum, fieldPath)
		}

		// D8: never send an empty prompt. Skip the record and keep going.
		if strings.TrimSpace(promptText) == "" {
			fmt.Fprintf(os.Stderr, "prompt: line %d: empty prompt, skipped\n", lineNum)
			if !scanner.Scan() {
				break
			}
			line = scanner.Text()
			continue
		}

		// Per-record budget check.
		if budget > 0 {
			count, err := tokcount.CountTokens(promptText)
			if err != nil {
				return shared.Errorf("prompt: line %d: counting tokens: %v", lineNum, err)
			}
			if count > budget {
				if jsonFlag {
					return shared.PrintJSONError(map[string]any{
						"error": fmt.Sprintf("prompt: line %d: %d tokens exceeds budget %d", lineNum, count, budget),
						"code":  1,
					})
				}
				return shared.Errorf("prompt: line %d: %d tokens exceeds budget %d", lineNum, count, budget)
			}
		}

		start := time.Now()
		cr, callErr := callWithSchema(send, promptText, schema, retryCount, failFlag, fmt.Sprintf("line %d: ", lineNum))
		if callErr != nil {
			safeErr := scrubKey(callErr.Error(), apiKey)
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("prompt: line %d: %s", lineNum, safeErr),
					"code":  1,
				})
			}
			return shared.Errorf("prompt: line %d: %s", lineNum, safeErr)
		}

		elapsedMs := time.Since(start).Milliseconds()

		// Output.
		if jsonFlag {
			// Warn if input record has a "response" field that will be overwritten.
			if _, exists := record["response"]; exists && !quietFlag {
				fmt.Fprintf(os.Stderr, "prompt: line %d: warning: input field \"response\" will be overwritten\n", lineNum)
			}

			// Merge: start from input record, add response fields.
			record["response"] = cr.text
			record["model"] = model
			record["prompt_tokens"] = cr.promptTokens
			record["response_tokens"] = cr.responseTokens
			record["elapsed_ms"] = elapsedMs

			enc := json.NewEncoder(bw)
			if err := enc.Encode(record); err != nil {
				_ = bw.Flush()
				return shared.Errorf("prompt: line %d: encoding JSON: %v", lineNum, err)
			}
		} else {
			// Escape newlines so each response stays on one output line.
			escaped := strings.ReplaceAll(cr.text, "\n", `\n`)
			if _, err := fmt.Fprintln(bw, escaped); err != nil {
				_ = bw.Flush()
				return shared.Errorf("prompt: line %d: writing output: %v", lineNum, err)
			}
		}

		if err := bw.Flush(); err != nil {
			return shared.Errorf("prompt: flushing output: %v", err)
		}

		// Advance to next line, or break if done.
		if !scanner.Scan() {
			break
		}
		line = scanner.Text()
	}

	if err := scanner.Err(); err != nil {
		return shared.Errorf("prompt: reading stdin: %v", err)
	}

	return 0
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("prompt", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringP("model", "m", "", "LLM model (default: claude-sonnet-4-6 or VRK_DEFAULT_MODEL)")
	fs.String("field", "", "dot-path field in each JSONL line to use as prompt text")
	fs.Int("budget", 0, "exit 1 if prompt exceeds N tokens (0 = disabled)")
	fs.BoolP("fail", "f", false, "exit 1 on schema mismatch (API errors always exit 1)")
	fs.BoolP("json", "j", false, "emit response as JSON envelope with metadata")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	fs.StringP("schema", "s", "", "JSON schema string or file path for response validation")
	fs.Bool("explain", false, "print equivalent curl command and exit, no API call")
	fs.Int("retry", 0, "retry N times on schema mismatch with temperature escalation")
	fs.String("endpoint", "", "OpenAI-compatible API base URL (e.g. http://localhost:11434/v1)")
	fs.String("system", "", "system prompt text, or @path to read from file")
	return fs
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	bw := bufio.NewWriter(os.Stdout)
	lines := []string{
		"usage: prompt [flags] [text]",
		"       echo <text> | prompt [flags]",
		"       cat docs.jsonl | prompt --field text [flags]",
		"",
		"Send a prompt to an LLM and print the response. Reads from stdin or",
		"a positional argument. Defaults to claude-sonnet-4-6 via Anthropic.",
		"",
		"With --field, reads JSONL from stdin line by line and makes one API",
		"call per record using the named field as the prompt text.",
		"",
		"  --system <text>      system prompt as literal text",
		"  --system @file.txt   system prompt read from file",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(bw, l); err != nil {
			_ = bw.Flush()
			return shared.Errorf("prompt: %v", err)
		}
	}
	fs.SetOutput(bw)
	fs.PrintDefaults()
	if err := bw.Flush(); err != nil {
		return shared.Errorf("prompt: flushing output: %v", err)
	}
	return 0
}
