package shared

import (
	"github.com/spf13/pflag"
)

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
