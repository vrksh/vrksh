package jwt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/internal/shared"
)

// jwtEnvelope is the shape emitted by --json when no other mode flag is set.
// Fields marshal in declaration order for a stable key sequence in output.
type jwtEnvelope struct {
	Header    map[string]any `json:"header"`
	Payload   map[string]any `json:"payload"`
	Signature string         `json:"signature"`
	Expired   bool           `json:"expired"`
	Valid     bool           `json:"valid"`
	// SignatureVerified is always false: "valid" covers the time claims only.
	SignatureVerified bool `json:"signature_verified"`
}

// now is a var so tests can pin the clock.
var now = time.Now

// jsonError emits {"error":"msg","code":N} to stdout and returns N.
// Used when --json is active so that all output — including errors — goes
// to stdout as structured JSON and stderr stays empty.
func jsonError(code int, msg string) int {
	return shared.PrintJSONError(map[string]any{"error": msg, "code": code})
}

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "jwt",
		Short: "JWT inspector — decode, --claim, --expired",
		Flags: []shared.FlagMeta{
			{Name: "json", Shorthand: "j", Usage: "emit output as JSON"},
			{Name: "claim", Shorthand: "c", Usage: "print value of a single claim as plain text"},
			{Name: "expired", Shorthand: "e", Usage: "exit 1 if the token is expired"},
			{Name: "valid", Usage: "exit 1 if token is expired, not yet valid (nbf), or issued in the future (iat)"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk jwt. Returns 0 (success), 1 (runtime error),
// or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("jwt", pflag.ContinueOnError)
	var jsonFlag bool
	var claimName string
	var checkExpired bool
	var checkValid bool
	var quietFlag bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit output as JSON")
	fs.StringVarP(&claimName, "claim", "c", "", "print value of a single claim as plain text")
	fs.BoolVarP(&checkExpired, "expired", "e", false, "exit 1 if the token is expired")
	fs.BoolVar(&checkValid, "valid", false, "exit 1 if token is expired, not yet valid (nbf), or issued in the future (iat)")
	fs.BoolVarP(&quietFlag, "quiet", "q", false, "suppress stderr output")

	// Suppress pflag's automatic printing so we control where output goes.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		// Parse errors happen before we know if --json is active. Use stderr.
		return shared.UsageErrorf("%s", err.Error())
	}

	// errorf and usageErrorf route errors through JSON when --json is active,
	// so that stdout always carries the structured result and stderr stays empty.
	errorf := func(format string, args ...any) int {
		if jsonFlag {
			return jsonError(shared.ExitError, fmt.Sprintf(format, args...))
		}
		return shared.Errorf(format, args...)
	}
	usageErrorf := func(format string, args ...any) int {
		if jsonFlag {
			return jsonError(shared.ExitUsage, fmt.Sprintf(format, args...))
		}
		return shared.UsageErrorf(format, args...)
	}

	// --quiet: suppress all stderr output (including errors) — callers get exit codes only.
	defer shared.SilenceStderr(quietFlag)()

	// Reject more than one positional arg early.
	if len(fs.Args()) > 1 {
		return usageErrorf("jwt: too many arguments, expected one token")
	}

	input, err := shared.ReadInput(fs.Args())
	if err != nil {
		return usageErrorf("jwt: %v", err)
	}
	// Strip exactly one trailing newline (echo appends one; printf does not).
	// Leading/trailing spaces are the caller's responsibility — JWT tokens are
	// base64url-encoded and contain no whitespace.
	input = strings.TrimSuffix(input, "\n")

	header, payload, signature, err := decodeJWT(input)
	if err != nil {
		return errorf("%v", err)
	}

	// --expired: check before formatting output so it acts as a guard.
	if checkExpired {
		expired, claimErr := isExpired(payload, now().Unix())
		if claimErr != nil {
			return errorf("%v", claimErr)
		}
		if jsonFlag {
			// --expired --json: emit {"expired":bool} — always to stdout, exit 1 if expired.
			if err := shared.PrintJSON(map[string]any{"expired": expired}); err != nil {
				return jsonError(shared.ExitError, err.Error())
			}
			if expired {
				return 1
			}
			return 0
		}
		if expired {
			return shared.Errorf("%v", validityError(payload, now().Unix()))
		}
		// Not expired — fall through to output.
	}

	// --valid: stricter than --expired. Checks exp, nbf, and iat in order.
	if checkValid {
		if err := validityError(payload, now().Unix()); err != nil {
			return errorf("%v", err)
		}
	}

	// --claim: extract a single field.
	if claimName != "" {
		val, ok := payload[claimName]
		if !ok {
			return errorf("jwt: claim %q not found", claimName)
		}
		if jsonFlag {
			if err := shared.PrintJSON(map[string]any{"claim": claimName, "value": formatClaim(val)}); err != nil {
				return jsonError(shared.ExitError, err.Error())
			}
			return 0
		}
		if _, err := fmt.Fprintln(os.Stdout, formatClaim(val)); err != nil {
			return shared.Errorf("jwt: %v", err)
		}
		return 0
	}

	// --json: full envelope with new shape.
	if jsonFlag {
		ts := now().Unix()
		expired, _ := isExpired(payload, ts)
		env := jwtEnvelope{
			Header:    header,
			Payload:   payload,
			Signature: signature,
			Expired:   expired,
			Valid:     validityError(payload, ts) == nil,
		}
		if err := shared.PrintJSON(env); err != nil {
			return jsonError(shared.ExitError, err.Error())
		}
		return 0
	}

	// Default: print the decoded payload as JSON.
	if err := shared.PrintJSON(payload); err != nil {
		return shared.Errorf("jwt: %v", err)
	}
	return 0
}

