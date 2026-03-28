// Package throttle implements vrk throttle — a rate limiter for pipes.
// Reads lines from stdin and re-emits them with delays to match a given rate.
package throttle

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
	"github.com/vrksh/vrksh/internal/shared/tokcount"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// sleepFn is a var so tests can skip waits without blocking.
var sleepFn = time.Sleep

// stdinReader allows tests to inject a reader that errors.
// If nil, os.Stdin is used.
var stdinReader io.Reader

// metaRecord is the --json trailing record.
type metaRecord struct {
	VRK       string `json:"_vrk"`
	Rate      string `json:"rate"`
	Lines     int    `json:"lines"`
	ElapsedMS int64  `json:"elapsed_ms"`
}

// Run is the entry point for vrk throttle. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("throttle", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var rateStr string
	var burst int
	var tokensField string
	var jsonFlag, quietFlag bool

	fs.StringVarP(&rateStr, "rate", "r", "", "rate limit: N/s or N/m (required)")
	fs.IntVar(&burst, "burst", 0, "emit first N lines without delay")
	fs.StringVar(&tokensField, "tokens-field", "", "rate by token count of a JSONL field")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `emit {"_vrk":"throttle","rate":"...","lines":N,"elapsed_ms":N} after all lines`)
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	// --rate is required.
	if rateStr == "" {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "throttle: --rate is required",
				"code":  2,
			})
		}
		return shared.UsageErrorf("throttle: --rate is required")
	}

	unitsPerSec, err := parseRate(rateStr)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("throttle: %s", err.Error()),
				"code":  2,
			})
		}
		return shared.UsageErrorf("throttle: %s", err.Error())
	}

	// TTY guard: interactive terminal with no piped input → usage error.
	if isTerminal(int(os.Stdin.Fd())) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "throttle: no input: pipe lines to stdin",
				"code":  2,
			})
		}
		return shared.UsageErrorf("throttle: no input: pipe lines to stdin")
	}

	var r io.Reader
	if stdinReader != nil {
		r = stdinReader
	} else {
		r = os.Stdin
	}

	w := bufio.NewWriter(os.Stdout)
	defer func() { _ = w.Flush() }()

	// interval is the fixed per-line delay in non-tokens mode.
	interval := time.Duration(float64(time.Second) / unitsPerSec)

	var (
		lineCount   int
		burstLeft   = burst
		nextAllowed time.Time
		startTime   time.Time
		started     bool
	)

	scanner := shared.ScanLines(r)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Record the start time on the first non-empty line.
		if !started {
			startTime = time.Now()
			nextAllowed = startTime
			started = true
		}

		// Determine this line's interval.
		var lineInterval time.Duration
		if tokensField != "" {
			tokCount, terr := extractTokenCount(line, tokensField, lineCount+1)
			if terr != nil {
				if jsonFlag {
					_ = w.Flush()
					return shared.PrintJSONError(map[string]any{
						"error": fmt.Sprintf("throttle: %s", terr.Error()),
						"code":  1,
					})
				}
				return shared.Errorf("throttle: %s", terr.Error())
			}
			if tokCount > 0 {
				lineInterval = time.Duration(float64(time.Second) * float64(tokCount) / unitsPerSec)
			}
		} else {
			lineInterval = interval
		}

		// Apply rate limiting — burst lines skip the wait.
		if burstLeft > 0 {
			burstLeft--
		} else {
			now := time.Now()
			if now.Before(nextAllowed) {
				sleepFn(nextAllowed.Sub(now))
			}
		}

		// Emit the line.
		if _, err := fmt.Fprintln(w, line); err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("throttle: writing output: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("throttle: writing output: %v", err)
		}
		if err := w.Flush(); err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("throttle: writing output: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("throttle: writing output: %v", err)
		}

		lineCount++

		// Advance nextAllowed: start from max(now, nextAllowed) so we never
		// drift backwards if we woke up early.
		now := time.Now()
		base := nextAllowed
		if now.After(base) {
			base = now
		}
		nextAllowed = base.Add(lineInterval)
	}

	if err := scanner.Err(); err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("throttle: reading stdin: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("throttle: reading stdin: %v", err)
	}

	if jsonFlag {
		var elapsedMS int64
		if started {
			elapsedMS = time.Since(startTime).Milliseconds()
		}
		meta := metaRecord{
			VRK:       "throttle",
			Rate:      rateStr,
			Lines:     lineCount,
			ElapsedMS: elapsedMS,
		}
		if err := json.NewEncoder(w).Encode(&meta); err != nil {
			return shared.Errorf("throttle: writing metadata: %v", err)
		}
	}

	return 0
}

