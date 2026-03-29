// Command oggen reads schema YAMLs and renders Open Graph PNGs.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vrksh/vrksh/internal/oggen"
	"github.com/vrksh/vrksh/internal/schema"
)

func main() {
	schemaDir := flag.String("schema", "schema", "path to schema YAML directory")
	outDir := flag.String("out", "hugo/static/og", "output directory for OG PNGs")
	flag.Parse()

	tools, err := schema.LoadDir(*schemaDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading schemas: %v\n", err)
		os.Exit(1)
	}

	if err := oggen.RenderAll(tools, *outDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: rendering tool PNGs: %v\n", err)
		os.Exit(1)
	}

	if err := oggen.RenderDefault(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: rendering default PNG: %v\n", err)
		os.Exit(1)
	}

	if err := oggen.RenderInstall(*outDir); err != nil {
		fmt.Fprintf(os.Stderr, "error: rendering install PNG: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("rendered %d OG images + default.png + install.png\n", len(tools))
}
