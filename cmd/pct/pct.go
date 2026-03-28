// Package pct implements vrk pct — a percent encoder/decoder.
// Encodes and decodes per RFC 3986. Processes input line by line.
package pct

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// readAll is a var so tests can inject I/O errors.
var readAll = io.ReadAll

// pctRecord is the JSON envelope emitted per line when --json is active.
type pctRecord struct {
	Input  string `json:"input"`
	Output string `json:"output"`
	Op     string `json:"op"`
	Mode   string `json:"mode"`
}

// Run is the entry point for vrk pct. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("pct", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var encodeFlag, decodeFlag, formFlag, jsonFlag, quietFlag bool
	fs.BoolVar(&encodeFlag, "encode", false, "percent-encode input (RFC 3986)")
	fs.BoolVar(&decodeFlag, "decode", false, "percent-decode input")
	fs.BoolVar(&formFlag, "form", false, "form encoding mode: spaces↔+ instead of %20")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit JSON object per line: {input, output, op, mode}")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr; exit codes unchanged")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quietFlag)()

	// Exactly one of --encode / --decode is required.
	if !encodeFlag && !decodeFlag {
		return shared.UsageErrorf("pct: either --encode or --decode is required")
	}
	if encodeFlag && decodeFlag {
		return shared.UsageErrorf("pct: flags --encode and --decode are mutually exclusive")
	}

	op := "encode"
	if decodeFlag {
		op = "decode"
	}
	mode := "percent"
	if formFlag {
		mode = "form"
	}

	// Collect input lines — positional args or stdin.
	var lines []string
	if args := fs.Args(); len(args) > 0 {
		lines = args
	} else {
		if isTerminal(int(os.Stdin.Fd())) {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": "pct: no input: pipe text to stdin",
					"code":  2,
				})
			}
			return shared.UsageErrorf("pct: no input: pipe text to stdin")
		}

		raw, err := readAll(os.Stdin)
		if err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("pct: reading stdin: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("pct: reading stdin: %v", err)
		}
		if len(raw) == 0 {
			return 0
		}
		// Strip exactly one trailing newline; split on remaining newlines.
		text := strings.TrimSuffix(string(raw), "\n")
		lines = strings.Split(text, "\n")
	}

	enc := json.NewEncoder(os.Stdout)
	for _, line := range lines {
		var result string

		if encodeFlag {
			if formFlag {
				result = url.QueryEscape(line)
			} else {
				result = percentEncode(line)
			}
		} else {
			var err error
			if formFlag {
				result, err = url.QueryUnescape(line)
			} else {
				result, err = url.PathUnescape(line)
			}
			if err != nil {
				if jsonFlag {
					return shared.PrintJSONError(map[string]any{
						"error": fmt.Sprintf("pct: invalid percent-encoded input: %v", err),
						"code":  1,
					})
				}
				return shared.Errorf("pct: invalid percent-encoded input: %v", err)
			}
		}

		if jsonFlag {
			if err := enc.Encode(&pctRecord{
				Input:  line,
				Output: result,
				Op:     op,
				Mode:   mode,
			}); err != nil {
				return shared.Errorf("pct: writing output: %v", err)
			}
		} else {
			if _, err := fmt.Fprintln(os.Stdout, result); err != nil {
				return shared.Errorf("pct: writing output: %v", err)
			}
		}
	}

	return 0
}

// percentEncode encodes s using strict RFC 3986 percent-encoding.
// Only unreserved characters (A-Za-z 0-9 - _ . ~) pass through unencoded.
// Iterates bytes so multi-byte UTF-8 sequences are encoded byte by byte,
// e.g. é (0xC3 0xA9) becomes %C3%A9.
func percentEncode(s string) string {
	var buf strings.Builder
	buf.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		b := s[i]
		if isUnreserved(b) {
			buf.WriteByte(b)
		} else {
			fmt.Fprintf(&buf, "%%%02X", b)
		}
	}
	return buf.String()
}

// isUnreserved reports whether b is an RFC 3986 unreserved character:
// ALPHA / DIGIT / "-" / "." / "_" / "~"
func isUnreserved(b byte) bool {
	return (b >= 'A' && b <= 'Z') ||
		(b >= 'a' && b <= 'z') ||
		(b >= '0' && b <= '9') ||
		b == '-' || b == '_' || b == '.' || b == '~'
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("pct", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.Bool("encode", false, "percent-encode input (RFC 3986)")
	fs.Bool("decode", false, "percent-decode input")
	fs.Bool("form", false, "form encoding mode: spaces↔+ instead of %20")
	fs.BoolP("json", "j", false, "emit JSON object per line: {input, output, op, mode}")
	fs.BoolP("quiet", "q", false, "suppress stderr; exit codes unchanged")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk pct [--encode|--decode] [flags]",
		"       echo 'hello world' | vrk pct --encode",
		"       echo 'hello%20world' | vrk pct --decode",
		"       vrk pct --encode 'hello world'",
		"",
		"Percent-encodes and decodes per RFC 3986.",
		"Processes input line by line; each line produces one output line.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("pct: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