// parseRate parses a rate string like "10/s" or "60/m".
// N must be a positive integer. Returns unitsPerSec and an error.
func parseRate(s string) (float64, error) {
	idx := strings.LastIndex(s, "/")
	if idx == -1 || idx == 0 {
		return 0, fmt.Errorf("invalid rate format: use N/s or N/m")
	}

	nStr := s[:idx]
	unit := s[idx+1:]

	n, err := strconv.Atoi(nStr)
	if err != nil {
		// If it parses as a float, give a more specific message.
		if _, ferr := strconv.ParseFloat(nStr, 64); ferr == nil {
			return 0, fmt.Errorf("rate N must be a positive integer: use e.g. 1/s or 10/m")
		}
		return 0, fmt.Errorf("invalid rate format: use N/s or N/m")
	}

	if n <= 0 {
		return 0, fmt.Errorf("rate must be > 0")
	}

	switch unit {
	case "s":
		return float64(n), nil
	case "m":
		return float64(n) / 60.0, nil
	default:
		return 0, fmt.Errorf("invalid rate format: use N/s or N/m")
	}
}

// extractTokenCount parses line as JSON, extracts the value at the dot-path
// field, and returns its token count. Returns an error if JSON is invalid,
// the field is not found, or the token encoder fails.
func extractTokenCount(line, field string, lineNum int) (int, error) {
	var obj map[string]any
	d := json.NewDecoder(strings.NewReader(line))
	d.UseNumber()
	if err := d.Decode(&obj); err != nil {
		return 0, fmt.Errorf("line %d: invalid JSON: %v", lineNum, err)
	}

	val, err := dotGet(obj, field)
	if err != nil {
		return 0, fmt.Errorf("line %d: field not found: %s", lineNum, field)
	}

	var text string
	switch v := val.(type) {
	case string:
		text = v
	case json.Number:
		text = v.String()
	default:
		text = fmt.Sprintf("%v", v)
	}

	if text == "" {
		return 0, nil
	}

	n, err := tokcount.CountTokens(text)
	if err != nil {
		return 0, fmt.Errorf("line %d: counting tokens: %v", lineNum, err)
	}
	return n, nil
}

// dotGet traverses a dot-separated path through a JSON object map.
func dotGet(obj map[string]any, path string) (any, error) {
	parts := strings.SplitN(path, ".", 2)
	val, ok := obj[parts[0]]
	if !ok {
		return nil, fmt.Errorf("key not found: %s", parts[0])
	}
	if len(parts) == 1 {
		return val, nil
	}
	nested, ok := val.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("not an object at: %s", parts[0])
	}
	return dotGet(nested, parts[1])
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("throttle", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringP("rate", "r", "", "rate limit: N/s or N/m (required)")
	fs.Int("burst", 0, "emit first N lines without delay")
	fs.String("tokens-field", "", "rate by token count of a JSONL field")
	fs.BoolP("json", "j", false, `emit {"_vrk":"throttle","rate":"...","lines":N,"elapsed_ms":N} after all lines`)
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	usage := []string{
		"usage: vrk throttle --rate N/s [flags]",
		"       seq 10 | vrk throttle --rate 5/s",
		"",
		"Rate limiter for pipes. Delays lines from stdin to match a rate constraint.",
		"Prevents hitting API rate limits when calling LLMs in a loop.",
		"",
		"flags:",
	}
	for _, l := range usage {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("throttle: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
