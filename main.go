package main

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/vrksh/vrksh/cmd/coax"
	"github.com/vrksh/vrksh/cmd/epoch"
	"github.com/vrksh/vrksh/cmd/jwt"
	"github.com/vrksh/vrksh/cmd/kv"
	"github.com/vrksh/vrksh/cmd/prompt"
	"github.com/vrksh/vrksh/cmd/sse"
	"github.com/vrksh/vrksh/cmd/tok"
	"github.com/vrksh/vrksh/cmd/uuid"
)

//go:embed integrations/skills/SKILLS.md
var skillsDoc string

//go:embed manifest.json
var manifestJSON string

var tools = map[string]func() int{
	"jwt":    jwt.Run,
	"epoch":  epoch.Run,
	"uuid":   uuid.Run,
	"tok":    tok.Run,
	"sse":    sse.Run,
	"coax":   coax.Run,
	"prompt": prompt.Run,
	"kv":     kv.Run,
}

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "--manifest":
			fmt.Print(manifestJSON)
			os.Exit(0)
		case "--skills":
			fmt.Print(skillsDoc)
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
