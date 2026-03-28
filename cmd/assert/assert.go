// Package assert implements vrk assert — a pipeline condition check.
// Evaluates jq conditions or text checks on stdin. Passes data through on
// success (exit 0) or stops the pipeline on failure (exit 1).
package assert

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/itchyny/gojq"
	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// readAll is a var so tests can inject I/O errors.
var readAll = io.ReadAll

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "assert",
		Short: "Pipeline condition check — evaluates conditions on stdin",
		Flags: []shared.FlagMeta{
			{Name: "json", Shorthand: "j", Usage: "emit JSON result to stdout"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr on failure"},
			{Name: "message", Shorthand: "m", Usage: "custom failure message"},
			{Name: "contains", Usage: "assert stdin contains substring"},
			{Name: "matches", Usage: "assert stdin matches regex"},
		},
	})
}

// Run is the entry point for vrk assert. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("assert", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var jsonFlag, quietFlag bool
	var messageFlag, containsFlag, matchesFlag string
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON result to stdout")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr on failure")
	fs.StringVarP(&messageFlag, "message", "m", "", "custom failure message")
	fs.StringVar(&containsFlag, "contains", "", "assert stdin contains substring")
	fs.StringVar(&matchesFlag, "matches", "", "assert stdin matches regex")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quietFlag)()

	conditions := fs.Args()
	hasContains := fs.Changed("contains")
	hasMatches := fs.Changed("matches")
	hasPlainText := hasContains || hasMatches

	// No condition at all.
	if len(conditions) == 0 && !hasPlainText {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "assert: a condition expression is required",
				"code":  2,
			})
		}
		return shared.UsageErrorf("assert: a condition expression is required")
	}

	// Mode conflict: positional jq conditions + plain text flags.
	if len(conditions) > 0 && hasPlainText {
		flags := []string{}
		if hasContains {
			flags = append(flags, "--contains")
		}
		if hasMatches {
			flags = append(flags, "--matches")
		}
		msg := fmt.Sprintf("assert: cannot combine positional conditions with %s", strings.Join(flags, " and "))
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": msg,
				"code":  2,
			})
		}
		return shared.UsageErrorf("%s", msg)
	}

	// Validate regex early (before reading stdin).
	var matchesRe *regexp.Regexp
	if hasMatches {
		var err error
		matchesRe, err = regexp.Compile(matchesFlag)
		if err != nil {
			msg := fmt.Sprintf("assert: invalid regex %q: %v", matchesFlag, err)
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": msg,
					"code":  2,
				})
			}
			return shared.UsageErrorf("%s", msg)
		}
	}

	// TTY guard.
	if isTerminal(int(os.Stdin.Fd())) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "assert: no input: pipe data to stdin",
				"code":  2,
			})
		}
		return shared.UsageErrorf("assert: no input: pipe data to stdin")
	}

	// Plain text mode: read all stdin as one blob.
	if hasPlainText {
		return runPlainText(containsFlag, hasContains, matchesFlag, hasMatches, matchesRe, messageFlag, jsonFlag)
	}

	// JSON/JSONL mode: compile jq conditions, then stream line-by-line.
	queries := make([]*gojq.Code, len(conditions))
	for i, cond := range conditions {
		query, err := gojq.Parse(cond)
		if err != nil {
			msg := fmt.Sprintf("assert: invalid condition %q: %v", cond, err)
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": msg,
					"code":  2,
				})
			}
			return shared.UsageErrorf("%s", msg)
		}
		code, err := gojq.Compile(query)
		if err != nil {
			msg := fmt.Sprintf("assert: compiling condition %q: %v", cond, err)
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": msg,
					"code":  2,
				})
			}
			return shared.UsageErrorf("%s", msg)
		}
		queries[i] = code
	}

	return runJSON(conditions, queries, messageFlag, jsonFlag)
}

// runPlainText handles --contains and --matches modes.
func runPlainText(containsStr string, hasContains bool, matchesStr string, hasMatches bool, matchesRe *regexp.Regexp, message string, jsonFlag bool) int {
	raw, err := readAll(os.Stdin)
	if err != nil {
		msg := fmt.Sprintf("assert: reading stdin: %v", err)
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": msg,
				"code":  1,
			})
		}
		return shared.Errorf("%s", msg)
	}

	input := string(raw)
	// Strip one trailing newline for matching (echo adds one).
	inputTrimmed := strings.TrimSuffix(input, "\n")

	// Check --contains.
	if hasContains && !strings.Contains(inputTrimmed, containsStr) {
		failMsg := fmt.Sprintf("input does not contain %q", containsStr)
		condition := "--contains: " + containsStr
		if jsonFlag {
			return emitPlainTextJSON(false, condition, failMsg, message)
		}
		return stderrFail(condition, failMsg, message)
	}

	// Check --matches.
	if hasMatches && !matchesRe.MatchString(inputTrimmed) {
		failMsg := fmt.Sprintf("input does not match %q", matchesStr)
		condition := "--matches: " + matchesStr
		if jsonFlag {
			return emitPlainTextJSON(false, condition, failMsg, message)
		}
		return stderrFail(condition, failMsg, message)
	}

	// All checks passed.
	if jsonFlag {
		// Emit one pass result per check that was set.
		if hasContains {
			if code := emitPlainTextJSON(true, "--contains: "+containsStr, "", message); code != 0 {
				return code
			}
		}
		if hasMatches {
			if code := emitPlainTextJSON(true, "--matches: "+matchesStr, "", message); code != 0 {
				return code
			}
		}
		return 0
	}

	// Pass through byte-for-byte.
	if _, err := os.Stdout.Write(raw); err != nil {
		return shared.Errorf("assert: writing output: %v", err)
	}
	return 0
}

