// Package mcp implements vrk mcp — a discovery-only MCP server.
//
// mcp is a utility, not a pipeline tool. It is not registered in the tool
// registry, does not appear in manifest.json, and is not enumerable via its
// own tools/list response. Its role is to make all registered pipeline tools
// discoverable by MCP clients.
//
// tools/call is explicitly out of scope and will be added in a future iteration.
package mcp

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// Version can be overridden via -ldflags at build time.
var Version = "dev"

// Run is the entry point for vrk mcp. Returns 0/1/2. Never calls os.Exit.
func Run(
	descriptions map[string]string,
	flagsFn map[string]func() *pflag.FlagSet,
	stdinRequired map[string]bool,
) int {
	fs := pflag.NewFlagSet("mcp", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var listFlag bool
	fs.BoolVar(&listFlag, "list", false, "print all exposed tools and exit")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	if listFlag {
		return runList(flagsFn, descriptions)
	}

	return runStdio(flagsFn, descriptions, stdinRequired)
}

// runList prints all tools with descriptions, column-aligned, and exits 0.
func runList(
	flagsFn map[string]func() *pflag.FlagSet,
	descriptions map[string]string,
) int {
	names := make([]string, 0, len(flagsFn))
	for name := range flagsFn {
		names = append(names, name)
	}
	sort.Strings(names)

	// Find longest name for padding.
	maxLen := 0
	for _, name := range names {
		if len(name) > maxLen {
			maxLen = len(name)
		}
	}

	for _, name := range names {
		desc := descriptions[name]
		padding := strings.Repeat(" ", maxLen-len(name))
		if _, err := fmt.Fprintf(os.Stdout, "%s%s   %s\n", name, padding, desc); err != nil {
			return shared.Errorf("mcp: writing list: %v", err)
		}
	}
	return 0
}

// JSON-RPC 2.0 types

type jsonrpcRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method"`
	Params  json.RawMessage  `json:"params,omitempty"`
}

type jsonrpcResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// MCP result types

type initializeResult struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Capabilities    map[string]any `json:"capabilities"`
	ServerInfo      serverInfo     `json:"serverInfo"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type toolsListResult struct {
	Tools []mcpTool `json:"tools"`
}

type mcpTool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties"`
	Required   []string                  `json:"required,omitempty"`
}

type schemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// runStdio runs the JSON-RPC 2.0 loop on stdin/stdout.
func runStdio(
	flagsFn map[string]func() *pflag.FlagSet,
	descriptions map[string]string,
	stdinRequired map[string]bool,
) int {
	// Build tools list once at startup.
	tools := buildToolsList(flagsFn, descriptions, stdinRequired)

	enc := json.NewEncoder(os.Stdout)
	scanner := bufio.NewScanner(os.Stdin)
	// Increase scanner buffer for large requests.
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req jsonrpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			// Parse error — respond with null id per JSON-RPC spec.
			_ = enc.Encode(jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      nil,
				Error:   &rpcError{Code: -32700, Message: "parse error"},
			})
			continue
		}

		// Notifications have no id — do not respond.
		if req.ID == nil {
			continue
		}

		// Decode the id to use in the response.
		var id interface{}
		if err := json.Unmarshal(*req.ID, &id); err != nil {
			id = nil
		}

		var resp jsonrpcResponse
		switch req.Method {
		case "initialize":
			resp = jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result: initializeResult{
					ProtocolVersion: "2024-11-05",
					Capabilities:    map[string]any{"tools": map[string]any{}},
					ServerInfo:      serverInfo{Name: "vrk", Version: Version},
				},
			}

		case "tools/list":
			resp = jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      id,
				Result:  toolsListResult{Tools: tools},
			}

		default:
			resp = jsonrpcResponse{
				JSONRPC: "2.0",
				ID:      id,
				Error:   &rpcError{Code: -32601, Message: "method not found: " + req.Method},
			}
		}
		_ = enc.Encode(resp)
	}

	return 0
}

// buildToolsList constructs the MCP tool list from flagsFn and descriptions.
func buildToolsList(
	flagsFn map[string]func() *pflag.FlagSet,
	descriptions map[string]string,
	stdinRequired map[string]bool,
) []mcpTool {
	names := make([]string, 0, len(flagsFn))
	for name := range flagsFn {
		names = append(names, name)
	}
	sort.Strings(names)

	tools := make([]mcpTool, 0, len(names))
	for _, name := range names {
		props := map[string]schemaProperty{
			"input": {Type: "string", Description: "stdin input for the tool"},
		}

		// Build properties from the tool's flag definitions.
		if fn, ok := flagsFn[name]; ok {
			fs := fn()
			fs.VisitAll(func(f *pflag.Flag) {
				props[f.Name] = schemaProperty{
					Type:        pflagTypeToJSONSchema(f.Value.Type()),
					Description: f.Usage,
				}
			})
		}

		schema := inputSchema{
			Type:       "object",
			Properties: props,
		}

		if stdinRequired[name] {
			schema.Required = []string{"input"}
		}

		tools = append(tools, mcpTool{
			Name:        "vrk_" + name,
			Description: descriptions[name],
			InputSchema: schema,
		})
	}
	return tools
}

// pflagTypeToJSONSchema maps pflag type strings to JSON Schema types.
func pflagTypeToJSONSchema(t string) string {
	switch t {
	case "bool":
		return "boolean"
	case "int", "int64", "float64":
		return "number"
	default:
		// string, duration, stringArray, intSlice, etc. — all represented as string
		return "string"
	}
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk mcp [flags]",
		"       vrk mcp --list",
		"",
		"Start a discovery-only MCP server (JSON-RPC 2.0 over stdio).",
		"Exposes all vrksh pipeline tools for MCP client discovery.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("mcp: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
