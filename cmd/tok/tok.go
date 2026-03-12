package tok

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/tokcount"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// tokOutput is the shape emitted by --json.
type tokOutput struct {
	Tokens int    `json:"tokens"`
	Model  string `json:"model"`
}

// Run is the entry point for vrk tok. Returns 0 (success), 1 (runtime/budget
// error), or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("tok", pflag.ContinueOnError)
	var jsonFlag bool
	var budget int
	var model string
	var quietFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit output as JSON")
	fs.IntVar(&budget, "budget", 0, "exit 1 if token count exceeds N")
	fs.StringVarP(&model, "model", "m", "cl100k_base", "tokenizer model (currently only cl100k_base is supported)")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	// Suppress pflag's automatic printing so all output goes through shared helpers.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	// Resolve the effective model name for output. --model affects the label
	// in --json output; all models currently count with cl100k_base.
	if model == "" {
		model = "cl100k_base"
	}

	// TTY guard: an interactive terminal with no args is a usage error. An empty
	// stdin pipe is intentional (0 tokens) and must pass through normally.
	// ReadInputOptional can't distinguish the two — so we check explicitly first.
	if len(fs.Args()) == 0 && isTerminal(int(os.Stdin.Fd())) {
		return shared.UsageErrorf("tok: no input: pipe text to stdin or pass as argument")
	}

	// Read input: positional args joined with a space, or full stdin.
	// tok needs the full text before counting — ReadInputOptional handles the
	// one-trailing-newline strip and returns "" for an empty pipe (→ 0 tokens).
	input, err := shared.ReadInputOptional(fs.Args())
	if err != nil {
		return shared.Errorf("tok: %v", err)
	}

	// Count tokens.
	count, err := tokcount.CountTokens(input)
	if err != nil {
		return shared.Errorf("tok: initialising tokeniser: %v", err)
	}

	// Budget guard: always a hard check. tok does not have a --fail flag;
	// --budget alone is the guard. A budget that silently passes is useless.
	if budget > 0 && count > budget {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("tok: %d tokens exceeds budget of %d", count, budget),
				"code":  1,
			})
		}
		return shared.Errorf("tok: %d tokens exceeds budget of %d", count, budget)
	}

	// Output.
	if jsonFlag {
		if err := shared.PrintJSON(tokOutput{Tokens: count, Model: model}); err != nil {
			return shared.Errorf("tok: %v", err)
		}
		return 0
	}

	if _, err := fmt.Fprintln(os.Stdout, count); err != nil {
		return shared.Errorf("tok: writing output: %v", err)
	}
	return 0
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: tok [flags] [text]",
		"       echo <text> | tok [flags]",
		"",
		"Token counter — counts tokens in stdin using cl100k_base (exact for GPT-4,",
		"~95% accurate for Claude). Optionally fails if over a token budget.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("tok: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
