package mask

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// stdinReader is the io.Reader that Run() scans. When nil, Run() falls back to
// os.Stdin at call time. Tests set this to inject I/O errors without touching
// the OS-level stdin pipe.
var stdinReader io.Reader

// builtinPattern is a compiled pattern with a short name for patterns_matched reporting.
type builtinPattern struct {
	name string
	re   *regexp.Regexp
}

// builtins are applied in order before entropy scanning. Each regex has exactly
// one capture group (the key prefix to preserve); the value after the group is
// replaced with [REDACTED]. Compiled once at package init — never modified.
var builtins = []builtinPattern{
	{"bearer", regexp.MustCompile(`(?i)(bearer\s+)\S+`)},
	{"password", regexp.MustCompile(`(?i)(password\s*[=:]\s*)\S+`)},
	{"secret", regexp.MustCompile(`(?i)(secret\s*[=:]\s*)\S+`)},
	{"api_key", regexp.MustCompile(`(?i)(api[_-]?key\s*[=:]\s*)\S+`)},
	{"token", regexp.MustCompile(`(?i)(token\s*[=:]\s*)\S+`)},
}

// compiledCustom holds a user-supplied pattern compiled to a regexp.
// name is the literal regex string provided by the user — used in patterns_matched.
type compiledCustom struct {
	re   *regexp.Regexp
	name string
}

// tokenRe matches contiguous non-whitespace sequences for entropy scanning.
// Used to iterate tokens without destroying inter-token whitespace.
var tokenRe = regexp.MustCompile(`\S+`)

// maskMeta is the JSON shape appended to stdout after EOF when --json is active.
type maskMeta struct {
	Vrk             string   `json:"_vrk"`
	Lines           int      `json:"lines"`
	Redacted        int      `json:"redacted"`
	PatternsMatched []string `json:"patterns_matched"`
}

// Run is the entry point for vrk mask. Returns 0, 1, or 2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("mask", pflag.ContinueOnError)
	var patterns []string
	var entropyThreshold float64
	var jsonOut bool

	fs.StringArrayVar(&patterns, "pattern", nil, "additional pattern regex (repeatable)")
	fs.Float64Var(&entropyThreshold, "entropy", 4.0, "Shannon entropy threshold (default 4.0; lower = more aggressive)")
	fs.BoolVarP(&jsonOut, "json", "j", false, "append metadata JSON record after text output")

	// Suppress pflag's automatic error printing so all output goes through shared helpers.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("mask: %s", err)
	}

	// Compile all --pattern regexes before touching stdin so that an invalid
	// regex exits 2 immediately without consuming any input.
	var customs []compiledCustom
	for _, p := range patterns {
		re, err := regexp.Compile(p)
		if err != nil {
			return shared.UsageErrorf("mask: invalid pattern %q: %s", p, err)
		}
		customs = append(customs, compiledCustom{re: re, name: p})
	}

	// TTY check: if stdin is an interactive terminal, the user ran vrk mask with
	// no piped input — that is a usage error.
	if isTerminal(int(os.Stdin.Fd())) {
		if jsonOut {
			return shared.PrintJSONError(map[string]any{
				"error": "mask: no input: pipe text to stdin",
				"code":  2,
			})
		}
		return shared.UsageErrorf("mask: no input: pipe text to stdin")
	}

	w := bufio.NewWriter(os.Stdout)
	defer func() { _ = w.Flush() }()

	// Use the package-level stdinReader if set (test injection); otherwise os.Stdin.
	r := stdinReader
	if r == nil {
		r = os.Stdin
	}
	scanner := bufio.NewScanner(r)

	var totalLines, redactedLines int
	firedPatterns := map[string]bool{}

	for scanner.Scan() {
		line := scanner.Text()
		totalLines++

		out, fired := redactLine(line, customs, entropyThreshold)
		for _, name := range fired {
			firedPatterns[name] = true
		}
		if out != line {
			redactedLines++
		}

		_, _ = fmt.Fprintln(w, out)
		_ = w.Flush()
	}

	if err := scanner.Err(); err != nil {
		if jsonOut {
			_ = w.Flush()
			return shared.PrintJSONError(map[string]any{
				"error": err.Error(),
				"code":  1,
			})
		}
		// Without --json, mask is a filter — silently stop on I/O error.
		return 0
	}

	if jsonOut {
		meta := maskMeta{
			Vrk:             "mask",
			Lines:           totalLines,
			Redacted:        redactedLines,
			PatternsMatched: buildMatchedList(firedPatterns, customs),
		}
		enc := json.NewEncoder(w)
		_ = enc.Encode(meta)
	}

	return 0
}

