// Package recase implements vrk recase — a naming convention converter.
// Reads text from stdin line by line, auto-detects the input convention,
// and converts each line to the target convention. One line in, one line out.
package recase

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode"

	"github.com/spf13/pflag"
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

// validConventions is the set of accepted --to values.
var validConventions = map[string]bool{
	"camel":     true,
	"pascal":    true,
	"snake":     true,
	"kebab":     true,
	"screaming": true,
	"title":     true,
	"lower":     true,
	"upper":     true,
}

// Run is the entry point for vrk recase. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("recase", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var toFlag string
	var jsonFlag, quietFlag bool
	fs.StringVar(&toFlag, "to", "", "target naming convention (required): camel, pascal, snake, kebab, screaming, title, lower, upper")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON object per line with input, output, from, to fields")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output; exit codes unchanged")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// --quiet: redirect stderr to /dev/null immediately so all subsequent error
	// messages are suppressed. The cleanup restores stderr when Run returns.
	defer shared.SilenceStderr(quietFlag)()

	// --to is required.
	if toFlag == "" {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "recase: --to is required",
				"code":  2,
			})
		}
		return shared.UsageErrorf("--to is required")
	}

	// Validate --to value.
	if !validConventions[toFlag] {
		msg := fmt.Sprintf("recase: unknown convention: %q; valid: camel, pascal, snake, kebab, screaming, title, lower, upper", toFlag)
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": msg, "code": 2})
		}
		return shared.UsageErrorf("%s", msg)
	}

	// TTY guard: interactive terminal with no piped input → usage error.
	if isTerminal(int(os.Stdin.Fd())) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "recase: no input: pipe text to stdin",
				"code":  2,
			})
		}
		return shared.UsageErrorf("recase: no input: pipe text to stdin")
	}

	enc := json.NewEncoder(os.Stdout)
	err := scanLines(os.Stdin, func(line string) error {
		output, from, convErr := convertLine(line, toFlag)
		if convErr != nil {
			return convErr
		}
		if jsonFlag {
			return enc.Encode(map[string]any{
				"input":  line,
				"output": output,
				"from":   from,
				"to":     toFlag,
			})
		}
		_, writeErr := fmt.Fprintln(os.Stdout, output)
		return writeErr
	})
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("recase: reading stdin: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("recase: reading stdin: %v", err)
	}

	return 0
}

// convertLine converts a single line to the target convention.
// Returns the converted string, the detected input convention name, and any error.
// For empty lines, returns ("", "lower", nil) — blank line in, blank line out.
func convertLine(line, to string) (output, from string, err error) {
	if line == "" {
		return "", "lower", nil
	}
	words, from := splitWords(line)
	output, err = joinWords(words, to)
	return
}

// splitWords splits s into lowercase words and detects the input convention.
//
// Detection priority:
//  1. Contains '_' and all-uppercase → screaming
//  2. Contains '_' → snake
//  3. Contains '-' → kebab
//  4. Contains ' ' and all-uppercase → upper
//  5. Contains ' ' and all-lowercase → lower
//  6. Contains ' ' → title
//  7. No separator, all-uppercase → upper (single-word uppercase)
//  8. No separator, all-lowercase → lower (single-word lowercase)
//  9. First char uppercase → pascal (run-based split)
//  10. First char lowercase → camel (run-based split)
func splitWords(s string) (words []string, from string) {
	switch {
	case strings.Contains(s, "_") && strings.ToUpper(s) == s:
		from = "screaming"
		for _, w := range strings.Split(s, "_") {
			if w != "" {
				words = append(words, strings.ToLower(w))
			}
		}
	case strings.Contains(s, "_"):
		from = "snake"
		for _, w := range strings.Split(s, "_") {
			if w != "" {
				words = append(words, strings.ToLower(w))
			}
		}
	case strings.Contains(s, "-"):
		from = "kebab"
		for _, w := range strings.Split(s, "-") {
			if w != "" {
				words = append(words, strings.ToLower(w))
			}
		}
	case strings.Contains(s, " "):
		switch {
		case s == strings.ToUpper(s):
			from = "upper"
		case s == strings.ToLower(s):
			from = "lower"
		default:
			from = "title"
		}
		for _, w := range strings.Split(s, " ") {
			if w != "" {
				words = append(words, strings.ToLower(w))
			}
		}
	default:
		// camelCase, PascalCase, or a single word with no separators.
		words = splitCamel(s)
		runes := []rune(s)
		if len(runes) > 0 && unicode.IsUpper(runes[0]) {
			if s == strings.ToUpper(s) {
				from = "upper"
			} else {
				from = "pascal"
			}
		} else {
			if s == strings.ToLower(s) {
				from = "lower"
			} else {
				from = "camel"
			}
		}
	}
	return
}

