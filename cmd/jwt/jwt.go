package jwt

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
}

// jsonError emits {"error":"msg","code":N} to stdout and returns N.
// Used when --json is active so that all output — including errors — goes
// to stdout as structured JSON and stderr stays empty.
func jsonError(code int, msg string) int {
	return shared.PrintJSONError(map[string]any{"error": msg, "code": code})
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
	// Strip whitespace: copy-paste artifacts or echo-appended newlines must not
	// corrupt the base64url-encoded header or payload.
	input = strings.TrimSpace(input)

	header, payload, signature, err := decodeJWT(input)
	if err != nil {
		return errorf("%v", err)
	}

	// --expired: check before formatting output so it acts as a guard.
	if checkExpired {
		expired := isTokenExpired(payload)
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
			if expNum, ok := payload["exp"].(json.Number); ok {
				if expUnix, e := expNum.Int64(); e == nil {
					exp := time.Unix(expUnix, 0)
					ago := time.Since(exp).Round(time.Second)
					return shared.Errorf("jwt: token expired %s ago (exp: %s)", ago, exp.UTC().Format(time.RFC3339))
				}
			}
			return shared.Errorf("jwt: token is expired")
		}
		// Not expired — fall through to output.
	}

	// --valid: stricter than --expired. Checks exp, nbf, and iat in order.
	if checkValid {
		now := time.Now().Unix()

		// 1. exp: must not be expired.
		if expNum, ok := payload["exp"].(json.Number); ok {
			if expUnix, e := expNum.Int64(); e == nil {
				if now > expUnix {
					exp := time.Unix(expUnix, 0)
					ago := time.Since(exp).Round(time.Second)
					return errorf("jwt: token expired %s ago (exp: %s)", ago, exp.UTC().Format(time.RFC3339))
				}
			}
		}

		// 2. nbf: must be past its not-before time.
		if nbfNum, ok := payload["nbf"].(json.Number); ok {
			if nbfUnix, e := nbfNum.Int64(); e == nil {
				if now < nbfUnix {
					nbf := time.Unix(nbfUnix, 0)
					return errorf("jwt: token not yet valid (nbf: %s)", nbf.UTC().Format(time.RFC3339))
				}
			}
		}

		// 3. iat: must not be issued in the future.
		if iatNum, ok := payload["iat"].(json.Number); ok {
			if iatUnix, e := iatNum.Int64(); e == nil {
				if now < iatUnix {
					iat := time.Unix(iatUnix, 0)
					return errorf("jwt: token issued in the future (iat: %s)", iat.UTC().Format(time.RFC3339))
				}
			}
		}
		// All checks passed — fall through to output.
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
		env := jwtEnvelope{
			Header:    header,
			Payload:   payload,
			Signature: signature,
			Expired:   isTokenExpired(payload),
			Valid:     isTokenValid(payload),
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

// isTokenExpired reports whether the token's exp claim is present and in the past.
func isTokenExpired(payload map[string]any) bool {
	if expNum, ok := payload["exp"].(json.Number); ok {
		if expUnix, err := expNum.Int64(); err == nil {
			return time.Now().Unix() > expUnix
		}
	}
	return false
}

// isTokenValid reports whether the token passes all three time checks:
// exp (not expired), nbf (past), iat (not in future). Missing claims are skipped.
func isTokenValid(payload map[string]any) bool {
	now := time.Now().Unix()

	if expNum, ok := payload["exp"].(json.Number); ok {
		if expUnix, err := expNum.Int64(); err == nil && now > expUnix {
			return false
		}
	}
	if nbfNum, ok := payload["nbf"].(json.Number); ok {
		if nbfUnix, err := nbfNum.Int64(); err == nil && now < nbfUnix {
			return false
		}
	}
	if iatNum, ok := payload["iat"].(json.Number); ok {
		if iatUnix, err := iatNum.Int64(); err == nil && now < iatUnix {
			return false
		}
	}
	return true
}

// decodeJWT splits a JWT into its three parts, base64url-decodes the header and
// payload, and JSON-unmarshals them. Also returns the raw signature string.
// Signatures are never verified — this is an inspector, not a validator.
func decodeJWT(token string) (header, payload map[string]any, signature string, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, "", fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid JWT: cannot decode header: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(headerBytes))
	dec.UseNumber()
	if err := dec.Decode(&header); err != nil {
		return nil, nil, "", fmt.Errorf("invalid JWT: header is not valid JSON: %v", err)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, "", fmt.Errorf("invalid JWT: cannot decode payload: %v", err)
	}
	dec = json.NewDecoder(bytes.NewReader(payloadBytes))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, nil, "", fmt.Errorf("invalid JWT: payload is not valid JSON: %v", err)
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