// redactLine applies built-in patterns, custom patterns, and entropy scanning
// (in that order) to a single input line. Returns the (possibly redacted) line
// and the deduplicated list of pattern names that fired.
func redactLine(line string, customs []compiledCustom, threshold float64) (string, []string) {
	out := line
	var fired []string
	seen := map[string]bool{}

	// Apply built-in patterns. Each preserves its first capture group (the key
	// prefix) and replaces everything after with [REDACTED].
	for _, bp := range builtins {
		replaced := bp.re.ReplaceAllString(out, "${1}[REDACTED]")
		if replaced != out {
			if !seen[bp.name] {
				fired = append(fired, bp.name)
				seen[bp.name] = true
			}
			out = replaced
		}
	}

	// Apply custom --pattern regexes. The entire match is replaced with [REDACTED].
	for _, cp := range customs {
		replaced := cp.re.ReplaceAllString(out, "[REDACTED]")
		if replaced != out {
			if !seen[cp.name] {
				fired = append(fired, cp.name)
				seen[cp.name] = true
			}
			out = replaced
		}
	}

	// Entropy scan: check each whitespace-delimited token that is ≥ 8 chars and
	// does not already contain [REDACTED] (which would indicate it was already
	// handled by a pattern, or is itself a placeholder from a prior substitution).
	out = tokenRe.ReplaceAllStringFunc(out, func(tok string) string {
		if strings.Contains(tok, "[REDACTED]") {
			return tok
		}
		if len(tok) < 8 {
			return tok
		}
		if shannonEntropy(tok) >= threshold {
			if !seen["entropy"] {
				fired = append(fired, "entropy")
				seen["entropy"] = true
			}
			return "[REDACTED]"
		}
		return tok
	})

	return out, fired
}

// shannonEntropy calculates the Shannon entropy of s in bits per character.
// H = -sum_over_unique_chars(freq * log2(freq))
func shannonEntropy(s string) float64 {
	if len(s) == 0 {
		return 0
	}
	freq := make(map[rune]int)
	for _, c := range s {
		freq[c]++
	}
	var total int
	for _, count := range freq {
		total += count
	}
	n := float64(total)
	var h float64
	for _, count := range freq {
		p := float64(count) / n
		h -= p * math.Log2(p)
	}
	return h
}

// buildMatchedList returns a deduplicated, ordered list of pattern names that
// fired across the run. Order: builtins in declaration order, then "entropy",
// then custom patterns in argument order.
func buildMatchedList(fired map[string]bool, customs []compiledCustom) []string {
	var matched []string
	for _, b := range builtins {
		if fired[b.name] {
			matched = append(matched, b.name)
		}
	}
	if fired["entropy"] {
		matched = append(matched, "entropy")
	}
	seen := map[string]bool{}
	for _, c := range customs {
		if fired[c.name] && !seen[c.name] {
			matched = append(matched, c.name)
			seen[c.name] = true
		}
	}
	if matched == nil {
		matched = []string{}
	}
	return matched
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: mask [flags]",
		"       <text> | mask [flags]",
		"",
		"Secret redactor — replaces secrets with [REDACTED] using pattern matching",
		"and Shannon entropy analysis. Streaming line-by-line. Best-effort only.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("mask: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
