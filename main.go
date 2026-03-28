package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/pflag"
	"github.com/vrksh/vrksh/cmd/assert"
	"github.com/vrksh/vrksh/cmd/base"
	"github.com/vrksh/vrksh/cmd/chunk"
	"github.com/vrksh/vrksh/cmd/coax"
	"github.com/vrksh/vrksh/cmd/digest"
	"github.com/vrksh/vrksh/cmd/emit"
	"github.com/vrksh/vrksh/cmd/epoch"
	"github.com/vrksh/vrksh/cmd/grab"
	"github.com/vrksh/vrksh/cmd/jsonl"
	"github.com/vrksh/vrksh/cmd/jwt"
	"github.com/vrksh/vrksh/cmd/kv"
	"github.com/vrksh/vrksh/cmd/links"
	"github.com/vrksh/vrksh/cmd/mask"
	"github.com/vrksh/vrksh/cmd/moniker"
	"github.com/vrksh/vrksh/cmd/pct"
	"github.com/vrksh/vrksh/cmd/plain"
	"github.com/vrksh/vrksh/cmd/prompt"
	"github.com/vrksh/vrksh/cmd/recase"
	"github.com/vrksh/vrksh/cmd/sip"
	"github.com/vrksh/vrksh/cmd/slug"
	"github.com/vrksh/vrksh/cmd/sse"
	"github.com/vrksh/vrksh/cmd/throttle"
	"github.com/vrksh/vrksh/cmd/tok"
	"github.com/vrksh/vrksh/cmd/urlinfo"
	"github.com/vrksh/vrksh/cmd/uuid"
	"github.com/vrksh/vrksh/cmd/validate"
	"github.com/vrksh/vrksh/internal/bare"
	"github.com/vrksh/vrksh/internal/completions"
	mcppkg "github.com/vrksh/vrksh/internal/mcp"
)

//go:embed integrations/skills/SKILLS.md
var skillsDoc string

//go:embed hugo/static/skills
var skillsFS embed.FS

//go:embed manifest.json
var manifestJSON string

var tools = map[string]func() int{
	"assert":   assert.Run,
	"base":     base.Run,
	"chunk":    chunk.Run,
	"digest":   digest.Run,
	"emit":     emit.Run,
	"grab":     grab.Run,
	"jsonl":    jsonl.Run,
	"jwt":      jwt.Run,
	"epoch":    epoch.Run,
	"urlinfo":  urlinfo.Run,
	"uuid":     uuid.Run,
	"tok":      tok.Run,
	"sse":      sse.Run,
	"coax":     coax.Run,
	"prompt":   prompt.Run,
	"kv":       kv.Run,
	"links":    links.Run,
	"mask":     mask.Run,
	"moniker":  moniker.Run,
	"pct":      pct.Run,
	"recase":   recase.Run,
	"sip":      sip.Run,
	"slug":     slug.Run,
	"plain":    plain.Run,
	"throttle": throttle.Run,
	"validate": validate.Run,
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--manifest":
			fmt.Print(manifestJSON)
			os.Exit(0)
		case "--skills":
			if len(os.Args) > 2 {
				os.Exit(printSkillFiles(os.Args[2:]))
			}
			overview, err := skillsFS.ReadFile("hugo/static/skills/overview.md")
			if err != nil {
				// Fall back to the monolithic SKILLS.md
				fmt.Print(skillsDoc)
			} else {
				fmt.Print(string(overview))
			}
			os.Exit(0)
		case "mcp":
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
			desc, flagsFn, stdinReq := mcpMaps()
			os.Exit(mcppkg.Run(desc, flagsFn, stdinReq))
		case "completions":
			os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
			os.Exit(completions.Run())
		case "--bare":
			names := make([]string, 0, len(tools))
			for name := range tools {
				names = append(names, name)
			}
			sort.Strings(names)
			os.Exit(bare.Run(os.Args[2:], names))
		case "--help", "-h":
			fmt.Fprintf(os.Stderr, "usage: vrk <tool> [args]\n")
			fmt.Fprintf(os.Stderr, "       vrk --bare [--force] [--remove] [--list] [--dry-run] [tools...]\n")
			fmt.Fprintf(os.Stderr, "       vrk --manifest\n")
			fmt.Fprintf(os.Stderr, "       vrk --skills [tool]\n")
			fmt.Fprintf(os.Stderr, "\nrun 'vrk <tool> --help' for tool-specific help\n")
			os.Exit(0)
		}
	}

	// multicall: check argv[0] first (symlink mode), then argv[1] (subcommand mode)
	name := filepath.Base(os.Args[0])
	if fn, ok := tools[name]; ok {
		os.Exit(fn())
	}

	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "usage: vrk <tool> [args]\n")
		fmt.Fprintf(os.Stderr, "       vrk --bare [--force] [--remove] [--list] [--dry-run] [tools...]\n")
		fmt.Fprintf(os.Stderr, "       vrk --manifest\n")
		fmt.Fprintf(os.Stderr, "       vrk --skills [tool]\n")
		fmt.Fprintf(os.Stderr, "\nrun 'vrk <tool> --help' for tool-specific help\n")
		os.Exit(2)
	}

	name = os.Args[1]
	os.Args = append([]string{os.Args[0]}, os.Args[2:]...)
	fn, ok := tools[name]
	if !ok {
		fmt.Fprintf(os.Stderr, "vrk: unknown tool %q\n", name)
		os.Exit(2)
	}
	os.Exit(fn())
}

