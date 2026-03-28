// Package sip implements vrk sip — a stream sampler.
// Samples lines from stdin using one of four strategies: --first, --every, --count, or --sample.
// Memory-efficient: reservoir sampling uses O(N) memory, never buffers the full stream.
package sip

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"sort"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// stdinReader is a var so tests can inject I/O errors without touching os.Stdin.
// nil means "use os.Stdin" at the time Run() is called.
var stdinReader io.Reader

// reservoirItem holds one sampled line together with its original stream position
// so the reservoir can be sorted back into input order before emitting.
type reservoirItem struct {
	pos  int
	line string
}

// sipMeta is the --json metadata trailer emitted after all data lines.
type sipMeta struct {
	Vrk       string `json:"_vrk"`
	Strategy  string `json:"strategy"`
	Requested int    `json:"requested"`
	Returned  int    `json:"returned"`
	TotalSeen int    `json:"total_seen"`
}

// usageErr emits a usage error (exit 2). When --json is active the error goes to stdout
// as JSON and stderr stays empty; otherwise it is written to stderr.
func usageErr(jsonFlag bool, msg string) int {
	if jsonFlag {
		return shared.PrintJSONError(map[string]any{"error": msg, "code": 2})
	}
	return shared.UsageErrorf("%s", msg)
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "sip",
		Short: "Stream sampler — first, reservoir, every-nth, probabilistic",
		Flags: []shared.FlagMeta{
			{Name: "first", Usage: "take first N lines (deterministic)"},
			{Name: "count", Shorthand: "n", Usage: "reservoir sample of exactly N lines (random, O(N) memory)"},
			{Name: "every", Usage: "emit every Nth line (deterministic)"},
			{Name: "sample", Usage: "include each line with N% probability (approximate)"},
			{Name: "seed", Usage: "fix random seed for deterministic output (0 is valid)"},
			{Name: "json", Shorthand: "j", Usage: `append {"_vrk":"sip",...} metadata record after output`},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr; exit codes unchanged"},
		},
	})
}