// runJSON handles positional jq conditions with JSONL streaming.
func runJSON(conditions []string, queries []*gojq.Code, message string, jsonFlag bool) int {
	w := bufio.NewWriter(os.Stdout)
	defer func() { _ = w.Flush() }()

	scanner := shared.ScanLines(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		// Parse JSON.
		var parsed any
		d := json.NewDecoder(strings.NewReader(string(line)))
		d.UseNumber()
		if err := d.Decode(&parsed); err != nil {
			if jsonFlag {
				_ = emitJSONResult(w, false, conditions[0], nil, "input is not valid JSON", message)
				_ = w.Flush()
				return 1
			}
			fmt.Fprintf(os.Stderr, "assert: input is not valid JSON\n")
			return 1
		}

		// Evaluate each condition.
		for i, code := range queries {
			result, ok := evalJQ(code, parsed)
			if !ok || !isTruthy(result) {
				if jsonFlag {
					_ = emitJSONResult(w, false, conditions[i], parsed, "", message)
					_ = w.Flush()
					return 1
				}
				return stderrFail(conditions[i], "", message)
			}
		}

		// All conditions passed — write original bytes.
		if jsonFlag {
			if err := emitJSONResult(w, true, conditions[0], parsed, "", ""); err != 0 {
				return err
			}
		} else {
			if _, err := w.Write(line); err != nil {
				return shared.Errorf("assert: writing output: %v", err)
			}
			if err := w.WriteByte('\n'); err != nil {
				return shared.Errorf("assert: writing output: %v", err)
			}
		}
		if err := w.Flush(); err != nil {
			return shared.Errorf("assert: writing output: %v", err)
		}
	}

	if err := scanner.Err(); err != nil {
		msg := fmt.Sprintf("assert: reading stdin: %v", err)
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": msg,
				"code":  1,
			})
		}
		return shared.Errorf("%s", msg)
	}

	return 0
}

// evalJQ runs a compiled jq query against input and returns the first result.
func evalJQ(code *gojq.Code, input any) (any, bool) {
	iter := code.Run(input)
	result, ok := iter.Next()
	if !ok {
		return nil, false
	}
	if err, isErr := result.(error); isErr {
		_ = err
		return nil, false
	}
	return result, true
}

// isTruthy implements jq truthiness: null and false are falsy, everything else is truthy.
func isTruthy(v any) bool {
	if v == nil {
		return false
	}
	if b, ok := v.(bool); ok {
		return b
	}
	return true
}

// stderrFail writes the failure message to stderr and returns exit code 1.
func stderrFail(condition, failMsg, userMessage string) int {
	if userMessage != "" {
		fmt.Fprintf(os.Stderr, "assert: assertion failed: %s (%s)\n", userMessage, condition)
	} else if failMsg != "" {
		fmt.Fprintf(os.Stderr, "assert: assertion failed: %s\n", failMsg)
	} else {
		fmt.Fprintf(os.Stderr, "assert: assertion failed: %s\n", condition)
	}
	return 1
}

// emitJSONResult writes a JSON result object for JSON mode (jq conditions).
func emitJSONResult(w *bufio.Writer, passed bool, condition string, input any, failMsg, userMessage string) int {
	obj := map[string]any{
		"passed":    passed,
		"condition": condition,
	}
	if input != nil {
		obj["input"] = input
	}
	if !passed && userMessage != "" {
		obj["message"] = userMessage
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return shared.Errorf("assert: encoding JSON: %v", err)
	}
	if _, err := w.Write(data); err != nil {
		return shared.Errorf("assert: writing output: %v", err)
	}
	if err := w.WriteByte('\n'); err != nil {
		return shared.Errorf("assert: writing output: %v", err)
	}
	return 0
}

// emitPlainTextJSON writes a JSON result object for plain text mode.
func emitPlainTextJSON(passed bool, condition, failMsg, userMessage string) int {
	obj := map[string]any{
		"passed":    passed,
		"condition": condition,
	}
	if !passed {
		if userMessage != "" {
			obj["message"] = userMessage
		} else if failMsg != "" {
			obj["message"] = failMsg
		}
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return shared.Errorf("assert: encoding JSON: %v", err)
	}
	if _, err := fmt.Fprintln(os.Stdout, string(data)); err != nil {
		return shared.Errorf("assert: writing output: %v", err)
	}
	if !passed {
		return 1
	}
	return 0
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("assert", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolP("json", "j", false, "emit JSON result to stdout")
	fs.BoolP("quiet", "q", false, "suppress stderr on failure")
	fs.StringP("message", "m", "", "custom failure message")
	fs.String("contains", "", "assert stdin contains substring")
	fs.String("matches", "", "assert stdin matches regex")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk assert [flags] <condition> [<condition>...]",
		"       echo '{\"status\":\"ok\"}' | vrk assert '.status == \"ok\"'",
		"       echo 'text' | vrk assert --contains 'expected'",
		"",
		"Pipeline condition check — evaluates conditions on stdin.",
		"Passes data through on success (exit 0), stops pipeline on failure (exit 1).",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("assert: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
