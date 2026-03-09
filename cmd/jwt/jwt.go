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

// jwtEnvelope is the shape emitted by --json. Struct fields marshal in declaration
// order, giving a stable key sequence in the output.
type jwtEnvelope struct {
	Header    map[string]any `json:"header"`
	Payload   map[string]any `json:"payload"`
	ExpiresIn string         `json:"expires_in,omitempty"`
}

// Run is the entry point for vrk jwt. Returns 0 (success), 1 (runtime error),
// or 2 (usage error). Never calls os.Exit.
func Run() int {
	fs := pflag.NewFlagSet("jwt", pflag.ContinueOnError)
	var jsonFlag bool
	var claimName string
	var checkExpired bool
	var checkValid bool
	fs.BoolVarP(&jsonFlag, "json", "j", false, "emit output as JSON")
	fs.StringVarP(&claimName, "claim", "c", "", "print value of a single claim as plain text")
	fs.BoolVarP(&checkExpired, "expired", "e", false, "exit 1 if the token is expired")
	fs.BoolVar(&checkValid, "valid", false, "exit 1 if token is expired, not yet valid (nbf), or issued in the future (iat)")

	// Suppress pflag's automatic printing so we control where output goes.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	// Reject more than one positional arg early — joining them with a space
	// produces a confusing "invalid JWT: expected 3 parts" error downstream.
	if len(fs.Args()) > 1 {
		return shared.UsageErrorf("jwt: too many arguments, expected one token")
	}

	input, err := shared.ReadInput(fs.Args())
	if err != nil {
		return shared.UsageErrorf("jwt: %v", err)
	}
	// JWT tokens are never whitespace-padded. Strip leading/trailing whitespace
	// so that \r\n line endings, leading spaces, or copy-paste artifacts don't
	// corrupt the base64url-encoded header or payload.
	input = strings.TrimSpace(input)

	header, payload, err := decodeJWT(input)
	if err != nil {
		return shared.Errorf("%v", err)
	}

	// --expired: check before formatting output so it acts as a guard regardless
	// of which output flag is also set.
	if checkExpired {
		if expNum, ok := payload["exp"].(json.Number); ok {
			if expUnix, err := expNum.Int64(); err == nil {
				if time.Now().Unix() > expUnix {
					exp := time.Unix(expUnix, 0)
					ago := time.Since(exp).Round(time.Second)
					return shared.Errorf("jwt: token expired %s ago (exp: %s)", ago, exp.UTC().Format(time.RFC3339))
				}
			}
		}
		// No exp claim, or exp is in the future — fall through to output.
	}

	// --valid: stricter than --expired. Checks exp, nbf, and iat in order.
	// Only checks a claim when it is present in the payload; absence is not an error.
	if checkValid {
		now := time.Now().Unix()

		// 1. exp: token must not be expired.
		if expNum, ok := payload["exp"].(json.Number); ok {
			if expUnix, err := expNum.Int64(); err == nil {
				if now > expUnix {
					exp := time.Unix(expUnix, 0)
					ago := time.Since(exp).Round(time.Second)
					return shared.Errorf("jwt: token expired %s ago (exp: %s)", ago, exp.UTC().Format(time.RFC3339))
				}
			}
		}

		// 2. nbf: token must be past its not-before time.
		if nbfNum, ok := payload["nbf"].(json.Number); ok {
			if nbfUnix, err := nbfNum.Int64(); err == nil {
				if now < nbfUnix {
					nbf := time.Unix(nbfUnix, 0)
					return shared.Errorf("jwt: token not yet valid (nbf: %s)", nbf.UTC().Format(time.RFC3339))
				}
			}
		}

		// 3. iat: token must not claim to be issued in the future.
		if iatNum, ok := payload["iat"].(json.Number); ok {
			if iatUnix, err := iatNum.Int64(); err == nil {
				if now < iatUnix {
					iat := time.Unix(iatUnix, 0)
					return shared.Errorf("jwt: token issued in the future (iat: %s)", iat.UTC().Format(time.RFC3339))
				}
			}
		}
		// All checks passed — fall through to output.
	}

	// --claim: extract a single field as plain text.
	if claimName != "" {
		val, ok := payload[claimName]
		if !ok {
			return shared.Errorf("jwt: claim %q not found", claimName)
		}
		if _, err := fmt.Fprintln(os.Stdout, formatClaim(val)); err != nil {
			return shared.Errorf("jwt: %v", err)
		}
		return 0
	}

	// --json: full envelope with header, payload, and expiry description.
	if jsonFlag {
		env := jwtEnvelope{Header: header, Payload: payload}
		if expNum, ok := payload["exp"].(json.Number); ok {
			if expUnix, err := expNum.Int64(); err == nil {
				remaining := time.Until(time.Unix(expUnix, 0))
				if remaining <= 0 {
					env.ExpiresIn = "expired"
				} else {
					env.ExpiresIn = remaining.Round(time.Second).String()
				}
			}
		}
		if err := shared.PrintJSON(env); err != nil {
			return shared.Errorf("jwt: %v", err)
		}
		return 0
	}

	// Default: print the decoded payload as JSON.
	if err := shared.PrintJSON(payload); err != nil {
		return shared.Errorf("jwt: %v", err)
	}
	return 0
}

// decodeJWT splits a JWT into its three parts, base64url-decodes the header and
// payload, and JSON-unmarshals them. Signatures are never verified — this is an
// inspector, not a validator.
func decodeJWT(token string) (header, payload map[string]any, err error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, nil, fmt.Errorf("invalid JWT: expected 3 parts, got %d", len(parts))
	}

	headerBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid JWT: cannot decode header: %v", err)
	}
	dec := json.NewDecoder(bytes.NewReader(headerBytes))
	dec.UseNumber()
	if err := dec.Decode(&header); err != nil {
		return nil, nil, fmt.Errorf("invalid JWT: header is not valid JSON: %v", err)
	}

	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, nil, fmt.Errorf("invalid JWT: cannot decode payload: %v", err)
	}
	dec = json.NewDecoder(bytes.NewReader(payloadBytes))
	dec.UseNumber()
	if err := dec.Decode(&payload); err != nil {
		return nil, nil, fmt.Errorf("invalid JWT: payload is not valid JSON: %v", err)
	}

	return header, payload, nil
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
