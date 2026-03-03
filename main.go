package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/vrksh/vrksh/cmd/ask"
	"github.com/vrksh/vrksh/cmd/coax"
	"github.com/vrksh/vrksh/cmd/epoch"
	"github.com/vrksh/vrksh/cmd/jwt"
	"github.com/vrksh/vrksh/cmd/kv"
	"github.com/vrksh/vrksh/cmd/sse"
	"github.com/vrksh/vrksh/cmd/tok"
	"github.com/vrksh/vrksh/cmd/uuid"
)

func main() {
	tool := filepath.Base(os.Args[0])
	if tool == "vrk" && len(os.Args) > 1 {
		tool = os.Args[1]
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}

	switch tool {
	case "jwt":
		jwt.Run()
	case "epoch":
		epoch.Run()
	case "uuid":
		uuid.Run()
	case "tok":
		tok.Run()
	case "sse":
		sse.Run()
	case "coax":
		coax.Run()
	case "ask":
		ask.Run()
	case "kv":
		kv.Run()
	default:
		fmt.Fprintf(os.Stderr, "vrk: unknown tool %q\n", tool)
		fmt.Fprintf(os.Stderr, "usage: vrk <tool> [args]\n")
		fmt.Fprintf(os.Stderr, "tools: jwt epoch uuid tok sse coax ask kv\n")
		os.Exit(2)
	}
}
