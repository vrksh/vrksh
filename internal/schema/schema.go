// Package schema defines the canonical tool schema and loads/validates YAML files.
package schema

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Tool is the canonical representation of a vrksh tool.
type Tool struct {
	Name            string     `yaml:"name"`
	Tagline         string     `yaml:"tagline"`
	MetaDescription string     `yaml:"meta_description"`
	MetaTitle       string     `yaml:"meta_title"`
	MetaLead        string     `yaml:"meta_lead"`
	OGTitle         string     `yaml:"og_title"`
	Description     string     `yaml:"description"`
	Problem         string     `yaml:"problem"`
	Before          string     `yaml:"before"`
	After           string     `yaml:"after"`
	Group           string     `yaml:"group"`
	Category        string     `yaml:"category"`
	Example         string     `yaml:"example"`
	Flags           []Flag     `yaml:"flags"`
	ExitCodes       []ExitCode `yaml:"exit_codes"`
	OGImage         OGImage    `yaml:"og_image"`
	MCPCallable     bool       `yaml:"mcp_callable"`
}

// Flag describes a single CLI flag.
type Flag struct {
	Flag        string `yaml:"flag"`
	Short       string `yaml:"short"`
	Type        string `yaml:"type"`
	Description string `yaml:"description"`
}

// ExitCode documents a tool's exit code semantics.
type ExitCode struct {
	Code    int    `yaml:"code"`
	Meaning string `yaml:"meaning"`
}

// OGImage holds the data for generating an Open Graph image.
type OGImage struct {
	Headline string `yaml:"headline"`
	Code     string `yaml:"code"`
}

// Recipe is a compose pattern from recipes.yaml.
type Recipe struct {
	Name        string   `yaml:"name"`
	Why         string   `yaml:"why"`
	Description string   `yaml:"description"`
	Steps       []string `yaml:"steps"`
	Tags        []string `yaml:"tags"`
}

var validGroups = map[string]bool{"v1": true, "v2": true, "v3": true}
var validCategories = map[string]bool{"core": true, "pipeline": true, "utilities": true, "meta": true}

// LoadDir reads all *.yaml files from dir, parses them as Tool structs,
// and returns them sorted alphabetically by name.
func LoadDir(dir string) ([]Tool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("reading schema dir: %w", err)
	}

	var tools []Tool
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", e.Name(), err)
		}
		var t Tool
		if err := yaml.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("parsing %s: %w", e.Name(), err)
		}
		tools = append(tools, t)
	}

	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Name < tools[j].Name
	})

	return tools, nil
}

// Validate checks that a Tool satisfies all required constraints.
func Validate(t Tool) error {
	if t.Name == "" {
		return fmt.Errorf("name is required")
	}
	if t.Tagline == "" {
		return fmt.Errorf("%s: tagline is required", t.Name)
	}
	if t.Example == "" {
		return fmt.Errorf("%s: example is required", t.Name)
	}
	if !validGroups[t.Group] {
		return fmt.Errorf("%s: group must be v1, v2, or v3 (got %q)", t.Name, t.Group)
	}
	if !validCategories[t.Category] {
		return fmt.Errorf("%s: category must be core, pipeline, utilities, or meta (got %q)", t.Name, t.Category)
	}
	if len(t.Flags) == 0 {
		return fmt.Errorf("%s: at least one flag is required", t.Name)
	}

	hasCode0, hasCode1 := false, false
	for _, ec := range t.ExitCodes {
		if ec.Code == 0 {
			hasCode0 = true
		}
		if ec.Code == 1 {
			hasCode1 = true
		}
	}
	if !hasCode0 {
		return fmt.Errorf("%s: exit_codes must include code 0", t.Name)
	}
	if !hasCode1 {
		return fmt.Errorf("%s: exit_codes must include code 1", t.Name)
	}

	if t.OGImage.Headline == "" {
		return fmt.Errorf("%s: og_image.headline is required", t.Name)
	}
	if t.OGImage.Code == "" {
		return fmt.Errorf("%s: og_image.code is required", t.Name)
	}
	// og_image.code must look like a shell command (starts with vrk or contains a pipe)
	if !strings.HasPrefix(t.OGImage.Code, "vrk") && !strings.Contains(t.OGImage.Code, "|") {
		return fmt.Errorf("%s: og_image.code must start with 'vrk' or contain a pipe", t.Name)
	}

	return nil
}
