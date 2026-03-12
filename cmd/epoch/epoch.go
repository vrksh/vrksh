package epoch

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embed IANA timezone database — works in Docker scratch and systems without /usr/share/zoneinfo

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// epochJSON is the shape emitted by --json.
type epochJSON struct {
	Input string `json:"input,omitempty"`
	Unix  int64  `json:"unix"`
	ISO   string `json:"iso"`
	Ref   string `json:"ref,omitempty"`
	TZ    string `json:"tz,omitempty"`
}

// Run is the entry point for vrk epoch. Returns 0 (success), 1 (runtime error),
// or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("epoch", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var isoFlag bool
	var jsonFlag bool
	var tzStr string
	var nowFlag bool
	var atStr string
	var quietFlag bool

	fs.BoolVar(&isoFlag, "iso", false, "output as ISO 8601 string instead of Unix integer")
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit output as JSON: {input, unix, iso, ref?, tz?}")
	fs.StringVar(&tzStr, "tz", "", "timezone for --iso or --json output (IANA name or +HH:MM offset)")
	fs.BoolVar(&nowFlag, "now", false, "print current Unix timestamp and exit")
	fs.StringVar(&atStr, "at", "", "reference timestamp for relative input (unix integer), e.g. --at 1740009600")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	// Pre-extract any negative-relative-time argument before pflag sees it.
	// pflag treats args starting with '-' as flags, so '-3d', '-2h', etc. would
	// trigger "unknown flag" errors without this pre-pass. isNegativeRelative
	// matches exactly: '-' + one-or-more digits + unit letter.
	args := os.Args[1:]
	var negRelInput string
	cleanArgs := make([]string, 0, len(args))
	for _, a := range args {
		if negRelInput == "" && isNegativeRelative(a) {
			negRelInput = a
		} else {
			cleanArgs = append(cleanArgs, a)
		}
	}

	if err := fs.Parse(cleanArgs); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("epoch: %s", err.Error())
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	// usageErrorf and errorf route errors through stdout as JSON when --json is
	// active, keeping stderr empty so downstream consumers see only structured data.
	usageErrorf := func(format string, args ...any) int {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": fmt.Sprintf(format, args...), "code": shared.ExitUsage})
		}
		return shared.UsageErrorf(format, args...)
	}

	errorf := func(format string, args ...any) int {
		if jsonFlag {
			return shared.PrintJSONError(map[string]any{"error": fmt.Sprintf(format, args...), "code": shared.ExitError})
		}
		return shared.Errorf(format, args...)
	}

	// --tz without --iso or --json: timezone has no meaning for a plain Unix integer.
	if tzStr != "" && !isoFlag && !jsonFlag {
		return usageErrorf("epoch: --tz requires --iso")
	}

	posArgs := fs.Args()

	// Reject more than one positional arg.
	if len(posArgs) > 1 {
		return usageErrorf("epoch: too many arguments, expected one input")
	}

	// If the pre-pass extracted a negative-relative arg AND there is also a
	// positional arg, that is two inputs — reject it.
	if negRelInput != "" && len(posArgs) > 0 {
		return usageErrorf("epoch: too many arguments, expected one input")
	}

	// --now with no other input source: print current timestamp immediately,
	// without touching stdin (reading stdin on a TTY would hang).
	if nowFlag && negRelInput == "" && len(posArgs) == 0 {
		if jsonFlag {
			loc, tzErr := parseTimezone(tzStr)
			if tzErr != nil {
				return usageErrorf("epoch: %v", tzErr)
			}
			now := time.Now()
			out := epochJSON{
				Unix: now.Unix(),
				ISO:  now.In(loc).Format(time.RFC3339),
			}
			if tzStr != "" {
				out.TZ = tzStr
			}
			if err := shared.PrintJSON(out); err != nil {
				return shared.Errorf("epoch: %v", err)
			}
			return 0
		}
		if _, err := fmt.Fprintf(os.Stdout, "%d\n", time.Now().Unix()); err != nil {
			return shared.Errorf("epoch: %v", err)
		}
		return 0
	}

	// Read input: negRelInput > positional arg > stdin.
	var input string
	switch {
	case negRelInput != "":
		input = negRelInput
	case len(posArgs) > 0:
		input = strings.TrimSpace(posArgs[0])
	default:
		var readErr error
		input, readErr = shared.ReadInputOptional(nil)
		if readErr != nil {
			return errorf("epoch: %v", readErr)
		}
	}

	// --at with no input: usage error (nothing to calculate relative to).
	if atStr != "" && input == "" {
		return usageErrorf("epoch: --at requires input; use --now to print the current timestamp")
	}

	// No input at all (no --now, no args, no stdin).
	if input == "" {
		return usageErrorf("epoch: no input: provide as argument or via stdin")
	}

	// Resolve the "now" reference timestamp for relative calculations.
	var nowRef int64
	if atStr != "" {
		v, err := strconv.ParseInt(atStr, 10, 64)
		if err != nil {
			return usageErrorf("epoch: --at: invalid timestamp %q: must be a Unix integer", atStr)
		}
		nowRef = v
	} else {
		nowRef = time.Now().Unix()
	}

	// Parse input into a Unix timestamp.
	ts, parseErr := parseInput(input, nowRef)
	if parseErr != nil {
		return usageErrorf("epoch: %v", parseErr)
	}

	// Resolve timezone (meaningful with --iso and --json).
	loc, tzErr := parseTimezone(tzStr)
	if tzErr != nil {
		return usageErrorf("epoch: %v", tzErr)
	}

	// --json: emit structured output.
	if jsonFlag {
		out := epochJSON{
			Input: input,
			Unix:  ts,
			ISO:   time.Unix(ts, 0).In(loc).Format(time.RFC3339),
		}
		if atStr != "" {
			out.Ref = atStr
		}
		if tzStr != "" {
			out.TZ = tzStr
		}
		if err := shared.PrintJSON(out); err != nil {
			return shared.Errorf("epoch: %v", err)
		}
		return 0
	}

	// Emit plain text output.
	if isoFlag {
		if _, err := fmt.Fprintln(os.Stdout, time.Unix(ts, 0).In(loc).Format(time.RFC3339)); err != nil {
			return shared.Errorf("epoch: %v", err)
		}
	} else {
		if _, err := fmt.Fprintf(os.Stdout, "%d\n", ts); err != nil {
			return shared.Errorf("epoch: %v", err)
		}
	}
	return 0
}

