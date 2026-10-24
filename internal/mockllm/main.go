// Command mockllm is a tiny OpenAI-compatible chat completions server for
// tests. It logs every request as one JSON line and returns canned replies.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
)

type server struct {
	reply     string
	badFirst  int
	failFirst int

	mu         sync.Mutex
	logFile    *os.File
	requests   int
	schemaReqs int
}

func main() {
	port := flag.Int("port", 0, "port to listen on (0 picks a free one)")
	logPath := flag.String("log", "", "append one JSON line per request to this file")
	reply := flag.String("reply", "", "always reply with this text")
	badFirst := flag.Int("bad-first", 0, "answer the first N schema requests with 'not json'")
	failFirst := flag.Int("fail-first", 0, "answer the first N requests with HTTP 500")
	flag.Parse()

	s := &server{reply: *reply, badFirst: *badFirst, failFirst: *failFirst}
	if *logPath != "" {
		f, err := os.OpenFile(*logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "mockllm:", err)
			os.Exit(1)
		}
		s.logFile = f
	}
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", *port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "mockllm:", err)
		os.Exit(1)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handle)
	fmt.Fprintf(os.Stderr, "listening on %s\n", ln.Addr())
	if err := http.Serve(ln, mux); err != nil {
		fmt.Fprintln(os.Stderr, "mockllm:", err)
		os.Exit(1)
	}
}

type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type request struct {
	Messages       []message `json:"messages"`
	Stream         bool      `json:"stream"`
	ResponseFormat struct {
		JSONSchema struct {
			Schema map[string]any `json:"schema"`
		} `json:"json_schema"`
	} `json:"response_format"`
}

func (m message) text() string {
	switch c := m.Content.(type) {
	case string:
		return c
	case []any:
		var b strings.Builder
		for _, p := range c {
			if pm, ok := p.(map[string]any); ok {
				if t, ok := pm["text"].(string); ok {
					b.WriteString(t)
				}
			}
		}
		return b.String()
	}
	return ""
}

func (s *server) handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	var req request
	_ = json.Unmarshal(body, &req)

	auth := r.Header.Get("Authorization")
	if len(auth) > 12 {
		auth = auth[:12]
	}
	var raw any = string(body)
	if json.Valid(body) {
		raw = json.RawMessage(body)
	}
	line, _ := json.Marshal(map[string]any{"path": r.URL.Path, "auth": auth, "body": raw})

	content, isSchema := s.content(req)

	s.mu.Lock()
	if s.logFile != nil {
		_, _ = s.logFile.Write(append(line, '\n'))
	}
	s.requests++
	fail := s.requests <= s.failFirst
	bad := false
	if isSchema {
		s.schemaReqs++
		bad = s.schemaReqs <= s.badFirst
	}
	s.mu.Unlock()

	if fail {
		http.Error(w, `{"error":{"message":"mock failure"}}`, http.StatusInternalServerError)
		return
	}
	if bad {
		content = "not json"
	}
	if req.Stream {
		writeStream(w, content)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id": "mock-1", "object": "chat.completion", "model": "mock",
		"choices": []any{map[string]any{
			"index": 0, "finish_reason": "stop",
			"message": map[string]any{"role": "assistant", "content": content},
		}},
		"usage": map[string]any{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
	})
}

func writeStream(w http.ResponseWriter, content string) {
	w.Header().Set("Content-Type", "text/event-stream")
	fl, _ := w.(http.Flusher)
	send := func(delta map[string]any, finish any) {
		b, _ := json.Marshal(map[string]any{
			"id": "mock-1", "object": "chat.completion.chunk", "model": "mock",
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish}},
		})
		_, _ = fmt.Fprintf(w, "data: %s\n\n", b)
		if fl != nil {
			fl.Flush()
		}
	}
	// ponytail: splits on runes in chunks of 8; real APIs split on tokens.
	rs := []rune(content)
	for i := 0; i < len(rs); i += 8 {
		j := min(i+8, len(rs))
		send(map[string]any{"content": string(rs[i:j])}, nil)
	}
	send(map[string]any{}, "stop")
	_, _ = fmt.Fprint(w, "data: [DONE]\n\n")
	if fl != nil {
		fl.Flush()
	}
}

// content picks the reply text. The bool reports whether a schema was found.
func (s *server) content(req request) (string, bool) {
	var sys strings.Builder
	last := ""
	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			sys.WriteString(m.text())
			sys.WriteString("\n")
		case "user":
			last = m.text()
		}
	}
	obj := schemaFromText(sys.String())
	if obj == nil && req.ResponseFormat.JSONSchema.Schema != nil {
		obj = fillSchema(req.ResponseFormat.JSONSchema.Schema)
	}
	if s.reply != "" {
		return s.reply, obj != nil
	}
	if obj != nil {
		b, _ := json.Marshal(obj)
		return string(b), true
	}
	rs := []rune(last)
	head := rs
	if len(head) > 50 {
		head = head[:50]
	}
	return fmt.Sprintf("MOCK(%d chars): %s", len(rs), string(head)), false
}

// schemaFromText finds the first JSON object in text that looks like a flat
// {"key":"type"} schema (or a JSON Schema with properties) and fills it.
func schemaFromText(text string) map[string]any {
	for i := 0; i < len(text); i++ {
		if text[i] != '{' {
			continue
		}
		var m map[string]any
		d := json.NewDecoder(strings.NewReader(text[i:]))
		if d.Decode(&m) != nil || len(m) == 0 {
			continue
		}
		if out := fillSchema(m); out != nil {
			return out
		}
	}
	return nil
}

var typeValues = map[string]any{
	"string": "mock", "number": 1, "integer": 1, "int": 1, "float": 1,
	"boolean": true, "bool": true, "array": []any{}, "object": map[string]any{},
}

// fillSchema accepts a flat key->type map or a JSON Schema with properties.
// It returns nil if the map is neither.
func fillSchema(m map[string]any) map[string]any {
	if props, ok := m["properties"].(map[string]any); ok && len(props) > 0 {
		m = props
	}
	out := map[string]any{}
	for k, v := range m {
		t, _ := v.(string)
		if sub, ok := v.(map[string]any); ok {
			t, _ = sub["type"].(string)
		}
		val, ok := typeValues[t]
		if !ok {
			return nil
		}
		out[k] = val
	}
	return out
}