// splitCamel splits a camelCase or PascalCase string into lowercase words
// using a run-based state machine with acronym support.
//
// Rules:
//   - lower→upper transition: flush current lowercase word, start uppercase run
//   - uppercase run followed by lowercase: if run > 1 char, flush run[:-1] as one
//     word and start a new word with run[-1] + current lowercase char; if run == 1
//     char, merge it into the lowercase word
//   - uppercase run at end-of-string: flush entire run as one word
//   - digits: treated as neutral — attached to adjacent word, never a word boundary
//
// Known limitation: two consecutive acronyms with no separator (e.g. getHTTPSURL)
// produce a single merged word for the uppercase run ("get_httpsurl"). A dictionary
// would be required to split HTTPSURL into HTTPS + URL.
func splitCamel(s string) []string {
	if s == "" {
		return nil
	}

	var words []string
	var lowerBuf, upperBuf []rune

	flushLower := func() {
		if len(lowerBuf) > 0 {
			words = append(words, strings.ToLower(string(lowerBuf)))
			lowerBuf = lowerBuf[:0]
		}
	}
	flushUpper := func() {
		if len(upperBuf) > 0 {
			words = append(words, strings.ToLower(string(upperBuf)))
			upperBuf = upperBuf[:0]
		}
	}

	for _, r := range s {
		if unicode.IsUpper(r) {
			// Entering or continuing an uppercase run.
			// If we were building a lowercase word, flush it first.
			if len(lowerBuf) > 0 {
				flushLower()
			}
			upperBuf = append(upperBuf, r)
		} else {
			// Non-uppercase rune (lowercase letter, digit, or other).
			if len(upperBuf) > 0 {
				if len(upperBuf) == 1 {
					// Single uppercase char before lowercase: it starts a new lowercase word.
					// e.g. "W" before "orld" in "helloWorld".
					lowerBuf = append(lowerBuf, upperBuf...)
					upperBuf = upperBuf[:0]
				} else {
					// Multiple uppercase chars before lowercase: all-but-last form one word,
					// last uppercase char starts the new word.
					// e.g. "HTMLP" before "arser" in "HTMLParser" → "HTML" word + "P" starts next.
					last := upperBuf[len(upperBuf)-1]
					upperBuf = upperBuf[:len(upperBuf)-1]
					flushUpper()
					lowerBuf = append(lowerBuf, last)
				}
			}
			lowerBuf = append(lowerBuf, r)
		}
	}

	// Flush whatever remains in the buffers.
	flushLower()
	flushUpper()

	return words
}

// joinWords rejoins lowercase words into the target naming convention.
func joinWords(words []string, to string) (string, error) {
	if len(words) == 0 {
		return "", nil
	}
	switch to {
	case "snake":
		return strings.Join(words, "_"), nil
	case "screaming":
		uppers := make([]string, len(words))
		for i, w := range words {
			uppers[i] = strings.ToUpper(w)
		}
		return strings.Join(uppers, "_"), nil
	case "kebab":
		return strings.Join(words, "-"), nil
	case "camel":
		var b strings.Builder
		for i, w := range words {
			if i == 0 {
				b.WriteString(w)
			} else {
				b.WriteString(capitalize(w))
			}
		}
		return b.String(), nil
	case "pascal":
		var b strings.Builder
		for _, w := range words {
			b.WriteString(capitalize(w))
		}
		return b.String(), nil
	case "title":
		titled := make([]string, len(words))
		for i, w := range words {
			titled[i] = capitalize(w)
		}
		return strings.Join(titled, " "), nil
	case "lower":
		return strings.Join(words, " "), nil
	case "upper":
		uppers := make([]string, len(words))
		for i, w := range words {
			uppers[i] = strings.ToUpper(w)
		}
		return strings.Join(uppers, " "), nil
	default:
		return "", fmt.Errorf("unknown convention: %q", to)
	}
}

// capitalize returns s with its first Unicode rune uppercased and the rest unchanged.
func capitalize(s string) string {
	if s == "" {
		return ""
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk recase --to <convention> [flags]",
		"       echo 'hello_world' | vrk recase --to camel",
		"       printf 'foo_bar\\nhello_world\\n' | vrk recase --to pascal",
		"",
		"Naming convention converter. Reads stdin line by line, auto-detects",
		"the input convention, and converts each line to the target convention.",
		"",
		"conventions: camel, pascal, snake, kebab, screaming, title, lower, upper",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("recase: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