// Run is the entry point for vrk sip. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("sip", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var (
		firstFlag  int
		countFlag  int
		everyFlag  int
		sampleFlag int
		seedFlag   int64
		jsonFlag   bool
		quietFlag  bool
	)

	fs.IntVar(&firstFlag, "first", 0, "take first N lines (deterministic)")
	fs.IntVarP(&countFlag, "count", "n", 0, "reservoir sample of exactly N lines (random, O(N) memory)")
	fs.IntVar(&everyFlag, "every", 0, "emit every Nth line (deterministic)")
	fs.IntVar(&sampleFlag, "sample", 0, "include each line with N% probability (approximate)")
	fs.Int64Var(&seedFlag, "seed", 0, "fix random seed for deterministic output (0 is valid)")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `append {"_vrk":"sip",...} metadata record after output`)
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr; exit codes unchanged")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quietFlag)()

	// Count which strategy flags were explicitly set.
	var strategies []string
	if fs.Changed("first") {
		strategies = append(strategies, "first")
	}
	if fs.Changed("count") {
		strategies = append(strategies, "count")
	}
	if fs.Changed("every") {
		strategies = append(strategies, "every")
	}
	if fs.Changed("sample") {
		strategies = append(strategies, "sample")
	}

	if len(strategies) == 0 {
		return usageErr(jsonFlag, "sip: a sampling flag is required")
	}
	if len(strategies) > 1 {
		return usageErr(jsonFlag, "sip: only one sampling strategy may be used at a time")
	}

	strategy := strategies[0]

	// Validate ranges for the chosen strategy.
	switch strategy {
	case "first":
		if firstFlag < 1 {
			return usageErr(jsonFlag, "sip: --first must be >= 1")
		}
	case "count":
		if countFlag < 1 {
			return usageErr(jsonFlag, "sip: --count must be >= 1")
		}
	case "every":
		if everyFlag < 1 {
			return usageErr(jsonFlag, "sip: --every must be >= 1")
		}
	case "sample":
		if sampleFlag <= 0 {
			return usageErr(jsonFlag, "sip: --sample must be > 0")
		}
		if sampleFlag > 100 {
			return usageErr(jsonFlag, "sip: --sample must be <= 100")
		}
	}

	// TTY guard: interactive terminal with no piped input → usage error.
	if isTerminal(int(os.Stdin.Fd())) {
		return usageErr(jsonFlag, "sip: no input: pipe lines to stdin")
	}

	// Build the RNG. Seed 0 is valid — use fs.Changed, not a zero-value sentinel.
	var rng *rand.Rand
	if fs.Changed("seed") {
		rng = rand.New(rand.NewSource(seedFlag)) //nolint:gosec — intentionally non-crypto; seed is a user-facing feature
	} else {
		rng = rand.New(rand.NewSource(time.Now().UnixNano())) //nolint:gosec
	}

	// Choose the input reader. nil means "use os.Stdin" (default); tests override via stdinReader.
	r := stdinReader
	if r == nil {
		r = os.Stdin
	}
	scanner := shared.ScanLines(r)

	w := bufio.NewWriter(os.Stdout)

	// Strategy functions return scan and write errors separately so Run() can
	// emit accurate error messages ("reading stdin" vs "writing output").
	var returned, totalSeen int
	var scanErr, writeErr error

	switch strategy {
	case "first":
		returned, totalSeen, scanErr, writeErr = runFirst(scanner, firstFlag, w, jsonFlag)
	case "every":
		returned, totalSeen, scanErr, writeErr = runEvery(scanner, everyFlag, w)
	case "count":
		returned, totalSeen, scanErr, writeErr = runReservoir(scanner, countFlag, rng, w)
	case "sample":
		returned, totalSeen, scanErr, writeErr = runSample(scanner, sampleFlag, rng, w)
	}

	// Flush buffered data. If a write already failed inside a strategy function,
	// skip the flush (the pipe is likely broken) but still report the original error.
	if writeErr == nil {
		if flushErr := w.Flush(); flushErr != nil {
			writeErr = flushErr
		}
	}

	if writeErr != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("sip: writing output: %v", writeErr),
				"code":  1,
			})
		}
		return shared.Errorf("sip: writing output: %v", writeErr)
	}

	if scanErr != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("sip: reading stdin: %v", scanErr),
				"code":  1,
			})
		}
		return shared.Errorf("sip: reading stdin: %v", scanErr)
	}

	if jsonFlag {
		meta := &sipMeta{
			Vrk:       "sip",
			Strategy:  strategyLabel(strategy),
			Requested: requestedVal(strategy, firstFlag, countFlag, everyFlag, sampleFlag),
			Returned:  returned,
			TotalSeen: totalSeen,
		}
		if err := json.NewEncoder(os.Stdout).Encode(meta); err != nil {
			return shared.Errorf("sip: writing output: %v", err)
		}
	}

	return 0
}

// strategyLabel maps internal strategy names to the JSON "strategy" field value.
func strategyLabel(strategy string) string {
	if strategy == "count" {
		return "reservoir"
	}
	return strategy
}

// requestedVal returns the integer value of the chosen strategy flag.
func requestedVal(strategy string, first, count, every, sample int) int {
	switch strategy {
	case "first":
		return first
	case "count":
		return count
	case "every":
		return every
	case "sample":
		return sample
	default:
		panic("unreachable: unknown strategy " + strategy)
	}
}

// runFirst emits the first n non-empty lines. When jsonActive is true the full
// stream is drained to compute total_seen for the metadata trailer; when false
// scanning stops immediately after n lines — behaves like head(1).
// Returns (returned, totalSeen, scanErr, writeErr).
func runFirst(s *bufio.Scanner, n int, w *bufio.Writer, jsonActive bool) (returned, totalSeen int, scanErr, writeErr error) {
	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}
		totalSeen++
		if returned < n {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return returned, totalSeen, nil, err
			}
			returned++
		} else if !jsonActive {
			// No metadata needed — stop reading as soon as we have our N lines.
			return returned, totalSeen, nil, nil
		}
	}
	if err := s.Err(); err != nil {
		return returned, totalSeen, err, nil
	}
	return returned, totalSeen, nil, nil
}