// printSkillFiles reads per-tool skill files from the embedded skillsFS and
// writes them to stdout. Returns 0 on full success, 1 if any file is missing.
func printSkillFiles(names []string) int {
	code := 0
	for i, name := range names {
		data, err := skillsFS.ReadFile("hugo/static/skills/" + name + ".md")
		if err != nil {
			fmt.Fprintf(os.Stderr, "vrk: no skill file for %q\n", name)
			code = 1
			continue
		}
		if i > 0 {
			fmt.Println()
		}
		fmt.Print(string(data))
	}
	return code
}

// mcpMaps builds the descriptions, flagsFn, and stdinRequired maps for mcp.
// main() calls this and passes the results to mcp.Run().
// main_test.go also calls this to verify coverage.
func mcpMaps() (
	descriptions map[string]string,
	flagsFn map[string]func() *pflag.FlagSet,
	stdinRequired map[string]bool,
) {
	// Parse descriptions from embedded manifest.json.
	var manifest struct {
		Tools []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		} `json:"tools"`
	}
	_ = json.Unmarshal([]byte(manifestJSON), &manifest)

	descriptions = make(map[string]string, len(manifest.Tools))
	for _, t := range manifest.Tools {
		descriptions[t.Name] = t.Description
	}

	flagsFn = map[string]func() *pflag.FlagSet{
		"assert":   assert.Flags,
		"base":     base.Flags,
		"chunk":    chunk.Flags,
		"coax":     coax.Flags,
		"digest":   digest.Flags,
		"emit":     emit.Flags,
		"epoch":    epoch.Flags,
		"grab":     grab.Flags,
		"jsonl":    jsonl.Flags,
		"jwt":      jwt.Flags,
		"kv":       kv.Flags,
		"links":    links.Flags,
		"mask":     mask.Flags,
		"moniker":  moniker.Flags,
		"pct":      pct.Flags,
		"plain":    plain.Flags,
		"prompt":   prompt.Flags,
		"recase":   recase.Flags,
		"sip":      sip.Flags,
		"slug":     slug.Flags,
		"sse":      sse.Flags,
		"throttle": throttle.Flags,
		"tok":      tok.Flags,
		"urlinfo":  urlinfo.Flags,
		"uuid":     uuid.Flags,
		"validate": validate.Flags,
	}

	stdinRequired = map[string]bool{
		// No stdin — generate output without input
		"uuid":    false,
		"epoch":   false,
		"moniker": false,

		// Optional stdin — ReadInputOptional, empty stdin = exit 0
		"tok":   false,
		"chunk": false,

		// Optional stdin — subcommand-dependent or optional buffering
		"coax": false, // stdin is optional buffering for subprocess
		"kv":   false, // set reads stdin only when no positional value

		// Required — stdin or positional args needed
		"jwt":      true, // single token input
		"grab":     true, // single URL input
		"slug":     true, // line-by-line text input
		"pct":      true, // line-by-line encode/decode
		"validate": true, // streaming JSONL input
		"sse":      true, // streaming SSE input
		"plain":    true, // full markdown input
		"digest":   true, // streaming bytes to hash
		"urlinfo":  true, // URL(s) to parse
		"assert":   true, // JSONL or text to check conditions against
		"mask":     true, // streaming text to redact
		"links":    true, // full text to extract links from
		"jsonl":    true, // JSON array or JSONL to convert
		"sip":      true, // streaming lines to sample
		"emit":     true, // streaming lines to wrap as log records
		"recase":   true, // line-by-line naming convention conversion
		"base":     true, // full input to encode/decode
		"throttle": true, // streaming lines to rate-limit
		"prompt":   true, // stdin text becomes the LLM prompt body
	}

	return descriptions, flagsFn, stdinRequired
}
