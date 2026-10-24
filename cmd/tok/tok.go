package tok

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/tokcount"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// readAll is a var so tests can inject I/O errors.
var readAll = io.ReadAll

type tokOutput struct {
	Tokens int    `json:"tokens"`
	Model  string `json:"model"`
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "tok",
		Short: "Token counter - o200k_base or cl100k_base, --check gate",
		Flags: []shared.FlagMeta{
			{Name: "json", Shorthand: "j", Usage: "emit output as JSON"},
			{Name: "check", Usage: "pass input through if within N tokens; exit 1 if over"},
			{Name: "model", Shorthand: "m", Usage: "tokenizer: an OpenAI model (gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo) or encoding (default o200k_base)"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk tok. Returns 0 (success), 1 (runtime/check
// error), or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("tok", pflag.ContinueOnError)
	var jsonFlag bool
	var check int
	var model string
	var quietFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit output as JSON")
	fs.IntVar(&check, "check", 0, "pass input through if within N tokens; exit 1 if over")
	fs.StringVarP(&model, "model", "m", "", "tokenizer: an OpenAI model (gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo) or encoding (default o200k_base)")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	// Suppress pflag's automatic printing so all output goes through shared helpers.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		// Custom message for --check without a value.
		if strings.Contains(err.Error(), "check") {
			return shared.UsageErrorf("tok: --check requires a token limit: vrk tok --check 8000")
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) - callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	enc, err := tokcount.ForModel(model)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": "tok: " + err.Error(), "code": 2})
		}
		return shared.UsageErrorf("tok: %v", err)
	}

	checkSet := fs.Changed("check")

	// TTY guard: an interactive terminal with no args is a usage error. An empty
	// stdin pipe is intentional (0 tokens) and must pass through normally.
	if len(fs.Args()) == 0 && isTerminal(int(os.Stdin.Fd())) {
		return shared.UsageErrorf("tok: no input: pipe text to stdin or pass as argument")
	}

	if checkSet {
		return runCheck(fs.Args(), check, jsonFlag, quietFlag, enc)
	}

	return runMeasure(fs.Args(), jsonFlag, enc)
}

// runMeasure is the measurement mode (no --check). Counts tokens and prints
// the count or JSON to stdout. Always exits 0 on success.
func runMeasure(args []string, jsonFlag bool, enc *tokcount.Encoding) int {
	input, err := shared.ReadInputOptional(args)
	if err != nil {
		return shared.Errorf("tok: %v", err)
	}

	count, err := enc.Count(input)
	if err != nil {
		return shared.Errorf("tok: initialising tokeniser: %v", err)
	}

	if jsonFlag {
		if err := shared.PrintJSON(tokOutput{Tokens: count, Model: enc.Name}); err != nil {
			return shared.Errorf("tok: %v", err)
		}
		return 0
	}

	if _, err := fmt.Fprintln(os.Stdout, count); err != nil {
		return shared.Errorf("tok: writing output: %v", err)
	}
	return 0
}

// runCheck is the --check mode. Reads all input, counts tokens, and either
// passes the raw input through to stdout (within limit) or exits 1 (over limit).
func runCheck(args []string, limit int, jsonFlag, quietFlag bool, enc *tokcount.Encoding) int {
	// Read raw input. Two paths:
	// - Positional args: join with space, no trailing newline.
	// - Stdin: read raw bytes, preserve byte-for-byte.
	var raw []byte
	if len(args) > 0 {
		raw = []byte(strings.Join(args, " "))
	} else {
		var err error
		raw, err = readAll(os.Stdin)
		if err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("tok: reading stdin: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("tok: reading stdin: %v", err)
		}
	}

	// Strip one trailing newline for token counting (echo adds \n).
	// Raw bytes are preserved for passthrough.
	countInput := strings.TrimSuffix(string(raw), "\n")

	count, err := enc.Count(countInput)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("tok: initialising tokeniser: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("tok: initialising tokeniser: %v", err)
	}

	if count > limit {
		// Over limit: stdout empty.
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error":  fmt.Sprintf("%d tokens exceeds limit of %d", count, limit),
				"tokens": count,
				"limit":  limit,
				"code":   1,
			})
		}
		if !quietFlag {
			fmt.Fprintf(os.Stderr, "tok: %d tokens exceeds limit of %d\n", count, limit)
		}
		return 1
	}

	// Within limit: write raw bytes to stdout.
	if _, err := os.Stdout.Write(raw); err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("tok: writing output: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("tok: writing output: %v", err)
	}
	return 0
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing - Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("tok", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolP("json", "j", false, "emit output as JSON")
	fs.Int("check", 0, "pass input through if within N tokens; exit 1 if over")
	fs.StringP("model", "m", "", "tokenizer: an OpenAI model (gpt-5, gpt-4o, gpt-4, gpt-3.5-turbo) or encoding (default o200k_base)")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: tok [flags] [text]",
		"       echo <text> | tok [flags]",
		"",
		"Token counter. Counts with o200k_base by default, the tokenizer of",
		"GPT-4o, GPT-4.1, GPT-5 and the o-series. --model gpt-4 or gpt-3.5-turbo",
		"counts with cl100k_base. Exact for OpenAI models, approximate for Claude.",
		"",
		"Without --check: prints the token count to stdout.",
		"With --check N: passes input through if within N tokens, exits 1 if over.",
		"--check reads all stdin before passing through.",
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