// numericDate returns claim k as Unix seconds. present is false only when the
// claim is absent. A claim that is there but is not a usable NumericDate is an
// error, so callers fail closed instead of skipping the check.
func numericDate(p map[string]any, k string) (sec int64, present bool, err error) {
	v, ok := p[k]
	if !ok {
		return 0, false, nil
	}
	n, ok := v.(json.Number)
	if !ok {
		return 0, true, fmt.Errorf("jwt: %s claim is not a number", k)
	}
	if i, e := n.Int64(); e == nil {
		return i, true, nil
	}
	f, e := n.Float64()
	switch {
	case e != nil && !math.IsInf(f, 0), math.IsNaN(f):
		return 0, true, fmt.Errorf("jwt: %s claim is not a valid NumericDate", k)
	case f >= math.MaxInt64:
		return math.MaxInt64, true, nil
	case f <= math.MinInt64:
		// A date before the start of int64 time is nonsense for exp/nbf/iat.
		return 0, true, fmt.Errorf("jwt: %s claim is out of range", k)
	}
	return int64(math.Floor(f)), true, nil
}

// isExpired reports whether exp is present and now is on or after it.
func isExpired(p map[string]any, now int64) (bool, error) {
	exp, ok, err := numericDate(p, "exp")
	if err != nil {
		return true, err
	}
	return ok && now >= exp, nil
}

// validityError checks exp, nbf and iat in that order. Missing claims are
// skipped; present but unusable ones fail.
func validityError(p map[string]any, now int64) error {
	exp, ok, err := numericDate(p, "exp")
	if err != nil {
		return err
	}
	if ok && now >= exp {
		t := time.Unix(exp, 0)
		return fmt.Errorf("jwt: token expired %s ago (exp: %s)", time.Unix(now, 0).Sub(t), t.UTC().Format(time.RFC3339))
	}
	nbf, ok, err := numericDate(p, "nbf")
	if err != nil {
		return err
	}
	if ok && now < nbf {
		return fmt.Errorf("jwt: token not yet valid (nbf: %s)", time.Unix(nbf, 0).UTC().Format(time.RFC3339))
	}
	iat, ok, err := numericDate(p, "iat")
	if err != nil {
		return err
	}
	if ok && now < iat {
		return fmt.Errorf("jwt: token issued in the future (iat: %s)", time.Unix(iat, 0).UTC().Format(time.RFC3339))
	}
	return nil
}

// decodeJWT splits a JWT into its three parts, base64url-decodes the header and
// payload, and JSON-unmarshals them. Also returns the raw signature string.
// Signatures are never verified — this is an inspector, not a validator.
func decodeJWT(token string) (header, payload map[string]any, signature string, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, "", fmt.Errorf("jwt: invalid token: expected 3 parts, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, "", errors.New("jwt: invalid token: header is not base64url")
	}
	dec := json.NewDecoder(bytes.NewReader(headerBytes))
	dec.UseNumber()
	if err := dec.Decode(&header); err != nil {
		return nil, nil, "", errors.New("jwt: invalid token: header is not valid JSON")
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, "", errors.New("jwt: invalid token: payload is not base64url")
	}
	dec = json.NewDecoder(bytes.NewReader(payloadBytes))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, nil, "", errors.New("jwt: invalid token: payload is not valid JSON")
	}

	return header, payload, parts[2], nil
}

// formatClaim prints a claim value as plain text. Strings are printed without
// quotes. Numbers (json.Number) are printed as-is. Bools print true/false.
// Objects and arrays fall back to compact JSON.
func formatClaim(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	case bool:
		if val {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("jwt", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.BoolP("json", "j", false, "emit output as JSON")
	fs.StringP("claim", "c", "", "print value of a single claim as plain text")
	fs.BoolP("expired", "e", false, "exit 1 if the token is expired")
	fs.Bool("valid", false, "exit 1 if token is expired, not yet valid (nbf), or issued in the future (iat)")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

// printUsage writes usage to stdout and returns 0. Called when --help is passed.
func printUsage(fs *pflag.FlagSet) int {
	lines := []string{
		"usage: jwt [flags] <token>",
		"       echo <token> | jwt [flags]",
		"",
		"JWT inspector — decode and inspect JWT tokens without verifying signatures.",
		"Accepts exactly one token as a positional argument or via stdin.",
		"",
		"flags:",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("jwt: %v", err)
		}
	}
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	return 0
}