// isNegativeRelative reports whether s is a negative relative time expression
// like -3d, -2h, -30s. Pattern: '-' + one-or-more digits + unit letter (s/m/h/d/w).
func isNegativeRelative(s string) bool {
	if len(s) < 3 || s[0] != '-' {
		return false
	}
	unit := s[len(s)-1]
	if unit != 's' && unit != 'm' && unit != 'h' && unit != 'd' && unit != 'w' {
		return false
	}
	_, err := strconv.ParseInt(s[1:len(s)-1], 10, 64)
	return err == nil
}

// parseInput converts a string representation into a Unix timestamp.
// Supported forms (checked in order):
//
//  1. All digits with optional leading minus, no unit suffix → Unix integer passthrough
//  2. +/-<digits><unit> → relative time from nowRef
//  3. <digits><unit> without sign → usage error (sign required)
//  4. YYYY-MM-DD or RFC3339 datetime → ISO parse
//  5. Everything else → unsupported format
func parseInput(s string, nowRef int64) (int64, error) {
	if s == "" {
		return 0, fmt.Errorf("no input: provide as argument or via stdin")
	}

	// Case 1: bare integer (Unix passthrough). Accept optional leading '-'.
	// Must have no letter suffix — disambiguates from relative forms like -3d.
	if isBareInteger(s) {
		return strconv.ParseInt(s, 10, 64)
	}

	// Case 2: relative time — must start with + or -.
	if len(s) >= 2 && (s[0] == '+' || s[0] == '-') {
		if hasUnitSuffix(s[1:]) {
			return parseRelative(s, nowRef)
		}
	}

	// Case 3: looks like relative but missing sign prefix (e.g. "3d", "10h").
	if hasUnitSuffix(s) {
		return 0, fmt.Errorf("sign required: use +%s or -%s", s, s)
	}

	// Cases 4+5: try ISO parsing; otherwise unsupported.
	return parseISO(s)
}

