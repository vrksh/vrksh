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

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/plaintext"
	"golang.org/x/term"
)

// isTerminal wraps term.IsTerminal so tests can override it without touching
// the real file descriptor.
var isTerminal = func(fd int) bool {
	return term.IsTerminal(fd)
}

// plainOutput is the shape emitted by --json on success.
type plainOutput struct {
	Text        string `json:"text"`
	InputBytes  int    `json:"input_bytes"`
	OutputBytes int    `json:"output_bytes"`
}

// Run is the entry point for vrk plain. Returns 0 (success), 1 (runtime error),
// or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("plain", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var jsonFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON envelope with text and byte counts")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

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

	rawBytes, err := io.ReadAll(os.Stdin)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("plain: reading stdin: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("plain: reading stdin: %v", err)
	}

	inputBytes := len(rawBytes)
	// Strip exactly one trailing newline — echo appends one, printf does not.
	input := strings.TrimSuffix(string(rawBytes), "\n")

	stripped := plaintext.StripMarkdown(input)
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
