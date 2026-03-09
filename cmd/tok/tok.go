package tok

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	_ "embed"

	"github.com/pkoukk/tiktoken-go"
	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"golang.org/x/term"
)

//go:embed data/cl100k_base.tiktoken
var cl100kData []byte

// embeddedBpeLoader satisfies tiktoken.BpeLoader by reading from the embedded
// cl100k_base vocab instead of downloading from a URL. The URL argument is
// intentionally ignored — the embedded data is always cl100k_base.
type embeddedBpeLoader struct{}

func (embeddedBpeLoader) LoadTiktokenBpe(_ string) (map[string]int, error) {
	ranks := make(map[string]int, 100256)
	sc := bufio.NewScanner(strings.NewReader(string(cl100kData)))
	for sc.Scan() {
		line := sc.Text()
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		if len(parts) != 2 {
			continue
		}
		token, err := base64.StdEncoding.DecodeString(parts[0])
		if err != nil {
			continue
		}
		rank, err := strconv.Atoi(parts[1])
		if err != nil {
			continue
		}
		ranks[string(token)] = rank
	}
	return ranks, sc.Err()
}

// getEncoder returns a cl100k_base encoder backed by the embedded vocabulary.
// tiktoken.SetBpeLoader is package-global state; calling it here is safe
// because tok is the only package that calls tiktoken in this binary.
func getEncoder() (*tiktoken.Tiktoken, error) {
	tiktoken.SetBpeLoader(embeddedBpeLoader{})
	return tiktoken.GetEncoding("cl100k_base")
}

// tokOutput is the shape emitted by --json.
type tokOutput struct {
	Tokens int    `json:"tokens"`
	Model  string `json:"model"`
}

// Run is the entry point for vrk tok. Returns 0 (success), 1 (runtime/budget
// error), or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := shared.StandardFlags()

	var budget int
	var model string
	fs.IntVar(&budget, "budget", 0, "exit 1 if token count exceeds N")
	fs.StringVarP(&model, "model", "m", "cl100k_base", "tokenizer model (currently only cl100k_base is supported)")

	// Suppress pflag's automatic printing so all output goes through shared helpers.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	jsonFlag, _ := fs.GetBool("json")

	// Resolve the effective model name for output. --model affects the label
	// in --json output; all models currently count with cl100k_base.
	if model == "" {
		model = "cl100k_base"
	}

	// Read input: positional args joined with a space, or full stdin.
	// tok needs the full input (io.ReadAll is correct here — not a record processor).
	// TTY detection: if stdin is a character device and no args were provided,
	// the user ran vrk tok interactively with no pipe — that is a usage error.
	var input string
	args := fs.Args()
	if len(args) > 0 {
		input = strings.Join(args, " ")
	} else {
		if term.IsTerminal(int(os.Stdin.Fd())) {
			return shared.UsageErrorf("tok: no input: pipe text to stdin or pass as argument")
		}
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return shared.Errorf("tok: reading stdin: %v", err)
		}
		input = string(b)
		// Strip exactly one trailing newline — echo appends one, printf does not.
		// Do not TrimSpace: leading/trailing whitespace is content.
		input = strings.TrimSuffix(input, "\n")
	}

	// Count tokens.
	var count int
	if input != "" {
		enc, err := getEncoder()
		if err != nil {
			return shared.Errorf("tok: initialising tokeniser: %v", err)
		}
		count = len(enc.Encode(input, nil, nil))
	}

	// Budget guard: always a hard check when --budget is set.
	// Deviation from flag-conventions.md: on tok, --budget N always exits 1 when exceeded
	// without needing --fail. The flag-conventions describe --fail as required for a hard guard,
	// but tok's primary purpose IS the budget guard — a budget that silently passes is useless.
	// --fail is accepted (it is in StandardFlags) but redundant here.
	if budget > 0 && count > budget {
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
