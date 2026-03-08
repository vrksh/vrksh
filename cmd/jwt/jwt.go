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
	fs := shared.StandardFlags()

	var claimName string
	var checkExpired bool
	fs.StringVarP(&claimName, "claim", "c", "", "print value of a single claim as plain text")
	fs.BoolVarP(&checkExpired, "expired", "e", false, "exit 1 if the token is expired")

	// Suppress pflag's automatic printing so we control where output goes.
	fs.SetOutput(io.Discard)

	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage(fs)
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	jsonFlag, _ := fs.GetBool("json")

	input, err := shared.ReadInput(fs.Args())
	if err != nil {
		return shared.UsageErrorf("jwt: %v", err)
	}

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
