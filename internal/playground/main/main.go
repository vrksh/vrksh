// Command playground is the vrk build that runs in the browser at vrk.sh/play/.
// Build it with `make playground` (GOOS=js GOARCH=wasm). It holds only the
// tools that work offline: kv and sql need SQLite (no wasm build), grab and
// prompt need the network, coax needs exec, pdf needs a file.
package main

import (
	"fmt"
	"os"

	"github.com/vrksh/vrksh/cmd/assert"
	"github.com/vrksh/vrksh/cmd/base"
	"github.com/vrksh/vrksh/cmd/chunk"
	"github.com/vrksh/vrksh/cmd/digest"
	"github.com/vrksh/vrksh/cmd/emit"
	"github.com/vrksh/vrksh/cmd/epoch"
	"github.com/vrksh/vrksh/cmd/jsonl"
	"github.com/vrksh/vrksh/cmd/jwt"
	"github.com/vrksh/vrksh/cmd/links"
	"github.com/vrksh/vrksh/cmd/mask"
	"github.com/vrksh/vrksh/cmd/moniker"
	"github.com/vrksh/vrksh/cmd/pct"
	"github.com/vrksh/vrksh/cmd/plain"
	"github.com/vrksh/vrksh/cmd/recase"
	"github.com/vrksh/vrksh/cmd/sip"
	"github.com/vrksh/vrksh/cmd/slug"
	"github.com/vrksh/vrksh/cmd/sse"
	"github.com/vrksh/vrksh/cmd/throttle"
	"github.com/vrksh/vrksh/cmd/tok"
	"github.com/vrksh/vrksh/cmd/urlinfo"
	"github.com/vrksh/vrksh/cmd/uuid"
	"github.com/vrksh/vrksh/cmd/validate"
)

// Keep in sync with PLAYGROUND_TOOLS in hugo/static/js/playground.js.
var tools = map[string]func() int{
	"assert":   assert.Run,
	"base":     base.Run,
	"chunk":    chunk.Run,
	"digest":   digest.Run,
	"emit":     emit.Run,
	"epoch":    epoch.Run,
	"jsonl":    jsonl.Run,
	"jwt":      jwt.Run,
	"links":    links.Run,
	"mask":     mask.Run,
	"moniker":  moniker.Run,
	"pct":      pct.Run,
	"plain":    plain.Run,
	"recase":   recase.Run,
	"sip":      sip.Run,
	"slug":     slug.Run,
	"sse":      sse.Run,
	"throttle": throttle.Run,
	"tok":      tok.Run,
	"urlinfo":  urlinfo.Run,
	"uuid":     uuid.Run,
	"validate": validate.Run,
}

// The page calls this with argv ["vrk", "<tool>", args...], the same shape
// as subcommand mode in the real binary.
func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: vrk <tool> [args]")
		os.Exit(2)
	}
	fn, ok := tools[os.Args[1]]
	if !ok {
		fmt.Fprintf(os.Stderr, "vrk: %q does not run in the browser playground; install vrk to use it\n", os.Args[1])
		os.Exit(2)
	}
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	os.Exit(fn())
}
