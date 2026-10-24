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
	// Known credential prefixes. The empty group keeps the ${1} replacement
	// uniform. These run on the whole line, so a key hidden inside a URL, path,
	// or image ref (which entropy scanning skips) is still caught.
	{"aws_key", regexp.MustCompile(`()\b(?:AKIA|ASIA)[0-9A-Z]{16}\b`)},
	{"openai_anthropic", regexp.MustCompile(`()\bsk-[A-Za-z0-9_-]{20,}`)},
	{"stripe", regexp.MustCompile(`()\b[rs]k_(?:live|test)_[A-Za-z0-9]{16,}`)},
	{"github", regexp.MustCompile(`()\b(?:gh[pousr]_[A-Za-z0-9]{30,}|github_pat_[A-Za-z0-9_]{22,})`)},
	{"slack", regexp.MustCompile(`()\bxox[abposr]-[A-Za-z0-9-]{10,}`)},
	{"google", regexp.MustCompile(`()\bAIza[0-9A-Za-z_-]{35}`)},
	{"huggingface", regexp.MustCompile(`()\bhf_[A-Za-z0-9]{30,}`)},
	{"npm", regexp.MustCompile(`()\bnpm_[A-Za-z0-9]{36}`)},
	{"sendgrid", regexp.MustCompile(`()\bSG\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{16,}`)},
	{"jwt", regexp.MustCompile(`()\beyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)},
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

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "mask",
		Short: "Secret redactor — pattern matching and entropy analysis",
		Flags: []shared.FlagMeta{
			{Name: "pattern", Usage: "additional pattern regex (repeatable)"},
			{Name: "entropy", Usage: "Shannon entropy threshold (default 4.0; lower = more aggressive)"},
			{Name: "json", Shorthand: "j", Usage: "append metadata JSON record after text output"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk mask. Returns 0, 1, or 2. Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("mask", pflag.ContinueOnError)
	var patterns []string
	var entropyThreshold float64
	var jsonOut bool

	var quietFlag bool
	fs.StringArrayVar(&patterns, "pattern", nil, "additional pattern regex (repeatable)")
	fs.Float64Var(&entropyThreshold, "entropy", 4.0, "Shannon entropy threshold (default 4.0; lower = more aggressive)")
	fs.BoolVarP(&jsonOut, "json", "j", false, "append metadata JSON record after text output")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	// Suppress pflag's automatic error printing so all output goes through shared helpers.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("mask: %s", err)
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

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
	scanner := shared.ScanLines(r)

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
	}

	if err := scanner.Err(); err != nil {
		_ = w.Flush()
		if jsonOut {
			return shared.PrintJSONError(map[string]any{
				"error": fmt.Sprintf("mask: reading stdin: %v", err),
				"code":  1,
			})
		}
		return shared.Errorf("mask: reading stdin: %v", err)
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

	// Entropy scan: check each whitespace-delimited token that is >= 8 chars and
	// does not already contain [REDACTED] (which would indicate it was already
	// handled by a pattern, or is itself a placeholder from a prior substitution).
	hit := func() {
		if !seen["entropy"] {
			fired = append(fired, "entropy")
			seen["entropy"] = true
		}
	}
	out = tokenRe.ReplaceAllStringFunc(out, func(tok string) string {
		if strings.Contains(tok, "[REDACTED]") {
			return tok
		}
		lead, core, trail := splitPunct(tok)
		if len(core) < 8 {
			return tok
		}
		if masked, ok := maskStructured(core, threshold); ok {
			if masked != core {
				hit()
			}
			return lead + masked + trail
		}
		if shannonEntropy(core) >= threshold {
			hit()
			return lead + "[REDACTED]" + trail
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

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("mask", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringArray("pattern", nil, "additional pattern regex (repeatable)")
	fs.Float64("entropy", 4.0, "Shannon entropy threshold (default 4.0; lower = more aggressive)")
	fs.BoolP("json", "j", false, "append metadata JSON record after text output")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
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

// splitPunct peels one leading opener and any trailing closers or sentence
// punctuation off tok, so "(KEY)," redacts to "([REDACTED]),".
func splitPunct(tok string) (lead, core, trail string) {
	core = tok
	if core != "" && strings.ContainsRune(`('"[{`, rune(core[0])) {
		lead, core = core[:1], core[1:]
	}
	trimmed := strings.TrimRight(core, `,.;:)]}'"`)
	return lead, trimmed, core[len(trimmed):]
}

var (
	uuidRe     = regexp.MustCompile(`^[0-9a-fA-F]{8}-(?:[0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}$`)
	digestRe   = regexp.MustCompile(`^(?:sha1|sha256|sha512|md5):[0-9a-fA-F]+$`)
	pathRe     = regexp.MustCompile(`^(?:/|\./|\.\./|~/)[A-Za-z0-9._/@-]*$`)
	imageRe    = regexp.MustCompile(`^[A-Za-z0-9-]+(?:\.[A-Za-z0-9-]+)+(?::[0-9]+)?/[A-Za-z0-9._/-]+(?::[A-Za-z0-9._-]+)?$`)
	urlRe      = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9+.-]*://`)
	keyValueRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*=(.+)$`)
)

// secretParams are URL query/fragment parameter names whose values are
// entropy-checked. A name matches on its full lowercased form or on its last
// _ - . separated part, so client_secret and X-Amz-Signature both count.
var secretParams = map[string]bool{
	"key": true, "api_key": true, "apikey": true, "token": true, "access_token": true,
	"secret": true, "code": true, "sig": true, "signature": true, "password": true, "pass": true, "auth": true,
}

func isSecretParam(name string) bool {
	n := strings.ToLower(name)
	if secretParams[n] {
		return true
	}
	if i := strings.LastIndexAny(n, "_-."); i >= 0 {
		return secretParams[n[i+1:]]
	}
	return false
}

// maskStructured handles tokens that look like URLs, paths, UUIDs, digests, or
// image refs. Those are context an LLM needs, so the whole token is never
// entropy-checked. Instead each path segment (and each secret-named query
// value) is checked on its own, which still catches a webhook secret in a
// path while leaving /v1/charges alone. ok is false when tok is none of these.
// A name=value token is judged by its value, so request_id=<uuid> is kept.
func maskStructured(tok string, threshold float64) (string, bool) {
	if m := keyValueRe.FindStringSubmatch(tok); m != nil {
		v, ok := maskStructured(m[1], threshold)
		return tok[:len(tok)-len(m[1])] + v, ok
	}
	switch {
	case uuidRe.MatchString(tok), digestRe.MatchString(tok):
		return tok, true
	case pathRe.MatchString(tok), imageRe.MatchString(tok):
		return maskSegments(tok, threshold), true
	case urlRe.MatchString(tok):
		return maskURL(tok, threshold)
	}
	return tok, false
}

func maskSegments(p string, threshold float64) string {
	segs := strings.Split(p, "/")
	for i, s := range segs {
		if len(s) >= 8 && shannonEntropy(s) >= threshold {
			segs[i] = "[REDACTED]"
		}
	}
	return strings.Join(segs, "/")
}

// maskURL splits scheme://host/path?query#fragment by hand (net/url would
// re-encode the parts). Any userinfo (user@ or user:pass@) means the URL
// carries a credential, so it falls back to whole-token entropy.
func maskURL(u string, threshold float64) (string, bool) {
	i := strings.Index(u, "://") + 3
	end := len(u)
	if j := strings.IndexAny(u[i:], "/?#"); j >= 0 {
		end = i + j
	}
	if strings.Contains(u[i:end], "@") {
		return u, false
	}
	rest := u[end:]
	path, tail := rest, ""
	if j := strings.IndexAny(rest, "?#"); j >= 0 {
		path, tail = rest[:j], rest[j:]
	}
	var b strings.Builder
	b.WriteString(u[:end])
	b.WriteString(maskSegments(path, threshold))
	// tail is a run of ?query and #fragment parts; both use k=v&k=v.
	for tail != "" {
		b.WriteByte(tail[0])
		tail = tail[1:]
		part := tail
		if j := strings.IndexAny(tail, "?#"); j >= 0 {
			part, tail = tail[:j], tail[j:]
		} else {
			tail = ""
		}
		params := strings.Split(part, "&")
		for k, p := range params {
			name, val, found := strings.Cut(p, "=")
			if found && isSecretParam(name) && len(val) >= 8 && shannonEntropy(val) >= threshold {
				params[k] = name + "=[REDACTED]"
			}
		}
		b.WriteString(strings.Join(params, "&"))
	}
	return b.String(), true
}
