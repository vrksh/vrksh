package prompt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// recordingServer is an OpenAI-compatible stub. It replies with contents in
// order (repeating the last one) and keeps every request body it received.
type recordingServer struct {
	*httptest.Server
	mu     sync.Mutex
	bodies []map[string]any
}

func newRecordingServer(t *testing.T, contents ...string) *recordingServer {
	t.Helper()
	rs := &recordingServer{}
	rs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("stub: bad request body: %v", err)
		}
		rs.mu.Lock()
		rs.bodies = append(rs.bodies, body)
		idx := len(rs.bodies) - 1
		rs.mu.Unlock()
		if idx >= len(contents) {
			idx = len(contents) - 1
		}
		c, _ := json.Marshal(contents[idx])
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"choices":[{"message":{"role":"assistant","content":%s}}],"usage":{"prompt_tokens":5,"completion_tokens":1}}`, c)
	}))
	t.Cleanup(rs.Close)
	return rs
}

func (rs *recordingServer) requests() []map[string]any {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	return append([]map[string]any(nil), rs.bodies...)
}

// explainBody pulls the JSON body out of the -d '...' line of --explain output.
func explainBody(t *testing.T, out string) map[string]any {
	t.Helper()
	i := strings.Index(out, "-d '")
	if i < 0 {
		t.Fatalf("no -d line in explain output:\n%s", out)
	}
	raw := strings.TrimSpace(out[i+4:])
	raw = strings.TrimSuffix(raw, "'")
	raw = strings.ReplaceAll(raw, `'\''`, "'")
	var body map[string]any
	if err := json.Unmarshal([]byte(raw), &body); err != nil {
		t.Fatalf("explain body is not JSON: %v\n%s", err, raw)
	}
	return body
}

func systemMessage(body map[string]any) string {
	msgs, _ := body["messages"].([]any)
	for _, m := range msgs {
		mm, _ := m.(map[string]any)
		if mm["role"] == "system" {
			s, _ := mm["content"].(string)
			return s
		}
	}
	return ""
}

// A-20a: OpenAI gets a real JSON Schema, not the flat map.
func TestOpenAIExplainSchemaIsJSONSchema(t *testing.T) {
	stdout, _, code := runPrompt(t, map[string]string{"OPENAI_API_KEY": "x"},
		[]string{"-m", "gpt-4o-mini", "--schema", `{"a":"string"}`, "--explain"}, "j")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	body := explainBody(t, stdout)
	rf := body["response_format"].(map[string]any)
	js := rf["json_schema"].(map[string]any)
	if js["strict"] != true {
		t.Errorf("strict = %v, want true for scalar-only schema", js["strict"])
	}
	got, _ := json.Marshal(js["schema"])
	want := `{"additionalProperties":false,"properties":{"a":{"type":"string"}},"required":["a"],"type":"object"}`
	if string(got) != want {
		t.Errorf("schema = %s\nwant     %s", got, want)
	}
}

func TestOpenAISchemaNonScalarNotStrict(t *testing.T) {
	s, strict := openAIJSONSchemaFor(map[string]string{"tags": "array", "meta": "object", "n": "integer", "b": "boolean"})
	if strict {
		t.Error("strict = true, want false when schema has array/object")
	}
	got, _ := json.Marshal(s)
	want := `{"additionalProperties":false,"properties":{"b":{"type":"boolean"},"meta":{"type":"object"},"n":{"type":"integer"},"tags":{"items":{},"type":"array"}},"required":["b","meta","n","tags"],"type":"object"}`
	if string(got) != want {
		t.Errorf("schema = %s\nwant     %s", got, want)
	}
}

func TestBuildSystemPromptStable(t *testing.T) {
	s := map[string]string{"z": "string", "a": "number", "m": "boolean"}
	first := buildSystemPrompt(s)
	for i := 0; i < 20; i++ {
		if buildSystemPrompt(s) != first {
			t.Fatal("buildSystemPrompt output is not stable")
		}
	}
	if strings.Index(first, `"a"`) > strings.Index(first, `"z"`) {
		t.Errorf("keys not sorted:\n%s", first)
	}
}

func TestEndpointSchemaMatchPasses(t *testing.T) {
	srv := newRecordingServer(t, `{"a":"x"}`)
	stdout, _, code := runPrompt(t, nil,
		[]string{"--endpoint", srv.URL, "-m", "mock", "--schema", `{"a":"string"}`, "--fail"}, "j")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, `{"a":"x"}`) {
		t.Errorf("stdout = %q", stdout)
	}
}

// A-20b
func TestEndpointSchemaMismatchFail(t *testing.T) {
	srv := newRecordingServer(t, `{"z":"x"}`)
	stdout, stderr, code := runPrompt(t, nil,
		[]string{"--endpoint", srv.URL, "-m", "mock", "--schema", `{"a":"string"}`, "--fail"}, "j")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stdout != "" {
		t.Errorf("stdout must be empty on error, got %q", stdout)
	}
	if !strings.Contains(stderr, "does not match schema") {
		t.Errorf("stderr = %q", stderr)
	}
	if n := len(srv.requests()); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestEndpointSchemaMismatchFailJSON(t *testing.T) {
	srv := newRecordingServer(t, `{"z":"x"}`)
	stdout, stderr, code := runPrompt(t, nil,
		[]string{"--endpoint", srv.URL, "-m", "mock", "--schema", `{"a":"string"}`, "--fail", "--json"}, "j")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr must be empty with --json, got %q", stderr)
	}
	if !strings.Contains(stdout, `"error"`) || !strings.Contains(stdout, "does not match schema") {
		t.Errorf("stdout = %q, want JSON error envelope", stdout)
	}
}

func TestEndpointSchemaMismatchNoFailExitsZero(t *testing.T) {
	srv := newRecordingServer(t, `{"z":"x"}`)
	stdout, _, code := runPrompt(t, nil,
		[]string{"--endpoint", srv.URL, "-m", "mock", "--schema", `{"a":"string"}`}, "j")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (no --fail, no --retry)", code)
	}
	if !strings.Contains(stdout, `{"z":"x"}`) {
		t.Errorf("stdout = %q, want the response printed", stdout)
	}
}

// A-20c
func TestEndpointSchemaRetryEscalates(t *testing.T) {
	srv := newRecordingServer(t, `{"z":"x"}`)
	_, stderr, code := runPrompt(t, nil,
		[]string{"--endpoint", srv.URL, "-m", "mock", "--schema", `{"a":"string"}`, "--retry", "2"}, "j")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "response does not match schema after 3 attempts") {
		t.Errorf("stderr = %q", stderr)
	}
	reqs := srv.requests()
	if len(reqs) != 3 {
		t.Fatalf("requests = %d, want 3", len(reqs))
	}
	prev := -1.0
	for i, r := range reqs {
		temp, _ := r["temperature"].(float64)
		if temp <= prev {
			t.Errorf("request %d temperature %v did not rise above %v", i+1, temp, prev)
		}
		prev = temp
	}
}

func TestEndpointNonJSONReplyFail(t *testing.T) {
	srv := newRecordingServer(t, "sorry, no json")
	_, _, code := runPrompt(t, nil,
		[]string{"--endpoint", srv.URL, "-m", "mock", "--schema", `{"a":"string"}`, "--fail"}, "j")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

// A-20d
func TestEndpointSchemaSystemMessage(t *testing.T) {
	srv := newRecordingServer(t, `{"a":"x"}`)
	_, _, code := runPrompt(t, nil,
		[]string{"--endpoint", srv.URL, "-m", "mock", "--system", "be terse", "--schema", `{"a":"string"}`}, "j")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	reqs := srv.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1", len(reqs))
	}
	sys := systemMessage(reqs[0])
	if !strings.HasPrefix(sys, "be terse\n\n") {
		t.Errorf("system message does not start with user --system text: %q", sys)
	}
	if !strings.Contains(sys, `"a"`) || !strings.Contains(sys, "JSON") {
		t.Errorf("system message lacks schema key or the word JSON: %q", sys)
	}
	rf, _ := reqs[0]["response_format"].(map[string]any)
	if rf["type"] != "json_object" {
		t.Errorf("response_format = %v, want json_object", reqs[0]["response_format"])
	}
}

// --explain on --endpoint prints the body that would actually be sent.
func TestEndpointExplainMatchesSentBody(t *testing.T) {
	args := []string{"-m", "mock", "--system", "be terse", "--schema", `{"a":"string"}`}
	srv := newRecordingServer(t, `{"a":"x"}`)
	stdout, _, code := runPrompt(t, nil, append([]string{"--endpoint", srv.URL, "--explain"}, args...), "j")
	if code != 0 {
		t.Fatalf("explain exit code = %d", code)
	}
	explained := explainBody(t, stdout)
	if _, _, code = runPrompt(t, nil, append([]string{"--endpoint", srv.URL}, args...), "j"); code != 0 {
		t.Fatalf("call exit code = %d", code)
	}
	reqs := srv.requests()
	if len(reqs) != 1 {
		t.Fatalf("requests = %d, want 1 (explain must not send)", len(reqs))
	}
	a, _ := json.Marshal(explained)
	b, _ := json.Marshal(reqs[0])
	if string(a) != string(b) {
		t.Errorf("explain body differs from sent body\nexplain: %s\nsent:    %s", a, b)
	}
}

// OpenAI provider (not --endpoint) now validates too. Route the hardcoded
// api.openai.com URL to the stub through the shared client's transport.
func TestOpenAISchemaMismatchFail(t *testing.T) {
	srv := newRecordingServer(t, `{"z":"x"}`)
	target, _ := url.Parse(srv.URL)
	orig := apiClient
	apiClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		r.URL.Scheme, r.URL.Host = target.Scheme, target.Host
		return http.DefaultTransport.RoundTrip(r)
	})}
	t.Cleanup(func() { apiClient = orig })

	_, stderr, code := runPrompt(t, map[string]string{"OPENAI_API_KEY": "sk-secret-123"},
		[]string{"-m", "gpt-4o-mini", "--schema", `{"a":"string"}`, "--retry", "1"}, "j")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if n := len(srv.requests()); n != 2 {
		t.Errorf("requests = %d, want 2", n)
	}
	if strings.Contains(stderr, "sk-secret-123") {
		t.Error("stderr leaked the API key")
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// A-20e / D8: empty prompt is exit 1 and sends nothing.
func TestEmptyPromptNothingSent(t *testing.T) {
	for _, in := range []string{"", "\n", "  \n\t\n"} {
		srv := newRecordingServer(t, "pong")
		stdout, stderr, code := runPrompt(t, nil, []string{"--endpoint", srv.URL, "-m", "mock"}, in)
		if code != 1 {
			t.Errorf("input %q: exit code = %d, want 1", in, code)
		}
		if stdout != "" {
			t.Errorf("input %q: stdout = %q, want empty", in, stdout)
		}
		if !strings.Contains(stderr, "prompt: empty prompt, nothing sent") {
			t.Errorf("input %q: stderr = %q", in, stderr)
		}
		if n := len(srv.requests()); n != 0 {
			t.Errorf("input %q: requests = %d, want 0", in, n)
		}
	}
}

func TestEmptyPromptJSON(t *testing.T) {
	stdout, stderr, code := runPrompt(t, map[string]string{"ANTHROPIC_API_KEY": "fake"}, []string{"--json"}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "empty prompt, nothing sent") {
		t.Errorf("stdout = %q, want JSON error", stdout)
	}
}

func TestEmptyPositionalArg(t *testing.T) {
	_, _, code := runPrompt(t, map[string]string{"ANTHROPIC_API_KEY": "fake"}, []string{"  "}, "")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
}

func TestFieldEmptyPromptSkipped(t *testing.T) {
	srv := newRecordingServer(t, "pong")
	stdin := "{\"text\":\"\"}\n{\"text\":\"hi\"}\n{\"text\":\"  \"}\n"
	stdout, stderr, code := runPrompt(t, nil,
		[]string{"--field", "text", "--endpoint", srv.URL, "-m", "mock"}, stdin)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0\nstderr: %s", code, stderr)
	}
	if stdout != "pong\n" {
		t.Errorf("stdout = %q, want one response", stdout)
	}
	if !strings.Contains(stderr, "line 1: empty prompt, skipped") || !strings.Contains(stderr, "line 3: empty prompt, skipped") {
		t.Errorf("stderr = %q", stderr)
	}
	if n := len(srv.requests()); n != 1 {
		t.Errorf("requests = %d, want 1", n)
	}
}

func TestFieldSchemaMismatchFail(t *testing.T) {
	srv := newRecordingServer(t, `{"z":"x"}`)
	_, stderr, code := runPrompt(t, nil,
		[]string{"--field", "text", "--endpoint", srv.URL, "-m", "mock", "--schema", `{"a":"string"}`, "--fail"},
		"{\"text\":\"hi\"}\n")
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stderr, "line 1:") {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestFieldSchemaMismatchNoFailExitsZero(t *testing.T) {
	srv := newRecordingServer(t, `{"z":"x"}`)
	stdout, _, code := runPrompt(t, nil,
		[]string{"--field", "text", "--endpoint", srv.URL, "-m", "mock", "--schema", `{"a":"string"}`},
		"{\"text\":\"hi\"}\n")
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(stdout, `{"z":"x"}`) {
		t.Errorf("stdout = %q", stdout)
	}
}

// Property: for any schema of known types, the OpenAI schema lists every key
// as required, in sorted order, with additionalProperties false.
func TestOpenAISchemaProperty(t *testing.T) {
	types := []string{"string", "number", "integer", "boolean", "array", "object"}
	for n := 1; n <= len(types); n++ {
		in := map[string]string{}
		for i := 0; i < n; i++ {
			in[fmt.Sprintf("k%d", n-i)] = types[i]
		}
		s, _ := openAIJSONSchemaFor(in)
		req, _ := s["required"].([]string)
		if len(req) != n || s["additionalProperties"] != false || s["type"] != "object" {
			t.Fatalf("bad schema for %v: %v", in, s)
		}
		for i := 1; i < len(req); i++ {
			if req[i-1] > req[i] {
				t.Fatalf("required not sorted: %v", req)
			}
		}
	}
}
