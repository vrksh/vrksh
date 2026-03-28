// Command validate checks all schema YAML files in a directory.
package main

import (
	"fmt"
	"os"

	"github.com/vrksh/vrksh/internal/schema"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: validate <schema-dir>")
		os.Exit(2)
	}
	dir := os.Args[1]

	tools, err := schema.LoadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: loading schemas: %v\n", err)
		os.Exit(1)
	}

	if len(tools) == 0 {
		fmt.Fprintf(os.Stderr, "error: no YAML files found in %s\n", dir)
		os.Exit(1)
	}

	for _, t := range tools {
		if err := schema.Validate(t); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("ok  %s\n", t.Name)
	}
	fmt.Printf("\n%d schemas valid\n", len(tools))
}
