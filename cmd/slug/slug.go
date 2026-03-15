// Package slug implements vrk slug — a URL/filename-safe slug generator.
// Reads text from stdin line by line, normalises unicode to ASCII,
// lowercases, replaces non-alphanumeric runs with hyphens, and emits one
// slug per line to stdout.
package slug

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/spf13/pflag"
	"golang.org/x/text/unicode/norm"

	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// scanLines is a var so tests can inject I/O errors.
// It reads lines from r using a bufio.Scanner and calls fn for each line.
// Returns the first non-nil error from either fn or the scanner itself.
var scanLines = func(r io.Reader, fn func(string) error) error {
	sc := shared.ScanLines(r)
	for sc.Scan() {
		if err := fn(sc.Text()); err != nil {
			return err
		}
	}
	return sc.Err()
}

// Run is the entry point for vrk slug. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("slug", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var sepFlag string
	var maxFlag int
	var jsonFlag, quietFlag bool
	fs.StringVar(&sepFlag, "separator", "-", "word separator (default: -)")
	fs.IntVar(&maxFlag, "max", 0, "max output length, truncated at word boundary (0 = unlimited)")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `emit {"input":"...","output":"..."} per line`)
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr; exit codes unchanged")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quietFlag)()

	posArgs := fs.Args()

	// TTY guard: only applies when there are no positional args.
	if len(posArgs) == 0 && isTerminal(int(os.Stdin.Fd())) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "slug: no input: pipe text to stdin",
				"code":  2,
			})
		}
		return shared.UsageErrorf("slug: no input: pipe text to stdin")
	}

	enc := json.NewEncoder(os.Stdout)

	processLine := func(line string) error {
		output := slugify(line, sepFlag, maxFlag)
		if output == "" {
			// Empty slug (empty input or all-punctuation) — suppress output.
			return nil
		}
		if jsonFlag {
			return enc.Encode(map[string]any{
				"input":  line,
				"output": output,
			})
		}
		_, err := fmt.Fprintln(os.Stdout, output)
		return err
	}

	// Positional args: each arg is treated as one input line.
	if len(posArgs) > 0 {
		for _, line := range posArgs {
			if err := processLine(line); err != nil {
				if jsonFlag {
					return shared.PrintJSONError(map[string]any{
						"error": fmt.Sprintf("slug: writing output: %v", err),
						"code":  1,
					})
				}
				return shared.Errorf("slug: writing output: %v", err)
			}
		}
		return 0
	}

	// Stdin path.
	err := scanLines(os.Stdin, processLine)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("slug: reading stdin: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("slug: reading stdin: %v", err)
	}

	return 0
}

// slugify converts a single line to a URL/filename-safe slug.
//
// Pipeline:
//  1. NFD normalisation — decomposes accented chars into base + combining marks
//     (e.g. "é" → "e" + U+0301 combining-acute-accent)
//  2. Filter runes — keep lowercase ASCII [a-z0-9]; drop combining marks (Mn);
//     everything else marks a word boundary
//  3. Join words with sep
//  4. If maxLen > 0, truncate at the last sep at or before maxLen;
//     if no sep exists in range, return ""
func slugify(input, sep string, maxLen int) string {
	// NFD decomposes accented characters into base letter + combining marks.
	s := norm.NFD.String(input)

	// Walk runes, building words (runs of [a-z0-9]) separated by everything else.
	var parts []string
	var cur strings.Builder

	for _, r := range s {
		lo := unicode.ToLower(r)
		switch {
		case lo >= 'a' && lo <= 'z':
			cur.WriteRune(lo)
		case lo >= '0' && lo <= '9':
			cur.WriteRune(lo)
		case unicode.Is(unicode.Mn, r):
			// Combining mark (diacritic) — drop silently; the base letter was
			// already written to cur in the previous iteration.
		default:
			// Any other character (space, punctuation, non-Latin, etc.) is a
			// word boundary. Flush the current word, if any.
			if cur.Len() > 0 {
				parts = append(parts, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		parts = append(parts, cur.String())
	}

	result := strings.Join(parts, sep)
	if result == "" {
		return ""
	}

	// Apply --max: truncate at the last word boundary at or before maxLen.
	if maxLen > 0 && len(result) > maxLen {
		truncated := result[:maxLen]
		if sep == "" {
			// No separator means no word boundary; hard-truncate at maxLen.
			return truncated
		}
		idx := strings.LastIndex(truncated, sep)
		if idx > 0 {
			return truncated[:idx]
		}
		// No separator found in range — the first word is longer than maxLen.
		return ""
	}

	return result
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk slug [flags]",
		"       echo 'Hello World' | vrk slug",
		"       vrk slug 'Hello World'",
		"       printf 'Line 1\\nLine 2\\n' | vrk slug",
		"",
		"Convert text to URL/filename-safe slugs. Lowercase, hyphen-separated,",
		"unicode normalised to ASCII. One slug per input line.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("slug: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
