// Command docgen reads schema YAMLs and generates Hugo content files.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/vrksh/vrksh/internal/docgen"
	"github.com/vrksh/vrksh/internal/schema"
)

func main() {
	schemaDir := flag.String("schema", "schema", "path to schema YAML directory")
	hugoContent := flag.String("content", "hugo/content/docs", "output dir for Hugo doc pages")
	hugoStatic := flag.String("static", "hugo/static", "output dir for static files (skills.md, agents.md, llms.txt)")
	hugoData := flag.String("data", "hugo/data/tools", "output dir for Hugo data JSON files")
	manifest := flag.String("manifest", "manifest.json", "path to manifest.json to update")
	flag.Parse()

	tools, err := schema.LoadDir(*schemaDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading schemas: %v\n", err)
		os.Exit(1)
	}

	for _, t := range tools {
		if err := schema.Validate(t); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	}

	if err := docgen.GenerateToolDocs(tools, *schemaDir, *hugoContent); err != nil {
		fmt.Fprintf(os.Stderr, "error: generating tool docs: %v\n", err)
		os.Exit(1)
	}

	if err := docgen.GenerateSkillsMD(tools, *hugoStatic); err != nil {
		fmt.Fprintf(os.Stderr, "error: generating skills.md: %v\n", err)
		os.Exit(1)
	}

	if err := docgen.GenerateAgentsMD(tools, *hugoStatic); err != nil {
		fmt.Fprintf(os.Stderr, "error: generating agents.md: %v\n", err)
		os.Exit(1)
	}

	if err := docgen.GenerateLLMsTxt(tools, *hugoStatic); err != nil {
		fmt.Fprintf(os.Stderr, "error: generating llms.txt: %v\n", err)
		os.Exit(1)
	}

	if err := docgen.GenerateToolData(tools, *hugoData); err != nil {
		fmt.Fprintf(os.Stderr, "error: generating tool data: %v\n", err)
		os.Exit(1)
	}

	if err := docgen.UpdateManifest(tools, *manifest); err != nil {
		fmt.Fprintf(os.Stderr, "error: updating manifest: %v\n", err)
		os.Exit(1)
	}

	// Copy manifest to Hugo static dir so it's served at /manifest.json
	manifestData, err := os.ReadFile(*manifest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading manifest for copy: %v\n", err)
		os.Exit(1)
	}
	staticManifest := fmt.Sprintf("%s/manifest.json", *hugoStatic)
	if err := os.WriteFile(staticManifest, manifestData, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error: copying manifest to static: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("generated docs for %d tools\n", len(tools))
}
