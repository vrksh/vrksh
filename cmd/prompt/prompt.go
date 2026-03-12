package prompt

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/tokcount"
	"golang.org/x/term"
)

// stdinIsTerminal is a package-level variable so tests can override it to
// simulate interactive terminal detection without needing a real TTY.
var stdinIsTerminal = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd()))
}

type provider int

const (
	providerAnthropic provider = iota
	providerOpenAI
)

// promptOutput is the shape emitted by --json.
type promptOutput struct {
	Response    string `json:"response"`
	Model       string `json:"model"`
	TokensUsed  int    `json:"tokens_used"`
	LatencyMs   int64  `json:"latency_ms"`
	RequestHash string `json:"request_hash"`
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
	return "claude-sonnet-4-5"
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

// requestHash returns a stable SHA-256 hex string for the given inputs.
func requestHash(model string, temp float64, prompt string) string {
	input := model + "|" + strconv.FormatFloat(temp, 'f', 1, 64) + "|" + prompt
	h := sha256.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)
}

// buildSystemPrompt returns a system prompt string that instructs the model to
// respond with JSON matching the schema.
func buildSystemPrompt(schema map[string]string) string {
	var sb strings.Builder
	sb.WriteString("You must respond with a valid JSON object. ")
	sb.WriteString("The JSON object must have exactly these keys with the specified types:\n")
	for k, v := range schema {
		fmt.Fprintf(&sb, "  %q: %s\n", k, v)
	}
	sb.WriteString("Respond with ONLY the JSON object, no other text.")
	return sb.String()
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

// callAnthropic makes a single call to the Anthropic messages API.
func callAnthropic(model, key, prompt, systemPrompt string, temp float64) (string, int, error) {
	req := anthropicRequest{
		Model:     model,
		MaxTokens: 4096,
		Temp:      temp,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}
	if systemPrompt != "" {
		req.System = systemPrompt
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", 0, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("x-api-key", key)
	httpReq.Header.Set("anthropic-version", "2023-06-01")
	httpReq.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		// Scrub key from response body before including in error.
		safeBody := scrubKey(string(respBody), key)
		return "", 0, fmt.Errorf("API error %d: %s", resp.StatusCode, safeBody)
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
		return "", 0, fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Content) == 0 {
		return "", 0, fmt.Errorf("empty content in response")
	}
	tokensUsed := result.Usage.InputTokens + result.Usage.OutputTokens
	return result.Content[0].Text, tokensUsed, nil
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

// callOpenAI makes a single call to the OpenAI chat completions API.
func callOpenAI(model, key, prompt string, temp float64, schema map[string]string) (string, int, error) {
	req := openAIRequest{
		Model:     model,
		MaxTokens: 4096,
		Temp:      temp,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
	}
	if schema != nil {
		schemaObj := make(map[string]interface{}, len(schema))
		for k, v := range schema {
			schemaObj[k] = v
		}
		req.ResponseFormat = &openAIRespFmt{
			Type: "json_schema",
			JSONSchema: &openAIJSONSchema{
				Strict: true,
				Name:   "output",
				Schema: schemaObj,
			},
		}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", 0, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("Authorization", "Bearer "+key)
	httpReq.Header.Set("content-type", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		safeBody := scrubKey(string(respBody), key)
		return "", 0, fmt.Errorf("API error %d: %s", resp.StatusCode, safeBody)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	d := json.NewDecoder(bytes.NewReader(respBody))
	d.UseNumber()
	if err := d.Decode(&result); err != nil {
		return "", 0, fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", 0, fmt.Errorf("empty choices in response")
	}
	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

// printExplain writes a curl equivalent command to stdout and returns 0.
// The API key is always shown as an env var reference, never its value.
func printExplain(w io.Writer, prov provider, model, prompt string, schema map[string]string) int {
	bw := bufio.NewWriter(w)

	switch prov {
	case providerOpenAI:
		body := map[string]interface{}{
			"model":       model,
			"max_tokens":  4096,
			"temperature": 0,
			"messages": []map[string]string{
				{"role": "user", "content": prompt},
			},
		}
		if schema != nil {
			schemaObj := make(map[string]interface{}, len(schema))
			for k, v := range schema {
				schemaObj[k] = v
			}
			body["response_format"] = map[string]interface{}{
				"type": "json_schema",
				"json_schema": map[string]interface{}{
					"strict": true,
					"name":   "output",
					"schema": schemaObj,
				},
			}
		}
		bodyJSON, _ := json.Marshal(body)
		_, _ = fmt.Fprintf(bw, "curl https://api.openai.com/v1/chat/completions \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"Authorization: Bearer $OPENAI_API_KEY\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"content-type: application/json\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -d '%s'\n", string(bodyJSON))

	default: // providerAnthropic
		type msgBody struct {
			Model     string             `json:"model"`
			MaxTokens int                `json:"max_tokens"`
			System    string             `json:"system,omitempty"`
			Messages  []anthropicMessage `json:"messages"`
		}
		mb := msgBody{
			Model:     model,
			MaxTokens: 4096,
			Messages:  []anthropicMessage{{Role: "user", Content: prompt}},
		}
		if schema != nil {
			mb.System = buildSystemPrompt(schema)
		}
		bodyJSON, _ := json.Marshal(mb)
		_, _ = fmt.Fprintf(bw, "curl https://api.anthropic.com/v1/messages \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"x-api-key: $ANTHROPIC_API_KEY\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"anthropic-version: 2023-06-01\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -H \"content-type: application/json\" \\\n")
		_, _ = fmt.Fprintf(bw, "  -d '%s'\n", string(bodyJSON))
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
	if err != nil || u.Scheme == "" {
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

// callOpenAICompatible sends a request to an OpenAI-compatible endpoint and returns
// the assistant reply text, total token count, and any error.
// Auth: reads VRK_LLM_KEY and sets Authorization: Bearer only if non-empty.
// Never uses OPENAI_API_KEY or ANTHROPIC_API_KEY.
func callOpenAICompatible(endpointURL, model, prompt string, schema map[string]string) (string, int, error) {
	req := openAICompatRequest{
		Model:     model,
		MaxTokens: 4096,
		Temp:      0,
		Stream:    false,
		Messages:  []openAIMessage{{Role: "user", Content: prompt}},
	}
	if schema != nil {
		req.ResponseFormat = &openAIRespFmt{Type: "json_object"}
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", 0, fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", endpointURL, bytes.NewReader(body))
	if err != nil {
		return "", 0, fmt.Errorf("creating request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	if key := os.Getenv("VRK_LLM_KEY"); key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+key)
	}

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return "", 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", 0, fmt.Errorf("reading response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		safeBody := scrubKey(string(respBody), os.Getenv("VRK_LLM_KEY"))
		return "", 0, fmt.Errorf("API error %d: %s", resp.StatusCode, safeBody)
	}

	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			TotalTokens int `json:"total_tokens"`
		} `json:"usage"`
	}
	d := json.NewDecoder(bytes.NewReader(respBody))
	d.UseNumber()
	if err := d.Decode(&result); err != nil {
		return "", 0, fmt.Errorf("decoding response: %w", err)
	}
	if len(result.Choices) == 0 {
		return "", 0, fmt.Errorf("empty choices in response")
	}
	return result.Choices[0].Message.Content, result.Usage.TotalTokens, nil
}

// printExplainEndpoint writes a curl command for a custom endpoint to w.
// The Authorization header line is included only when VRK_LLM_KEY is set,
// and is shown as $VRK_LLM_KEY — the actual value is never printed.
func printExplainEndpoint(w io.Writer, endpointURL, model, prompt string, schema map[string]string) int {
	bw := bufio.NewWriter(w)

	body := map[string]interface{}{
		"model":       model,
		"max_tokens":  4096,
		"temperature": 0,
		"stream":      false,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}
	if schema != nil {
		body["response_format"] = map[string]string{"type": "json_object"}
	}
	bodyJSON, _ := json.Marshal(body)

	_, _ = fmt.Fprintf(bw, "curl %s \\\n", endpointURL)
	if os.Getenv("VRK_LLM_KEY") != "" {
		_, _ = fmt.Fprintf(bw, "  -H \"Authorization: Bearer $VRK_LLM_KEY\" \\\n")
	}
	_, _ = fmt.Fprintf(bw, "  -H \"content-type: application/json\" \\\n")
	_, _ = fmt.Fprintf(bw, "  -d '%s'\n", string(bodyJSON))

	if err := bw.Flush(); err != nil {
		return shared.Errorf("prompt: flushing output: %v", err)
	}
	return 0
}

// Run is the entry point for vrk prompt. Returns 0 (success), 1 (runtime
// error), or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("prompt", pflag.ContinueOnError)

	var (
		modelFlag   string
		budget      int
		failFlag    bool
		jsonFlag    bool
		schemaArg   string
		explainFlag bool
		retryCount  int
		endpoint    string
	)

	fs.StringVarP(&modelFlag, "model", "m", "", "LLM model (default: claude-sonnet-4-5 or VRK_DEFAULT_MODEL)")
	fs.IntVar(&budget, "budget", 0, "exit 1 if prompt exceeds N tokens (0 = disabled)")
	fs.BoolVarP(&failFlag, "fail", "f", false, "fail on non-2xx API response or schema mismatch")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit response as JSON envelope with metadata")
	fs.StringVarP(&schemaArg, "schema", "s", "", "JSON schema string or file path for response validation")
	fs.BoolVar(&explainFlag, "explain", false, "print equivalent curl command and exit, no API call")
	fs.IntVar(&retryCount, "retry", 0, "retry N times on schema mismatch with temperature escalation")
	fs.StringVar(&endpoint, "endpoint", "", "OpenAI-compatible API base URL (e.g. http://localhost:11434/v1)")

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

	model := resolveModel(modelFlag)

	// TTY detection: if stdin is a terminal and no positional arg and no --explain,
	// there is no input — that is a usage error.
	args := fs.Args()
	if stdinIsTerminal() && len(args) == 0 && !explainFlag {
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

	// Read input: positional arg wins over stdin.
	// For --explain with no positional arg and TTY stdin, use empty prompt.
	var promptText string
	if len(args) > 0 {
		promptText = strings.Join(args, " ")
	} else if !stdinIsTerminal() {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return shared.Errorf("prompt: reading stdin: %v", err)
		}
		promptText = strings.TrimSuffix(string(b), "\n")
	}
	// If --explain and no prompt, use empty string — that is fine.

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

	// Custom endpoint path — bypasses all provider detection.
	// --endpoint (or VRK_LLM_URL) always uses OpenAI chat completions format.
	if endpoint != "" {
		if explainFlag {
			return printExplainEndpoint(os.Stdout, endpoint, model, promptText, schema)
		}
		// Local model names cannot be guessed — require an explicit --model.
		if model == "" {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{"error": "prompt: --model is required when using --endpoint", "code": 2})
			}
			return shared.UsageErrorf("prompt: --model is required when using --endpoint")
		}

		start := time.Now()
		responseText, tokensUsed, callErr := callOpenAICompatible(endpoint, model, promptText, schema)
		if callErr != nil {
			safeErr := scrubKey(callErr.Error(), os.Getenv("VRK_LLM_KEY"))
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("prompt: %s", safeErr),
					"code":  1,
				})
			}
			return shared.Errorf("prompt: %s", safeErr)
		}
		latencyMs := time.Since(start).Milliseconds()

		bw := bufio.NewWriter(os.Stdout)
		if jsonFlag {
			out := promptOutput{
				Response:    responseText,
				Model:       model,
				TokensUsed:  tokensUsed,
				LatencyMs:   latencyMs,
				RequestHash: requestHash(model, 0, promptText),
			}
			enc := json.NewEncoder(bw)
			if encErr := enc.Encode(out); encErr != nil {
				_ = bw.Flush()
				return shared.Errorf("prompt: encoding JSON output: %v", encErr)
			}
		} else {
			if _, writeErr := fmt.Fprint(bw, responseText); writeErr != nil {
				_ = bw.Flush()
				return shared.Errorf("prompt: writing output: %v", writeErr)
			}
			if len(responseText) == 0 || responseText[len(responseText)-1] != '\n' {
				_, _ = fmt.Fprintln(bw)
			}
		}
		if flushErr := bw.Flush(); flushErr != nil {
			return shared.Errorf("prompt: flushing output: %v", flushErr)
		}
		return 0
	}

	// Read API keys so we can select provider.
	anthropicKey := os.Getenv("ANTHROPIC_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")

	// Determine provider. For --explain with no keys, default to Anthropic
	// (user still gets a useful curl template).
	var prov provider
	var apiKey string
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
	} else {
		// For --explain we pick a provider for display purposes only.
		if openaiKey != "" && anthropicKey == "" {
			prov = providerOpenAI
		} else {
			prov = providerAnthropic
		}
	}

	// Apply provider-appropriate default when no explicit model was given.
	if model == "" {
		model = defaultModel(prov)
	}

	// --explain: print curl equivalent and exit 0.
	if explainFlag {
		bw := bufio.NewWriter(os.Stdout)
		code := printExplain(bw, prov, model, promptText, schema)
		_ = bw.Flush()
		return code
	}

	// Temperature escalation for retries.
	temps := []float64{0.0, 0.1, 0.2}

	var (
		responseText string
		tokensUsed   int
		finalTemp    float64
		startTime    = time.Now()
	)

	for attempt := 0; attempt <= retryCount; attempt++ {
		temp := 0.0
		if attempt < len(temps) {
			temp = temps[attempt]
		} else {
			temp = temps[len(temps)-1]
		}
		finalTemp = temp

		var systemPrompt string
		if schema != nil && prov == providerAnthropic {
			systemPrompt = buildSystemPrompt(schema)
		}

		var callErr error
		switch prov {
		case providerAnthropic:
			responseText, tokensUsed, callErr = callAnthropic(model, apiKey, promptText, systemPrompt, temp)
		case providerOpenAI:
			var schemaForOpenAI map[string]string
			if schema != nil {
				schemaForOpenAI = schema
			}
			responseText, tokensUsed, callErr = callOpenAI(model, apiKey, promptText, temp, schemaForOpenAI)
		}

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

		// Schema validation (Anthropic only — OpenAI uses response_format).
		if schema != nil && prov == providerAnthropic {
			if validateSchema(responseText, schema) {
				break
			}
			if attempt < retryCount {
				fmt.Fprintf(os.Stderr, "prompt: attempt %d failed schema validation, retrying...\n", attempt+1)
				continue
			}
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("prompt: response does not match schema after %d attempts", retryCount+1),
					"code":  1,
				})
			}
			return shared.Errorf("prompt: response does not match schema after %d attempts", retryCount+1)
		}

		break
	}

	latencyMs := time.Since(startTime).Milliseconds()

	// Output.
	bw := bufio.NewWriter(os.Stdout)

	if jsonFlag {
		out := promptOutput{
			Response:    responseText,
			Model:       model,
			TokensUsed:  tokensUsed,
			LatencyMs:   latencyMs,
			RequestHash: requestHash(model, finalTemp, promptText),
		}
		enc := json.NewEncoder(bw)
		if err := enc.Encode(out); err != nil {
			_ = bw.Flush()
			return shared.Errorf("prompt: encoding JSON output: %v", err)
		}
	} else {
		if _, err := fmt.Fprint(bw, responseText); err != nil {
			_ = bw.Flush()
			return shared.Errorf("prompt: writing output: %v", err)
		}
		// Ensure output ends with a newline.
		if len(responseText) == 0 || responseText[len(responseText)-1] != '\n' {
			_, _ = fmt.Fprintln(bw)
		}
	}

	if err := bw.Flush(); err != nil {
		return shared.Errorf("prompt: flushing output: %v", err)
	}
	return 0
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	bw := bufio.NewWriter(os.Stdout)
	lines := []string{
		"usage: prompt [flags] [text]",
		"       echo <text> | prompt [flags]",
		"",
		"Send a prompt to an LLM and print the response. Reads from stdin or",
		"a positional argument. Defaults to claude-sonnet-4-5 via Anthropic.",
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