// isBareInteger reports whether s is a bare integer: optional leading '-'
// followed only by ASCII digits, with no unit letter suffix.
func isBareInteger(s string) bool {
	if s == "" {
		return false
	}
	start := 0
	if s[0] == '-' {
		if len(s) == 1 {
			return false
		}
		start = 1
	}
	for i := start; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// hasUnitSuffix reports whether s ends with a recognised unit letter (s/m/h/d/w)
// and everything before the unit is all digits.
func hasUnitSuffix(s string) bool {
	if len(s) < 2 {
		return false
	}
	last := s[len(s)-1]
	switch last {
	case 's', 'm', 'h', 'd', 'w':
		// OK
	default:
		return false
	}
	digits := s[:len(s)-1]
	if digits == "" {
		return false
	}
	for _, c := range digits {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// parseRelative parses a relative time expression like +3d or -6h.
// s must start with + or -.
func parseRelative(s string, nowRef int64) (int64, error) {
	if len(s) < 3 {
		return 0, fmt.Errorf("invalid relative time %q: use +3d or -3d", s)
	}

	sign := int64(1)
	if s[0] == '-' {
		sign = -1
	}
	rest := s[1:] // strip sign

	unit := rest[len(rest)-1]
	magnitudeStr := rest[:len(rest)-1]

	magnitude, err := strconv.ParseInt(magnitudeStr, 10, 64)
	if err != nil || magnitude < 0 {
		return 0, fmt.Errorf("invalid relative time %q: magnitude must be a positive integer", s)
	}

	var secondsPerUnit int64
	switch unit {
	case 's':
		secondsPerUnit = 1
	case 'm':
		secondsPerUnit = 60
	case 'h':
		secondsPerUnit = 3600
	case 'd':
		secondsPerUnit = 86400
	case 'w':
		secondsPerUnit = 604800
	default:
		return 0, fmt.Errorf("invalid relative time %q: unknown unit %q, use s/m/h/d/w", s, string(unit))
	}

	return nowRef + sign*magnitude*secondsPerUnit, nil
}

// parseISO parses an ISO 8601 date or datetime string into a Unix timestamp.
// Tries RFC3339 first (covers datetimes with timezone), then date-only
// (YYYY-MM-DD, treated as midnight UTC).
func parseISO(s string) (int64, error) {
	// RFC3339 covers "2025-02-20T10:00:00Z" and offset variants.
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.Unix(), nil
	}

	// Date-only: "2025-02-20" → midnight UTC.
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t.UTC().Unix(), nil
	}

	return 0, fmt.Errorf("unsupported format %q: natural language not supported; accepted forms: unix integer, +3d/-3d, YYYY-MM-DD, YYYY-MM-DDTHH:MM:SSZ", s)
}

// parseTimezone parses a timezone string into a *time.Location.
// Accepted forms:
//   - "" → UTC
//   - "UTC" or "GMT" → UTC
//   - "+HH:MM" or "-HH:MM" → fixed offset
//   - "<region>/<city>" (contains /) → IANA name via time.LoadLocation
//   - anything else (e.g. "IST", "EST") → usage error (ambiguous)
func parseTimezone(s string) (*time.Location, error) {
	if s == "" || s == "UTC" || s == "GMT" {
		return time.UTC, nil
	}

	// Numeric offset: starts with + or - and contains :
	if (s[0] == '+' || s[0] == '-') && strings.Contains(s, ":") {
		return parseNumericOffset(s)
	}

	// IANA name: must contain a slash (e.g. "America/New_York").
	if strings.Contains(s, "/") {
		loc, err := time.LoadLocation(s)
		if err != nil {
			return nil, fmt.Errorf("unknown timezone %q: %v", s, err)
		}
		return loc, nil
	}

	// Abbreviation — always ambiguous. Never accept these.
	return nil, fmt.Errorf("%s is ambiguous; use a full IANA name (e.g. Asia/Kolkata) or a numeric offset (e.g. +05:30)", s)
}

// parseNumericOffset parses "+05:30" or "-05:00" into a fixed *time.Location.
func parseNumericOffset(s string) (*time.Location, error) {
	sign := 1
	rest := s[1:]
	if s[0] == '-' {
		sign = -1
	}

	parts := strings.SplitN(rest, ":", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid timezone offset %q: expected ±HH:MM", s)
	}

	hours, err := strconv.Atoi(parts[0])
	if err != nil || hours < 0 || hours > 23 {
		return nil, fmt.Errorf("invalid timezone offset %q: hours out of range", s)
	}
	minutes, err := strconv.Atoi(parts[1])
	if err != nil || minutes < 0 || minutes > 59 {
		return nil, fmt.Errorf("invalid timezone offset %q: minutes out of range", s)
	}

	offsetSeconds := sign * (hours*3600 + minutes*60)
	return time.FixedZone(s, offsetSeconds), nil
}

// printUsage writes usage information to stdout and returns 0.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: epoch [flags] <input>",
		"       echo <input> | epoch [flags]",
		"",
		"Timestamp converter — converts between Unix integers and ISO 8601 dates/times.",
		"Default output is a Unix integer. Use --iso for ISO 8601 output.",
		"",
		"Accepted input forms:",
		"  1740009600           Unix integer (passed through)",
		"  2025-02-20           ISO date (midnight UTC)",
		"  2025-02-20T10:00:00Z ISO datetime (RFC3339)",
		"  +3d / -3d            Relative time (sign required); units: s m h d w",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("epoch: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
