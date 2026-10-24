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
	"unicode/utf8"

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

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "slug",
		Short: "URL/filename-safe slug generator",
		Flags: []shared.FlagMeta{
			{Name: "separator", Usage: "word separator (default: -)"},
			{Name: "max", Usage: "max output length, truncated at word boundary (0 = unlimited)"},
			{Name: "ascii", Usage: "keep only ASCII; other scripts are dropped"},
			{Name: "json", Shorthand: "j", Usage: `emit {"input":"...","output":"..."} per line`},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr; exit codes unchanged"},
		},
	})
}

// Run is the entry point for vrk slug. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("slug", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var sepFlag string
	var maxFlag int
	var jsonFlag, quietFlag, asciiFlag bool
	fs.StringVar(&sepFlag, "separator", "-", "word separator")
	fs.IntVar(&maxFlag, "max", 0, "max output length, truncated at word boundary (0 = unlimited)")
	fs.BoolVar(&asciiFlag, "ascii", false, "keep only ASCII; other scripts are dropped")
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

	lineNo := 0
	processLine := func(line string) error {
		lineNo++
		output := slugify(line, sepFlag, maxFlag, asciiFlag)
		if output == "" && !jsonFlag {
			// One output line per input line, always: print an empty line and warn.
			if asciiFlag && strings.TrimSpace(line) != "" && slugify(line, sepFlag, maxFlag, false) != "" {
				shared.Warn("line %d: no ASCII characters, empty slug", lineNo)
			} else {
				shared.Warn("line %d: empty slug", lineNo)
			}
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

// foldTable maps Latin letters that have no NFD decomposition to ASCII.
var foldTable = map[rune]string{
	'ß': "ss", 'ẞ': "ss", 'æ': "ae", 'Æ': "ae", 'œ': "oe", 'Œ': "oe",
	'ø': "o", 'Ø': "o", 'đ': "d", 'Đ': "d", 'ł': "l", 'Ł': "l",
	'þ': "th", 'Þ': "th", 'ð': "d", 'Ð': "d",
}

// foldRune returns the ASCII letters/digits for r (accents stripped, fold
// table applied), or "" if r has no ASCII form.
func foldRune(r rune) string {
	if s, ok := foldTable[r]; ok {
		return s
	}
	var b strings.Builder
	for _, d := range norm.NFD.String(string(r)) {
		switch {
		case unicode.IsMark(d):
		case d < 128 && (unicode.IsLetter(d) || unicode.IsDigit(d)):
			b.WriteRune(unicode.ToLower(d))
		default:
			return ""
		}
	}
	return b.String()
}

// slugify converts a single line to a URL/filename-safe slug.
//
// Per rune of the NFC input: Latin letters fold to ASCII (accents stripped,
// ß→ss etc.); letters, marks and digits of other scripts are kept as-is,
// lowercased; everything else is a word boundary. With ascii set, non-ASCII
// runes are word boundaries too. If maxLen > 0 (counted in runes), the slug is
// cut at the last separator that fits; if none fits, "" is returned.
func slugify(input, sep string, maxLen int, ascii bool) string {
	var parts []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
	}
	lastASCII := false // last rune written to cur was ASCII

	for _, r := range norm.NFC.String(input) {
		if f := foldRune(r); f != "" {
			cur.WriteString(f)
			lastASCII = true
			continue
		}
		keep := !ascii && (unicode.IsLetter(r) || unicode.IsMark(r) || unicode.IsDigit(r))
		if keep && unicode.IsMark(r) && (cur.Len() == 0 || lastASCII) {
			// Stray mark on a Latin letter (or with no base): drop it.
			if cur.Len() == 0 {
				keep = false
			} else {
				continue
			}
		}
		if !keep {
			flush()
			continue
		}
		cur.WriteRune(unicode.ToLower(r))
		lastASCII = false
	}
	flush()

	result := strings.Join(parts, sep)
	if maxLen <= 0 || utf8.RuneCountInString(result) <= maxLen {
		return result
	}
	cut := 0
	for i := 0; i < maxLen; i++ {
		_, n := utf8.DecodeRuneInString(result[cut:])
		cut += n
	}
	if sep == "" {
		return result[:cut]
	}
	if strings.HasPrefix(result[cut:], sep) {
		return result[:cut] // the last word ends exactly at the limit
	}
	if idx := strings.LastIndex(result[:cut], sep); idx > 0 {
		return result[:idx]
	}
	return ""
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("slug", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("separator", "-", "word separator")
	fs.Int("max", 0, "max output length, truncated at word boundary (0 = unlimited)")
	fs.Bool("ascii", false, "keep only ASCII; other scripts are dropped")
	fs.BoolP("json", "j", false, `emit {"input":"...","output":"..."} per line`)
	fs.BoolP("quiet", "q", false, "suppress stderr; exit codes unchanged")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk slug [flags]",
		"       echo 'Hello World' | vrk slug",
		"       vrk slug 'Hello World'",
		"       printf 'Line 1\\nLine 2\\n' | vrk slug",
		"",
		"Convert text to URL/filename-safe slugs. Lowercase, hyphen-separated.",
		"Latin text folds to ASCII, other scripts are kept (--ascii to drop them).",
		"Exactly one output line per input line; empty slugs print an empty line.",
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
