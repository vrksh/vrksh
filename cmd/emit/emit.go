package emit

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// stdinReader allows tests to inject a custom reader for error-path testing.
// If nil, os.Stdin is used. Never set this in production code.
var stdinReader io.Reader

// levelPrefixes is the ordered list used by --parse-level detection.
// WARNING must appear before WARN so that "WARNING:" does not short-circuit
// to the shorter WARN entry.
var levelPrefixes = []struct{ prefix, level string }{
	{"ERROR", "error"},
	{"WARNING", "warn"},
	{"WARN", "warn"},
	{"INFO", "info"},
	{"DEBUG", "debug"},
}

var validLevels = map[string]bool{
	"debug": true,
	"info":  true,
	"warn":  true,
	"error": true,
}

// Run is the entry point for vrk emit. Returns 0, 1, or 2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("emit", pflag.ContinueOnError)
	var level string
	var tag string
	var msg string
	var parseLevel bool

	fs.StringVarP(&level, "level", "l", "info", "log level: debug, info, warn, error")
	fs.StringVar(&tag, "tag", "", "add tag field to every record")
	fs.StringVar(&msg, "msg", "", "override message; stdin treated as JSON to merge extra fields")
	fs.BoolVar(&parseLevel, "parse-level", false, "auto-detect level from line prefix (ERROR/WARN/WARNING/INFO/DEBUG)")

	fs.SetOutput(io.Discard)
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// Normalise and validate --level.
	level = strings.ToLower(level)
	if !validLevels[level] {
		return shared.UsageErrorf("emit: unknown level %q: must be debug, info, warn, or error", level)
	}

	// Resolve input: positional arg takes priority over stdin.
	positional := fs.Args()
	var r io.Reader
	if len(positional) > 0 {
		r = strings.NewReader(positional[0])
	} else {
		if isTerminal(int(os.Stdin.Fd())) {
			return shared.UsageErrorf("emit: no input: pipe lines to stdin")
		}
		if stdinReader != nil {
			r = stdinReader
		} else {
			r = os.Stdin
		}
	}

	w := bufio.NewWriter(os.Stdout)
	defer func() { _ = w.Flush() }()

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		rawLine := scanner.Text()
		if rawLine == "" {
			continue
		}

		lineLevel := level
		textForMsg := rawLine

		if parseLevel {
			if det, stripped := detectLevel(rawLine); det != "" {
				lineLevel = det
				textForMsg = stripped
			}
			// Unknown prefix: lineLevel stays as the --level value.
		}

		var lineMsg string
		var extra map[string]json.RawMessage
		if msg != "" {
			lineMsg = msg
			extra = tryDecodeJSON(rawLine)
		} else {
			lineMsg = textForMsg
		}

		record := buildRecord(lineLevel, tag, lineMsg, extra)
		if _, err := w.Write(record); err != nil {
			return shared.Errorf("emit: write: %v", err)
		}
		if err := w.WriteByte('\n'); err != nil {
			return shared.Errorf("emit: write: %v", err)
		}
		if err := w.Flush(); err != nil {
			return shared.Errorf("emit: flush: %v", err)
		}
	}
	if err := scanner.Err(); err != nil {
		return shared.Errorf("emit: %v", err)
	}

	return shared.ExitOK
}

// detectLevel checks whether line starts with a recognised level prefix
// (case-insensitive) followed by a word boundary (':', ' ', '\t', or end of
// string). Returns the detected level and the remaining message text with the
// prefix and separator stripped. Returns ("", line) when no prefix matches.
func detectLevel(line string) (level, stripped string) {
	upper := strings.ToUpper(line)
	for _, p := range levelPrefixes {
		if !strings.HasPrefix(upper, p.prefix) {
			continue
		}
		rest := line[len(p.prefix):]
		// Word-boundary check: prefix must be followed by ':', ' ', '\t', or EOL.
		if len(rest) > 0 {
			first := rest[0]
			if first != ':' && first != ' ' && first != '\t' {
				continue
			}
		}
		// Strip optional single colon then leading whitespace.
		rest = strings.TrimPrefix(rest, ":")
		rest = strings.TrimLeft(rest, " \t")
		return p.level, rest
	}
	return "", line
}

// tryDecodeJSON attempts to decode line as a JSON object, returning the fields
// as raw message values. Returns nil if line is not valid JSON or not an object.
// Using json.RawMessage preserves exact byte representation of values (including
// large integers) without float64 conversion.
func tryDecodeJSON(line string) map[string]json.RawMessage {
	var m map[string]json.RawMessage
	d := json.NewDecoder(strings.NewReader(line))
	d.UseNumber()
	if err := d.Decode(&m); err != nil {
		return nil
	}
	return m
}

// buildRecord constructs the JSONL record as a byte slice with deterministic
// field order: ts → level → tag (if non-empty) → msg → extra fields (alphabetical).
// Core field names (ts, level, tag, msg) in extra are silently suppressed so
// the flag-provided values always win.
func buildRecord(level, tag, msg string, extra map[string]json.RawMessage) []byte {
	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000") + "Z"

	var buf bytes.Buffer
	buf.WriteByte('{')
	appendStringField(&buf, "ts", ts)
	buf.WriteByte(',')
	appendStringField(&buf, "level", level)
	if tag != "" {
		buf.WriteByte(',')
		appendStringField(&buf, "tag", tag)
	}
	buf.WriteByte(',')
	appendStringField(&buf, "msg", msg)

	if len(extra) > 0 {
		coreFields := map[string]bool{"ts": true, "level": true, "tag": true, "msg": true}
		keys := make([]string, 0, len(extra))
		for k := range extra {
			if !coreFields[k] {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		for _, k := range keys {
			kBytes, _ := json.Marshal(k)
			buf.WriteByte(',')
			buf.Write(kBytes)
			buf.WriteByte(':')
			buf.Write(extra[k])
		}
	}

	buf.WriteByte('}')
	return buf.Bytes()
}

// appendStringField writes a JSON "key":"value" pair to buf.
func appendStringField(buf *bytes.Buffer, key, value string) {
	kBytes, _ := json.Marshal(key)
	vBytes, _ := json.Marshal(value)
	buf.Write(kBytes)
	buf.WriteByte(':')
	buf.Write(vBytes)
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk emit [flags] [message]",
		"       <input> | vrk emit [flags]",
		"",
		"Wrap stdin lines as structured JSONL log records with timestamps and levels.",
		"emit is JSONL-native — every output line is already a JSON object.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("emit: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