// runEvery emits every nth non-empty line. The full stream is always scanned
// because Nth lines are spread throughout; total_seen is a free side effect.
// Returns (returned, totalSeen, scanErr, writeErr).
func runEvery(s *bufio.Scanner, n int, w *bufio.Writer) (returned, totalSeen int, scanErr, writeErr error) {
	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}
		totalSeen++
		if totalSeen%n == 0 {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return returned, totalSeen, nil, err
			}
			returned++
		}
	}
	if err := s.Err(); err != nil {
		return returned, totalSeen, err, nil
	}
	return returned, totalSeen, nil, nil
}

// runReservoir samples exactly n lines using Vitter's Algorithm R.
//
// For each non-empty line i (1-indexed):
//   - If i <= n: add to reservoir.
//   - Else: j = rng.Intn(i) (random index 0 to i-1).
//     If j < n (probability n/i): replace reservoir[j] with the current line.
//     NOT reservoir[rng.Intn(n)] — that would be uniform replacement, not Vitter's.
//
// After the stream ends, the reservoir is sorted by original position so output
// reflects input order. Returns (returned, totalSeen, scanErr, writeErr).
func runReservoir(s *bufio.Scanner, n int, rng *rand.Rand, w *bufio.Writer) (returned, totalSeen int, scanErr, writeErr error) {
	reservoir := make([]reservoirItem, 0, n)

	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}
		totalSeen++
		if totalSeen <= n {
			reservoir = append(reservoir, reservoirItem{pos: totalSeen, line: line})
		} else {
			j := rng.Intn(totalSeen) // random index 0 to totalSeen-1
			if j < n {               // probability n/totalSeen of replacement
				reservoir[j] = reservoirItem{pos: totalSeen, line: line}
			}
		}
	}
	if err := s.Err(); err != nil {
		return 0, totalSeen, err, nil
	}

	// Restore input order before emitting.
	if len(reservoir) > 1 {
		sort.Slice(reservoir, func(a, b int) bool {
			return reservoir[a].pos < reservoir[b].pos
		})
	}

	for _, item := range reservoir {
		if _, err := fmt.Fprintln(w, item.line); err != nil {
			return returned, totalSeen, nil, err
		}
		returned++
	}
	return returned, totalSeen, nil, nil
}

// runSample includes each non-empty line with pct% probability by drawing
// rng.Float64() < float64(pct)/100 for each line. --sample 100 always passes
// all lines because Float64 returns [0, 1). Returns (returned, totalSeen, scanErr, writeErr).
func runSample(s *bufio.Scanner, pct int, rng *rand.Rand, w *bufio.Writer) (returned, totalSeen int, scanErr, writeErr error) {
	threshold := float64(pct) / 100.0
	for s.Scan() {
		line := s.Text()
		if line == "" {
			continue
		}
		totalSeen++
		if rng.Float64() < threshold {
			if _, err := fmt.Fprintln(w, line); err != nil {
				return returned, totalSeen, nil, err
			}
			returned++
		}
	}
	if err := s.Err(); err != nil {
		return returned, totalSeen, err, nil
	}
	return returned, totalSeen, nil, nil
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("sip", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Int("first", 0, "take first N lines (deterministic)")
	fs.IntP("count", "n", 0, "reservoir sample of exactly N lines (random, O(N) memory)")
	fs.Int("every", 0, "emit every Nth line (deterministic)")
	fs.Int("sample", 0, "include each line with N% probability (approximate)")
	fs.Int64("seed", 0, "fix random seed for deterministic output (0 is valid)")
	fs.BoolP("json", "j", false, `append {"_vrk":"sip",...} metadata record after output`)
	fs.BoolP("quiet", "q", false, "suppress stderr; exit codes unchanged")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk sip [flags]",
		"       seq 100 | vrk sip --first 10",
		"       cat data.jsonl | vrk sip --count 1000",
		"",
		"Stream sampler — samples lines from stdin using one of four strategies.",
		"Memory-efficient: --count uses O(N) memory regardless of stream length.",
		"",
		"Exactly one of --first, --count (-n), --every, --sample must be specified.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("sip: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
