// Package base implements vrk base — an encoding converter.
// Encodes and decodes between base64, base64url, hex, and base32.
// All four encodings are handled by the Go standard library; no new dependencies.
package base

import (
	"bytes"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
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

// readAll is a var so tests can inject I/O errors.
var readAll = io.ReadAll

func init() {
	shared.Register(shared.ToolMeta{
		Name:  "base",
		Short: "Encoding converter — base64, base64url, hex, base32",
		Flags: []shared.FlagMeta{
			{Name: "to", Usage: "target encoding (base64, base64url, hex, base32)"},
			{Name: "from", Usage: "source encoding (base64, base64url, hex, base32)"},
			{Name: "quiet", Shorthand: "q", Usage: "suppress stderr output"},
		},
	})
}

// Run is the entry point for vrk base. Returns 0/1/2. Never calls os.Exit.
func Run() int {
	if len(os.Args) < 2 {
		return shared.UsageErrorf("base: usage: vrk base <encode|decode> [flags]")
	}
	switch os.Args[1] {
	case "--help", "-h":
		return printUsage()
	case "encode":
		return encode(os.Args[2:])
	case "decode":
		return decode(os.Args[2:])
	default:
		return shared.UsageErrorf("base: unknown subcommand %q — want encode or decode", os.Args[1])
	}
}

func encode(args []string) int {
	fs := pflag.NewFlagSet("base-encode", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var toFlag string
	var quiet bool
	fs.StringVar(&toFlag, "to", "", "target encoding (base64, base64url, hex, base32)")
	fs.BoolVarP(&quiet, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage()
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quiet)()

	if toFlag == "" {
		return shared.UsageErrorf("base: --to is required")
	}
	if !supportedEncoding(toFlag) {
		return shared.UsageErrorf("base: unsupported encoding: %s", toFlag)
	}

	input, code := readInput(fs)
	if code != 0 {
		return code
	}

	input = bytes.TrimSuffix(input, []byte("\n"))
	if len(input) == 0 {
		return shared.ExitOK
	}

	encoded := encodeWith(toFlag, input)
	if _, err := fmt.Fprintln(os.Stdout, encoded); err != nil {
		return shared.Errorf("base: writing output: %v", err)
	}
	return shared.ExitOK
}

func decode(args []string) int {
	fs := pflag.NewFlagSet("base-decode", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)

	var fromFlag string
	var quiet bool
	fs.StringVar(&fromFlag, "from", "", "source encoding (base64, base64url, hex, base32)")
	fs.BoolVarP(&quiet, "quiet", "q", false, "suppress stderr output")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return printUsage()
		}
		return shared.UsageErrorf("%s", err.Error())
	}

	defer shared.SilenceStderr(quiet)()

	if fromFlag == "" {
		return shared.UsageErrorf("base: --from is required")
	}
	if !supportedEncoding(fromFlag) {
		return shared.UsageErrorf("base: unsupported encoding: %s", fromFlag)
	}

	input, code := readInput(fs)
	if code != 0 {
		return code
	}

	input = bytes.TrimSuffix(input, []byte("\n"))
	if len(input) == 0 {
		return shared.ExitOK
	}

	decoded, err := decodeWith(fromFlag, input)
	if err != nil {
		return shared.Errorf("base: %s", err.Error())
	}

	if _, err := os.Stdout.Write(decoded); err != nil {
		return shared.Errorf("base: writing output: %v", err)
	}
	return shared.ExitOK
}

// readInput returns raw input bytes from a positional argument (if provided)
// or from stdin. Returns (nil, exit-code) on error.
func readInput(fs *pflag.FlagSet) ([]byte, int) {
	if posArgs := fs.Args(); len(posArgs) > 0 {
		return []byte(strings.Join(posArgs, " ")), 0
	}
	if isTerminal(int(os.Stdin.Fd())) {
		return nil, shared.UsageErrorf("base: no input: pipe data to stdin")
	}
	data, err := readAll(os.Stdin)
	if err != nil {
		return nil, shared.Errorf("base: reading stdin: %v", err)
	}
	return data, 0
}

func supportedEncoding(enc string) bool {
	switch enc {
	case "base64", "base64url", "hex", "base32":
		return true
	}
	return false
}

func encodeWith(enc string, data []byte) string {
	switch enc {
	case "base64":
		return base64.StdEncoding.EncodeToString(data)
	case "base64url":
		// RawURLEncoding: no padding, URL-safe alphabet (- and _ instead of + and /).
		return base64.RawURLEncoding.EncodeToString(data)
	case "hex":
		return hex.EncodeToString(data) // lowercase output
	default: // base32
		return base32.StdEncoding.EncodeToString(data) // uppercase with = padding
	}
}

func decodeWith(enc string, data []byte) ([]byte, error) {
	s := string(data)
	switch enc {
	case "base64":
		out, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, errors.New("invalid base64 input")
		}
		return out, nil
	case "base64url":
		out, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			return nil, errors.New("invalid base64url input")
		}
		return out, nil
	case "hex":
		out, err := hex.DecodeString(s)
		if err != nil {
			return nil, errors.New("invalid hex input")
		}
		return out, nil
	default: // base32
		out, err := base32.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, errors.New("invalid base32 input")
		}
		return out, nil
	}
}

// Flags returns flag metadata for MCP schema generation.
// This FlagSet is never used for parsing — Run() creates its own.
// Registers the union of encode and decode flags.
func Flags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("base", pflag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.String("to", "", "target encoding (base64, base64url, hex, base32)")
	fs.String("from", "", "source encoding (base64, base64url, hex, base32)")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	return fs
}

func printUsage() int {
	lines := []string{
		"usage: vrk base encode --to <encoding> [flags]",
		"       vrk base decode --from <encoding> [flags]",
		"       echo 'hello' | vrk base encode --to base64",
		"       echo 'aGVsbG8=' | vrk base decode --from base64",
		"",
		"Encoding converter — encodes and decodes between base64, base64url, hex, base32.",
		"Strips exactly one trailing newline from stdin (echo-safe). Use printf to",
		"preserve a trailing newline. Binary input works correctly via stdin.",
		"",
		"subcommands:",
		"  encode    encode stdin or positional arg to the target encoding",
		"  decode    decode stdin or positional arg from the source encoding",
		"",
		"flags (encode):",
		"      --to string   target encoding: base64, base64url, hex, base32",
		"  -q, --quiet       suppress stderr output (exit codes unaffected)",
		"",
		"flags (decode):",
		"      --from string  source encoding: base64, base64url, hex, base32",
		"  -q, --quiet        suppress stderr output (exit codes unaffected)",
	}
	for _, l := range lines {
		if _, err := fmt.Fprintln(os.Stdout, l); err != nil {
			return shared.Errorf("base: writing usage: %v", err)
		}
	}
	return 0
}
