// Package urlinfo implements vrk urlinfo — a URL parser and inspector.
// Parses a URL string into its components as JSON. Pure string operation,
// no network calls. Accepts a positional argument or stdin (single or batch).
package urlinfo

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// isTerminal is a var so tests can override TTY detection without a real fd.
var isTerminal = shared.IsTerminal

// readAll is a var so tests can inject I/O errors.
var readAll = io.ReadAll

// urlRecord is the stable JSON output shape. All fields always present.
// Password is deliberately excluded — never output credentials.
type urlRecord struct {
	Scheme   string            `json:"scheme"`
	Host     string            `json:"host"`
	Port     int               `json:"port"`
	Path     string            `json:"path"`
	Query    map[string]string `json:"query"`
	Fragment string            `json:"fragment"`
	User     string            `json:"user"`
}

// metaRecord is the --json trailing envelope.
type metaRecord struct {
	VRK   string `json:"_vrk"`
	Count int    `json:"count"`
}

// Run is the entry point for vrk urlinfo. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("urlinfo", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var jsonFlag, quietFlag bool
	var fieldFlag string
	// -F matches sse's --field shorthand — consistent dot-path extraction convention
	fs.StringVarP(&fieldFlag, "field", "F", "", "extract a single field (dot-path for query params, e.g. query.page)")
	fs.BoolVarP(&jsonFlag, "json", "j", false, `append {"_vrk":"urlinfo","count":N} after all records`)
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quietFlag)()

	args := fs.Args()

	// Positional arg — single URL, no stdin needed.
	if len(args) > 0 {
		rawURL := strings.Join(args, " ")
		return runSingle(rawURL, fieldFlag, jsonFlag)
	}

	// TTY guard: interactive terminal with no piped input → usage error.
	if isTerminal(int(os.Stdin.Fd())) {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": "urlinfo: no input: pipe a URL to stdin",
				"code":  2,
			})
		}
		return shared.UsageErrorf("urlinfo: no input: pipe a URL to stdin")
	}

	// Batch mode: read all stdin, process line by line.
	raw, err := readAll(os.Stdin)
	if err != nil {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("urlinfo: reading stdin: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("urlinfo: reading stdin: %v", err)
	}

	if len(raw) == 0 {
		// Empty input is valid — exit 0, no output.
		return 0
	}

	input := strings.TrimSuffix(string(raw), "\n")
	lines := strings.Split(input, "\n")

	enc := json.NewEncoder(os.Stdout)
	count := 0

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		rec, err := parseURL(line)
		if err != nil {
			return shared.Errorf("urlinfo: invalid URL: %s", line)
		}

		if fieldFlag != "" {
			val := extractField(rec, rawQueryString(line), fieldFlag)
			if _, werr := fmt.Fprintln(os.Stdout, val); werr != nil {
				return shared.Errorf("urlinfo: writing output: %v", werr)
			}
		} else {
			if werr := enc.Encode(rec); werr != nil {
				return shared.Errorf("urlinfo: writing output: %v", werr)
			}
		}
		count++
	}

	// Emit metadata trailer only when --json is active and --field is not.
	if jsonFlag && fieldFlag == "" {
		if werr := enc.Encode(&metaRecord{VRK: "urlinfo", Count: count}); werr != nil {
			return shared.Errorf("urlinfo: writing output: %v", werr)
		}
	}

	return 0
}

// runSingle processes exactly one URL (from positional arg).
func runSingle(rawURL, fieldFlag string, jsonFlag bool) int {
	rec, err := parseURL(rawURL)
	if err != nil {
		return shared.Errorf("urlinfo: invalid URL: %s", rawURL)
	}

	if fieldFlag != "" {
		val := extractField(rec, rawQueryString(rawURL), fieldFlag)
		if _, werr := fmt.Fprintln(os.Stdout, val); werr != nil {
			return shared.Errorf("urlinfo: writing output: %v", werr)
		}
		return 0
	}

	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(rec); err != nil {
		return shared.Errorf("urlinfo: writing output: %v", err)
	}
	if jsonFlag {
		if err := enc.Encode(&metaRecord{VRK: "urlinfo", Count: 1}); err != nil {
			return shared.Errorf("urlinfo: writing output: %v", err)
		}
	}
	return 0
}

// parseURL parses a raw URL string into a urlRecord.
// Returns an error when both scheme and host are empty (not a recognisable URL).
func parseURL(rawURL string) (*urlRecord, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return nil, err
	}
	// Option A: both scheme and host empty → not a recognisable URL.
	if u.Scheme == "" && u.Host == "" {
		return nil, errors.New("not a URL")
	}

	port := 0
	if p := u.Port(); p != "" {
		if n, err := strconv.Atoi(p); err == nil {
			port = n
		}
	}

	username := ""
	if u.User != nil {
		// Password is deliberately never read — never output credentials.
		username = u.User.Username()
	}

	query := make(map[string]string)
	for k, vs := range u.Query() {
		if len(vs) > 0 {
			query[k] = vs[0]
		}
	}

	return &urlRecord{
		Scheme:   u.Scheme,
		Host:     u.Hostname(),
		Port:     port,
		Path:     u.Path,
		Query:    query,
		Fragment: u.Fragment,
		User:     username,
	}, nil
}

// rawQueryString extracts the raw query string from a URL without re-encoding.
func rawQueryString(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.RawQuery
}

// extractField returns the value for the given dot-path field from a urlRecord.
// Returns empty string for absent values — never an error.
func extractField(rec *urlRecord, rawQuery, path string) string {
	switch {
	case path == "scheme":
		return rec.Scheme
	case path == "host":
		return rec.Host
	case path == "port":
		if rec.Port == 0 {
			return ""
		}
		return strconv.Itoa(rec.Port)
	case path == "path":
		return rec.Path
	case path == "fragment":
		return rec.Fragment
	case path == "user":
		return rec.User
	case path == "query":
		return rawQuery
	case strings.HasPrefix(path, "query."):
		key := strings.TrimPrefix(path, "query.")
		return rec.Query[key]
	default:
		return ""
	}
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("urlinfo", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringP("field", "F", "", "extract a single field (dot-path for query params, e.g. query.page)")
	fs.BoolP("json", "j", false, `append {"_vrk":"urlinfo","count":N} after all records`)
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: vrk urlinfo [flags] [url]",
		"       vrk urlinfo 'https://example.com/path?x=1'",
		"       echo 'https://example.com' | vrk urlinfo",
		"       printf 'https://a.com\\nhttps://b.com\\n' | vrk urlinfo",
		"",
		"URL parser — parses a URL into its components as JSON. No network calls.",
		"Accepts a positional argument or stdin (single or multiline batch).",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("urlinfo: writing usage: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
