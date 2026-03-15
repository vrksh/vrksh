// Package jsonl implements vrk jsonl — a JSON array ↔ JSONL converter.
// Splits a JSON array into one record per line (default) or collects JSONL
// lines into a single JSON array (--collect). Uses streaming JSON decoding
// in split mode so arrays larger than memory are handled safely.
package jsonl

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// stdinReader is a var so tests can inject I/O errors. nil means use os.Stdin.
var stdinReader io.Reader

// Run is the entry point for vrk jsonl. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("jsonl", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var collectFlag, jsonFlag bool
	fs.BoolVarP(&collectFlag, "collect", "c", false, "collect JSONL lines into a JSON array")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `append {"_vrk":"jsonl","count":N} after all records (split mode only)`)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// Positional arg: use it as the input reader, skip stdin entirely.
	if args := fs.Args(); len(args) > 0 {
		r := strings.NewReader(args[0])
		if collectFlag {
			return runCollect(r)
		}
		return runSplit(r, jsonFlag)
	}

	// TTY guard: interactive terminal with no piped input → usage error.
	if isTerminal(int(os.Stdin.Fd())) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "jsonl: no input: pipe JSON to stdin",
				"code":  2,
			})
		}
		return shared.UsageErrorf("jsonl: no input: pipe JSON to stdin")
	}

	r := io.Reader(os.Stdin)
	if stdinReader != nil {
		r = stdinReader
	}

	if collectFlag {
		return runCollect(r)
	}
	return runSplit(r, jsonFlag)
}

// runSplit reads a JSON array from r and emits each element as a JSONL line.
// Uses json.Decoder for streaming so arrays larger than memory are handled.
func runSplit(r io.Reader, jsonFlag bool) int {
	d := json.NewDecoder(r)
	d.UseNumber()

	// Read the first token to determine what we have.
	tok, err := d.Token()
	if err == io.EOF {
		// Empty or whitespace-only input — exit 0, no output.
		return 0
	}
	if err != nil {
		var syntaxErr *json.SyntaxError
		if errors.As(err, &syntaxErr) {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{"error": "jsonl: invalid JSON", "code": 1})
			}
			return shared.Errorf("jsonl: invalid JSON")
		}
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("jsonl: reading stdin: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("jsonl: reading stdin: %v", err)
	}

	// Expect the opening '[' of a JSON array.
	delim, ok := tok.(json.Delim)
	if !ok || delim != '[' {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "jsonl: input is not a JSON array; for JSONL → array use --collect",
				"code":  1,
			})
		}
		return shared.Errorf("jsonl: input is not a JSON array; for JSONL → array use --collect")
	}

	w := bufio.NewWriter(os.Stdout)
	defer func() { _ = w.Flush() }()
	count := 0
	for d.More() {
		var v any
		if err := d.Decode(&v); err != nil {
			if jsonFlag {
				return shared.PrintJSONError(map[string]any{
					"error": fmt.Sprintf("jsonl: invalid JSON: %v", err),
					"code":  1,
				})
			}
			return shared.Errorf("jsonl: invalid JSON: %v", err)
		}
		b, err := json.Marshal(v)
		if err != nil {
			return shared.Errorf("jsonl: encoding record: %v", err)
		}
		if _, err := w.Write(b); err != nil {
			return shared.Errorf("jsonl: writing output: %v", err)
		}
		if err := w.WriteByte('\n'); err != nil {
			return shared.Errorf("jsonl: writing output: %v", err)
		}
		count++
	}

	if jsonFlag {
		meta := map[string]any{"_vrk": "jsonl", "count": count}
		b, _ := json.Marshal(meta)
		_, _ = w.Write(b)
		_ = w.WriteByte('\n')
	}

	return 0
}

// runCollect reads JSONL lines from r and emits a single JSON array.
// Empty stdin → outputs []. Invalid JSON on any line → exit 1 with line number.
func runCollect(r io.Reader) int {
	sc := shared.ScanLines(r)
	var records []json.RawMessage
	lineNum := 0
	for sc.Scan() {
		lineNum++
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		d := json.NewDecoder(strings.NewReader(line))
		d.UseNumber()
		var v json.RawMessage
		if err := d.Decode(&v); err != nil {
			return shared.Errorf("jsonl: invalid JSON on line %d", lineNum)
		}
		records = append(records, v)
	}
	if err := sc.Err(); err != nil {
		return shared.Errorf("jsonl: reading stdin: %v", err)
	}

	if records == nil {
		records = []json.RawMessage{}
	}
	b, err := json.Marshal(records)
	if err != nil {
		return shared.Errorf("jsonl: encoding output: %v", err)
	}
	if _, err := fmt.Fprintf(os.Stdout, "%s\n", b); err != nil {
		return shared.Errorf("jsonl: writing output: %v", err)
	}
	return 0
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk jsonl [flags]",
		`       echo '[{"a":1},{"b":2}]' | vrk jsonl`,
		`       printf '{"a":1}\n{"b":2}\n' | vrk jsonl --collect`,
		"",
		"Converts a JSON array to JSONL (one record per line), or collects JSONL into an array.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("jsonl: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
