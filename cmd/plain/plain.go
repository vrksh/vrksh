// Package plain implements vrk plain — a markdown-to-plain-text stripper.
// It reads markdown from stdin and writes plain prose to stdout, preserving
// all content while discarding formatting syntax.
package plain

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/plaintext"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// stripMarkdown and plainTimeout are vars so tests can force a timeout.
var (
	stripMarkdown = plaintext.StripMarkdown
	plainTimeout  = 10 * time.Second
)

// plainOutput is the shape emitted by --json on success.
type plainOutput struct {
	Text        string `json:"text"`
	InputBytes  int    `json:"input_bytes"`
	OutputBytes int    `json:"output_bytes"`
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "plain",
		Short: "Markdown stripper — removes formatting, preserves content",
		Flags: []shared.FlagMeta{
			{Name: "json", Shorthand: "j", Usage: "emit JSON envelope with text and byte counts"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk plain. Returns 0 (success), 1 (runtime error),
// or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("plain", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var jsonFlag, quietFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON envelope with text and byte counts")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	var rawBytes []byte
	if args := fs.Args(); len(args) > 0 {
		rawBytes = []byte(strings.Join(args, " "))
	} else {
		// TTY guard: no piped input and nothing passed via args → usage error.
		if isTerminal(int(os.Stdin.Fd())) {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": "plain: no input: pipe markdown to stdin",
					"code":  2,
				})
			}
			return shared.UsageErrorf("plain: no input: pipe markdown to stdin")
		}

		var err error
		rawBytes, err = io.ReadAll(os.Stdin)
		if err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("plain: reading stdin: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("plain: reading stdin: %v", err)
		}
	}

	inputBytes := len(rawBytes)
	// Strip exactly one trailing newline — echo appends one, printf does not.
	input := strings.TrimSuffix(string(rawBytes), "\n")

	// Run the parse with a deadline: goldmark has quadratic paths that the
	// nesting check in plaintext does not catch. On timeout the goroutine is
	// abandoned; the process exits right after.
	done := make(chan string, 1)
	go func() { done <- stripMarkdown(input) }()
	var stripped string
	select {
	case stripped = <-done:
	case <-time.After(plainTimeout):
		msg := fmt.Sprintf("plain: markdown too complex to parse within %s", plainTimeout)
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": msg, "code": 1})
		}
		return shared.Errorf("%s", msg)
	}
	outputBytes := len(stripped)

	if jsonFlag {
		if err := shared.PrintJSON(&plainOutput{
			Text:        stripped,
			InputBytes:  inputBytes,
			OutputBytes: outputBytes,
		}); err != nil {
			return shared.Errorf("plain: %v", err)
		}
		return 0
	}

	if stripped == "" {
		return 0
	}
	if _, err := fmt.Fprintln(os.Stdout, stripped); err != nil {
		return shared.Errorf("plain: writing output: %v", err)
	}
	return 0
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("plain", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolP("json", "j", false, "emit JSON envelope with text and byte counts")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk plain [flags]",
		"       echo '**bold**' | vrk plain",
		"",
		"Markdown stripper — removes formatting syntax, preserves all content.",
		"Reads markdown from stdin. Writes plain text to stdout.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("plain: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
