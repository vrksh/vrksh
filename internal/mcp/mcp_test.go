package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/pflag"
)

// stubFlags returns a FlagSet with two flags for testing schema generation.
func stubFlags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("stub", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolP("json", "j", false, "emit output as JSON")
	fs.String("name", "", "a name flag")
	return fs
}

// emptyFlags returns a FlagSet with no flags (simulates a broken Flags() implementation).
func emptyFlags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("empty", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	return fs
}

// stubDescriptions, stubFlagsFn, stubStdinRequired provide a minimal test registry.
var (
	stubDescriptions = map[string]string{
		"alpha": "first tool",
		"beta":  "second tool",
	}
	stubFlagsFn = map[string]func() *pflag.FlagSet{
		"alpha": stubFlags,
		"beta":  stubFlags,
	}
	stubStdinRequired = map[string]bool{
		"alpha": true,
		"beta":  false,
	}
)

// runMCP calls Run with the given args, capturing stdout and stderr.
func runMCP(
	args []string,
	stdin string,
	descriptions map[string]string,
	flagsFn map[string]func() *pflag.FlagSet,
	stdinRequired map[string]bool,
) (stdout string, stderr string, exitCode int) {
	origArgs := os.Args
	origStdin := os.Stdin
	origStdout := os.Stdout
	origStderr := os.Stderr
	defer func() {
		os.Args = origArgs
		os.Stdin = origStdin
		os.Stdout = origStdout
		os.Stderr = origStderr
	}()

	// Set os.Args
	os.Args = append([]string{"mcp"}, args...)

	// Replace stdin
	pr, pw, _ := os.Pipe()
	go func() {
		_, _ = pw.WriteString(stdin)
		_ = pw.Close()
	}()
	os.Stdin = pr

	// Capture stdout
	outR, outW, _ := os.Pipe()
	os.Stdout = outW
	var outBuf bytes.Buffer
	outDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&outBuf, outR)
		close(outDone)
	}()

	// Capture stderr
	errR, errW, _ := os.Pipe()
	os.Stderr = errW
	var errBuf bytes.Buffer
	errDone := make(chan struct{})
	go func() {
		_, _ = io.Copy(&errBuf, errR)
		close(errDone)
	}()

	exitCode = Run(descriptions, flagsFn, stdinRequired)

	_ = outW.Close()
	_ = errW.Close()
	<-outDone
	<-errDone
	_ = pr.Close()

	return outBuf.String(), errBuf.String(), exitCode
}

func TestListPrintsToolsColumnAligned(t *testing.T) {
	stdout, _, code := runMCP([]string{"--list"}, "", stubDescriptions, stubFlagsFn, stubStdinRequired)
	if code != 0 {
		t.Fatalf("--list: want exit 0, got %d", code)
	}
	if !strings.Contains(stdout, "alpha") {
		t.Error("--list output missing 'alpha'")
	}
	if !strings.Contains(stdout, "beta") {
		t.Error("--list output missing 'beta'")
	}
	if !strings.Contains(stdout, "first tool") {
		t.Error("--list output missing description 'first tool'")
	}
	// Verify column alignment: both tool names should be padded to same width
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	if len(lines) != 2 {
		t.Fatalf("--list: expected 2 lines, got %d", len(lines))
	}
	// "alpha" (5 chars) and "beta" (4 chars) — alpha is longest, beta should be padded
	// Both description columns should start at the same position
	idx0 := strings.Index(lines[0], stubDescriptions["alpha"])
	idx1 := strings.Index(lines[1], stubDescriptions["beta"])
	if idx0 < 0 || idx1 < 0 {
		t.Fatal("--list: descriptions not found in output lines")
	}
	if idx0 == 0 || idx1 == 0 {
		t.Error("--list: descriptions not indented from tool name")
	}
}

func TestHelpExitsZero(t *testing.T) {
	_, _, code := runMCP([]string{"--help"}, "", stubDescriptions, stubFlagsFn, stubStdinRequired)
	if code != 0 {
		t.Fatalf("--help: want exit 0, got %d", code)
	}
}

func TestUnknownFlagExitsTwo(t *testing.T) {
	_, _, code := runMCP([]string{"--bogus"}, "", stubDescriptions, stubFlagsFn, stubStdinRequired)
	if code != 2 {
		t.Fatalf("--bogus: want exit 2, got %d", code)
	}
}

