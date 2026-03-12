package shared

import (
	"os"

	"github.com/spf13/pflag"
)

// SilenceStderr redirects os.Stderr to /dev/null when quiet is true and
// returns a restore func. All stderr output — including error messages — is
// suppressed for the duration, so the caller receives exit codes only.
// If quiet is false, returns a no-op. If /dev/null cannot be opened (rare),
// returns a no-op so --quiet is silently ignored rather than crashing.
//
// Call immediately after flag parsing with defer so cleanup always runs:
//
//	defer shared.SilenceStderr(quietFlag)()
func SilenceStderr(quiet bool) func() {
	if !quiet {
		return func() {}
	}
	f, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return func() {}
	}
	orig := os.Stderr
	os.Stderr = f
	return func() {
		os.Stderr = orig
		_ = f.Close()
	}
}

// StandardFlags returns a pflag.FlagSet with the flags that are common across
// every vrksh tool. Tools call StandardFlags(), add their own flags, then parse.
//
// Registered flags:
//
//	-j / --json     emit output as JSON
//	-q / --quiet    suppress stderr output
//	     --dry-run  preview side effects without executing  (no shorthand — intentional)
//	     --explain  print what the tool would do, no action (no shorthand — intentional)
func StandardFlags() *pflag.FlagSet {
	fs := pflag.NewFlagSet("vrk", pflag.ContinueOnError)
	fs.BoolP("json", "j", false, "emit output as JSON")
	fs.BoolP("quiet", "q", false, "suppress stderr output")
	fs.BoolP("fail", "f", false, "exit 1 if condition not met")
	fs.Bool("dry-run", false, "preview side effects without executing")
	fs.Bool("explain", false, "print what the tool would do without executing")
	return fs
}
