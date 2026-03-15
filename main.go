package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	"github.com/vrksh/vrksh/cmd/plain"
	"github.com/vrksh/vrksh/cmd/prompt"
	"github.com/vrksh/vrksh/cmd/recase"
	"github.com/vrksh/vrksh/cmd/slug"
	"github.com/vrksh/vrksh/cmd/sse"
	"github.com/vrksh/vrksh/cmd/throttle"
	"github.com/vrksh/vrksh/cmd/tok"
	"github.com/vrksh/vrksh/cmd/uuid"
	"github.com/vrksh/vrksh/cmd/validate"
)

//go:embed integrations/skills/SKILLS.md
var skillsDoc string

//go:embed manifest.json
var manifestJSON string

var tools = map[string]func() int{
	"base":     base.Run,
	"chunk":    chunk.Run,
	"digest":   digest.Run,
	"emit":     emit.Run,
	"grab":     grab.Run,
	"jsonl":    jsonl.Run,
	"jwt":      jwt.Run,
	"epoch":    epoch.Run,
	"uuid":     uuid.Run,
	"tok":      tok.Run,
	"sse":      sse.Run,
	"coax":     coax.Run,
	"prompt":   prompt.Run,
	"kv":       kv.Run,
	"links":    links.Run,
	"mask":     mask.Run,
	"recase":   recase.Run,
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
				fmt.Print(skillsSection(os.Args[2]))
			} else {
				fmt.Print(skillsDoc)
			}
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

// skillsSection returns the SKILLS.md section for a single tool.
// Sections start with "## <tool>" and end before the next "## " heading.
// Falls back to the full document if the tool is not found.
func skillsSection(tool string) string {
	prefix := "## " + tool
	lines := strings.Split(skillsDoc, "\n")
	start := -1
	for i, line := range lines {
		if strings.HasPrefix(line, prefix) {
			start = i
			break
		}
	}
	if start == -1 {
		return skillsDoc
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n") + "\n"
}