func TestInitialize(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"
	stdout, _, code := runMCP(nil, req, stubDescriptions, stubFlagsFn, stubStdinRequired)
	if code != 0 {
		t.Fatalf("initialize: want exit 0, got %d", code)
	}

	var resp struct {
		JSONRPC string `json:"jsonrpc"`
		ID      int    `json:"id"`
		Result  struct {
			ProtocolVersion string `json:"protocolVersion"`
			Capabilities    struct {
				Tools map[string]any `json:"tools"`
			} `json:"capabilities"`
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &resp); err != nil {
		t.Fatalf("initialize: cannot parse response: %v\nstdout: %s", err, stdout)
	}
	if resp.JSONRPC != "2.0" {
		t.Errorf("jsonrpc: want 2.0, got %q", resp.JSONRPC)
	}
	if resp.ID != 1 {
		t.Errorf("id: want 1, got %d", resp.ID)
	}
	if resp.Result.ProtocolVersion != "2024-11-05" {
		t.Errorf("protocolVersion: want 2024-11-05, got %q", resp.Result.ProtocolVersion)
	}
	if resp.Result.ServerInfo.Name != "vrk" {
		t.Errorf("serverInfo.name: want vrk, got %q", resp.Result.ServerInfo.Name)
	}
	if resp.Result.ServerInfo.Version == "" {
		t.Error("serverInfo.version: want non-empty")
	}
	if resp.Result.Capabilities.Tools == nil {
		t.Error("capabilities.tools: want non-nil map")
	}
}

func TestToolsList(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	stdout, _, code := runMCP(nil, req, stubDescriptions, stubFlagsFn, stubStdinRequired)
	if code != 0 {
		t.Fatalf("tools/list: want exit 0, got %d", code)
	}

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				Description string `json:"description"`
				InputSchema struct {
					Type       string         `json:"type"`
					Properties map[string]any `json:"properties"`
					Required   []string       `json:"required"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &resp); err != nil {
		t.Fatalf("tools/list: cannot parse: %v\nstdout: %s", err, stdout)
	}

	tools := resp.Result.Tools
	if len(tools) != 2 {
		t.Fatalf("tools/list: want 2 tools, got %d", len(tools))
	}

	// Find alpha and beta by name
	toolMap := map[string]int{}
	for i, tool := range tools {
		toolMap[tool.Name] = i
	}

	alphaIdx, ok := toolMap["vrk_alpha"]
	if !ok {
		t.Fatal("tools/list: missing vrk_alpha")
	}
	betaIdx, ok := toolMap["vrk_beta"]
	if !ok {
		t.Fatal("tools/list: missing vrk_beta")
	}

	// Check descriptions
	if tools[alphaIdx].Description != "first tool" {
		t.Errorf("vrk_alpha description: want 'first tool', got %q", tools[alphaIdx].Description)
	}

	// Check inputSchema has properties
	alphaProps := tools[alphaIdx].InputSchema.Properties
	if alphaProps == nil {
		t.Fatal("vrk_alpha: inputSchema.properties is nil")
	}
	if _, ok := alphaProps["input"]; !ok {
		t.Error("vrk_alpha: inputSchema missing 'input' property")
	}
	if _, ok := alphaProps["json"]; !ok {
		t.Error("vrk_alpha: inputSchema missing 'json' property")
	}
	if _, ok := alphaProps["name"]; !ok {
		t.Error("vrk_alpha: inputSchema missing 'name' property")
	}

	// Check schema type
	if tools[alphaIdx].InputSchema.Type != "object" {
		t.Errorf("vrk_alpha inputSchema.type: want 'object', got %q", tools[alphaIdx].InputSchema.Type)
	}

	// Check that beta has no required (stdinRequired=false)
	if len(tools[betaIdx].InputSchema.Required) != 0 {
		t.Errorf("vrk_beta: want no required, got %v", tools[betaIdx].InputSchema.Required)
	}
}

func TestToolsListStdinRequired(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}` + "\n"
	stdout, _, _ := runMCP(nil, req, stubDescriptions, stubFlagsFn, stubStdinRequired)

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Required []string `json:"required"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	for _, tool := range resp.Result.Tools {
		hasInputRequired := false
		for _, r := range tool.InputSchema.Required {
			if r == "input" {
				hasInputRequired = true
			}
		}
		switch tool.Name {
		case "vrk_alpha":
			if !hasInputRequired {
				t.Error("vrk_alpha: stdinRequired=true but 'input' not in required")
			}
		case "vrk_beta":
			if hasInputRequired {
				t.Error("vrk_beta: stdinRequired=false but 'input' in required")
			}
		}
	}
}

func TestUnknownMethodReturnsError(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":5,"method":"tools/call","params":{}}` + "\n"
	stdout, _, code := runMCP(nil, req, stubDescriptions, stubFlagsFn, stubStdinRequired)
	if code != 0 {
		t.Fatalf("want exit 0 (server ran), got %d", code)
	}

	var resp struct {
		Error struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &resp); err != nil {
		t.Fatalf("parse error: %v\nstdout: %s", err, stdout)
	}
	if resp.Error.Code != -32601 {
		t.Errorf("error code: want -32601, got %d", resp.Error.Code)
	}
}

func TestMalformedJSONReturnsParseError(t *testing.T) {
	req := "not json at all\n"
	stdout, _, code := runMCP(nil, req, stubDescriptions, stubFlagsFn, stubStdinRequired)
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}

	var resp struct {
		Error struct {
			Code int `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &resp); err != nil {
		t.Fatalf("parse error: %v\nstdout: %s", err, stdout)
	}
	if resp.Error.Code != -32700 {
		t.Errorf("error code: want -32700, got %d", resp.Error.Code)
	}
}

func TestEmptyRegistryReturnsEmptyToolsList(t *testing.T) {
	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"
	empty := map[string]string{}
	emptyFn := map[string]func() *pflag.FlagSet{}
	emptyStdin := map[string]bool{}

	stdout, _, code := runMCP(nil, req, empty, emptyFn, emptyStdin)
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}

	var resp struct {
		Result struct {
			Tools []any `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if resp.Result.Tools == nil {
		t.Error("tools should be empty array, not null")
	}
	if len(resp.Result.Tools) != 0 {
		t.Errorf("want 0 tools, got %d", len(resp.Result.Tools))
	}
}

func TestNotificationProducesNoResponse(t *testing.T) {
	// Send a notification (no id) followed by a valid request.
	// Should produce exactly one response line — for the request, not the notification.
	input := `{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}` + "\n"

	stdout, _, code := runMCP(nil, input, stubDescriptions, stubFlagsFn, stubStdinRequired)
	if code != 0 {
		t.Fatalf("want exit 0, got %d", code)
	}

	// Count non-empty lines in stdout
	scanner := bufio.NewScanner(strings.NewReader(stdout))
	lineCount := 0
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lineCount++
		}
	}
	if lineCount != 1 {
		t.Errorf("want exactly 1 response line, got %d\nstdout: %s", lineCount, stdout)
	}
}

func TestEmptyFlagSetProducesInputOnly(t *testing.T) {
	desc := map[string]string{"broken": "tool with no flags"}
	fn := map[string]func() *pflag.FlagSet{"broken": emptyFlags}
	stdin := map[string]bool{"broken": false}

	req := `{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}` + "\n"
	stdout, _, _ := runMCP(nil, req, desc, fn, stdin)

	var resp struct {
		Result struct {
			Tools []struct {
				Name        string `json:"name"`
				InputSchema struct {
					Properties map[string]any `json:"properties"`
				} `json:"inputSchema"`
			} `json:"tools"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(stdout)), &resp); err != nil {
		t.Fatalf("parse error: %v", err)
	}

	if len(resp.Result.Tools) != 1 {
		t.Fatalf("want 1 tool, got %d", len(resp.Result.Tools))
	}

	props := resp.Result.Tools[0].InputSchema.Properties
	// Should have input but nothing else
	if _, ok := props["input"]; !ok {
		t.Error("missing 'input' property")
	}
	nonInputCount := 0
	for k := range props {
		if k != "input" {
			nonInputCount++
		}
	}
	if nonInputCount != 0 {
		t.Errorf("empty Flags() should produce 0 non-input properties, got %d", nonInputCount)
	}
}
