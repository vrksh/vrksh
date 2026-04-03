package sse

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// stdinReader is a var so tests can inject I/O errors without touching os.Stdin.
// nil means "use os.Stdin" at the time Run() is called.
var stdinReader io.Reader

type sseRecord struct {
	Event string      `json:"event"`
	Data  interface{} `json:"data"`
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "sse",
		Short: "SSE stream parser — text/event-stream to JSONL",
		Flags: []shared.FlagMeta{
			{Name: "event", Shorthand: "e", Usage: "only emit events of this type; skip all others"},
			{Name: "field", Shorthand: "F", Usage: "extract dot-path field from the record and print as plain text"},
		},
	})
}

// Run is the entry point for vrk sse. Returns 0, 1, or 2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("sse", pflag.ContinueOnError)
	var eventFilter string
	var fieldPath string
	fs.StringVarP(&eventFilter, "event", "e", "", "only emit events of this type; skip all others")
	fs.StringVarP(&fieldPath, "field", "F", "", "extract dot-path field from the record and print as plain text")

	// Suppress pflag's automatic printing so all output goes through shared helpers.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// TTY detection: if stdin is a character device (interactive terminal) and no
	// positional args were given, the user ran vrk sse with no piped stream.
	if isTerminal(int(os.Stdin.Fd())) {
		return shared.UsageErrorf("sse: no input: pipe an SSE stream to stdin")
	}

	w := bufio.NewWriter(os.Stdout)
	defer func() { _ = w.Flush() }()

	r := stdinReader
	if r == nil {
		r = os.Stdin
	}
	scanner := shared.ScanLines(r)

	// SSE state machine: per-block accumulators, reset on each blank line.
	var eventName string
	var dataLines []string
	hasData := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			// Blank line: dispatch the current block if it has data.
			if hasData {
				code := dispatch(w, eventFilter, fieldPath, eventName, dataLines)
				if code == -1 {
					// [DONE] sentinel — stop cleanly.
					return 0
				}
				if code != 0 {
					return code
				}
			}
			// Reset state for the next block.
			eventName = ""
			dataLines = dataLines[:0]
			hasData = false
			continue
		}

		// Comment lines start with ':' — silently skip.
		if strings.HasPrefix(line, ":") {
			continue
		}

		if strings.HasPrefix(line, "event:") {
			eventName = stripOneSpace(line[len("event:"):])
			continue
		}

		if strings.HasPrefix(line, "data:") {
			val := stripOneSpace(line[len("data:"):])
			dataLines = append(dataLines, val)
			hasData = true
			continue
		}

		// Any other line (unrecognised field name) — silently skip per SSE spec.
	}

	if err := scanner.Err(); err != nil {
		return shared.Errorf("sse: reading stdin: %v", err)
	}

	// EOF mid-block (no trailing blank line): pending block is dropped per SSE spec.
	return 0
}

// dispatch processes a completed SSE block and writes output. Returns:
//
//	-1  — caller should stop and return 0 ([DONE] sentinel)
//	 0  — success, continue
//	 1  — runtime error
func dispatch(w *bufio.Writer, eventFilter, fieldPath, eventName string, dataLines []string) int {
	dataStr := strings.Join(dataLines, "\n")

	// [DONE] is the Anthropic/OpenAI stream termination sentinel.
	// It must terminate regardless of any active --event filter.
	if dataStr == "[DONE]" {
		return -1
	}

	// Resolve the effective event name.
	name := eventName
	if name == "" {
		name = "message"
	}

	// Apply --event filter: skip non-matching events.
	if eventFilter != "" && name != eventFilter {
		return 0
	}

	// Parse data as JSON (using UseNumber to preserve large integers and avoid
	// float64 precision loss). Fall back to raw string on parse failure.
	var dataVal interface{}
	dec := json.NewDecoder(strings.NewReader(dataStr))
	dec.UseNumber()
	if err := dec.Decode(&dataVal); err != nil {
		dataVal = dataStr
	}

	if fieldPath != "" {
		// Build the root map that --field navigates from: {event, data}.
		root := map[string]interface{}{
			"event": name,
			"data":  dataVal,
		}
		val, ok := navigatePath(root, strings.Split(fieldPath, "."))
		if !ok {
			// Path not found or non-navigable type — skip silently.
			return 0
		}
		if _, err := fmt.Fprintln(w, renderValue(val)); err != nil {
			return shared.Errorf("sse: writing output: %v", err)
		}
		if err := w.Flush(); err != nil {
			return shared.Errorf("sse: writing output: %v", err)
		}
		return 0
	}

	// Emit as JSONL. Struct field order is deterministic: event then data.
	rec := sseRecord{Event: name, Data: dataVal}
	b, err := json.Marshal(rec)
	if err != nil {
		return shared.Errorf("sse: marshaling record: %v", err)
	}
	if _, err := fmt.Fprintf(w, "%s\n", b); err != nil {
		return shared.Errorf("sse: writing output: %v", err)
	}
	if err := w.Flush(); err != nil {
		return shared.Errorf("sse: writing output: %v", err)
	}
	return 0
}

// stripOneSpace strips exactly one leading space from s if present.
// Per the SSE spec: "If value's first character is U+0020 SPACE, remove it."
func stripOneSpace(s string) string {
	if len(s) > 0 && s[0] == ' ' {
		return s[1:]
	}
	return s
}

// navigatePath walks a dot-split path through a nested map[string]interface{}.
// Returns the value and true on success, or nil and false if any step fails
// (key missing, non-object encountered, or empty path).
func navigatePath(root map[string]interface{}, parts []string) (interface{}, bool) {
	var current interface{} = root
	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

// renderValue converts a navigated field value to its string representation:
//   - string → returned as-is (no quotes)
//   - json.Number → its numeric string (e.g. "42", "3.14")
//   - anything else (bool, null, array, object) → JSON-encoded representation
func renderValue(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("sse", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringP("event", "e", "", "only emit events of this type; skip all others")
	fs.StringP("field", "F", "", "extract dot-path field from the record and print as plain text")
	return fs
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: sse [flags]",
		"       <stream> | sse [flags]",
		"",
		"SSE stream parser — reads a raw text/event-stream from stdin,",
		"parses it, and emits one JSON object per event to stdout (JSONL).",
		"",
		"The [DONE] sentinel (Anthropic/OpenAI) stops parsing cleanly (exit 0).",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("sse: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
