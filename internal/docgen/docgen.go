// Package docgen reads tool schema YAMLs and generates Hugo content,
// skills.md, agents.md, llms.txt, tool data JSON, and manifest updates.
package docgen

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vrksh/vrksh/internal/schema"
)

// GenerateToolDocs writes one Hugo markdown file per tool into outDir.
// If notesDir is non-empty, it looks for <tool>.notes.md and appends the content.
func GenerateToolDocs(tools []schema.Tool, notesDir, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	for _, t := range tools {
		var b strings.Builder

		// Frontmatter
		b.WriteString("---\n")
		fmt.Fprintf(&b, "title: \"vrk %s\"\n", t.Name)
		fmt.Fprintf(&b, "description: \"%s\"\n", t.Tagline)
		fmt.Fprintf(&b, "tool: %s\n", t.Name)
		fmt.Fprintf(&b, "group: %s\n", t.Group)
		fmt.Fprintf(&b, "mcp_callable: %v\n", t.MCPCallable)
		b.WriteString("noindex: false\n")
		b.WriteString("---\n\n")

		// Generated marker
		b.WriteString("<!-- generated — do not edit below this line -->\n\n")

		// Contract section
		b.WriteString("## Contract\n\n")
		fmt.Fprintf(&b, "`stdin → %s → stdout`\n\n", t.Name)
		for i, ec := range t.ExitCodes {
			if i > 0 {
				b.WriteString(" · ")
			}
			fmt.Fprintf(&b, "Exit %d %s", ec.Code, ec.Meaning)
		}
		b.WriteString("\n\n")

		// Flags section
		b.WriteString("## Flags\n\n")
		b.WriteString("| Flag | Short | Type | Description |\n")
		b.WriteString("|------|-------|------|-------------|\n")
		for _, f := range t.Flags {
			short := f.Short
			if short == "" {
				short = " "
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", f.Flag, short, f.Type, f.Description)
		}
		b.WriteString("\n")

		// Example section
		b.WriteString("## Example\n\n")
		b.WriteString("```bash\n")
		b.WriteString(t.Example)
		b.WriteString("\n```\n")

		// Notes (if present)
		if notesDir != "" {
			notesPath := filepath.Join(notesDir, t.Name+".notes.md")
			notesData, err := os.ReadFile(notesPath)
			if err == nil && len(notesData) > 0 {
				b.WriteString("\n<!-- notes — edit in schema/")
				b.WriteString(t.Name)
				b.WriteString(".notes.md -->\n\n")
				b.WriteString(strings.TrimRight(string(notesData), "\n"))
				b.WriteString("\n")
			}
		}

		path := filepath.Join(outDir, t.Name+".md")
		if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	return nil
}

// GenerateSkillsMD writes a machine-readable skills index, tier-ordered.
func GenerateSkillsMD(tools []schema.Tool, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	// Sort by group (v1 < v2 < v3), then by name within group
	sorted := make([]schema.Tool, len(tools))
	copy(sorted, tools)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Group != sorted[j].Group {
			return sorted[i].Group < sorted[j].Group
		}
		return sorted[i].Name < sorted[j].Name
	})

	var b strings.Builder
	b.WriteString("# vrksh skills reference\n\n")
	b.WriteString("Machine-readable tool reference. One section per tool.\n\n")

	for _, t := range sorted {
		fmt.Fprintf(&b, "## %s — %s\n\n", t.Name, t.Tagline)
		fmt.Fprintf(&b, "Group: %s\n\n", t.Group)

		// Flags table
		b.WriteString("| Flag | Short | Description |\n")
		b.WriteString("|------|-------|-------------|\n")
		for _, f := range t.Flags {
			short := f.Short
			if short == "" {
				short = " "
			}
			fmt.Fprintf(&b, "| `%s` | %s | %s |\n", f.Flag, short, f.Description)
		}
		b.WriteString("\n")

		// Exit codes
		for _, ec := range t.ExitCodes {
			fmt.Fprintf(&b, "Exit %d: %s\n", ec.Code, ec.Meaning)
		}
		b.WriteString("\n")

		// Example
		b.WriteString("```bash\n")
		b.WriteString(t.Example)
		b.WriteString("\n```\n\n")
	}

	path := filepath.Join(outDir, "skills.md")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// GenerateAgentsMD writes a CLAUDE.md snippet with one line per tool.
func GenerateAgentsMD(tools []schema.Tool, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# vrksh tools\n\n")
	b.WriteString("One binary: `vrk`. Unix tools for AI pipelines.\n\n")

	b.WriteString("| Tool | Description |\n")
	b.WriteString("|------|-------------|\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "| `%s` | %s |\n", t.Name, t.Tagline)
	}
	b.WriteString("\n")
	b.WriteString("Full reference: `vrk --skills` or https://vrk.sh/skills.md\n")

	path := filepath.Join(outDir, "agents.md")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// GenerateLLMsTxt writes an LLM discovery file.
func GenerateLLMsTxt(tools []schema.Tool, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	var b strings.Builder
	b.WriteString("# vrksh\n\n")
	b.WriteString("> Unix tools for AI pipelines. One static Go binary.\n\n")
	b.WriteString("## Tools\n\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "- vrk %s — %s\n", t.Name, t.Tagline)
	}
	b.WriteString("\n## Full reference\n\n")
	b.WriteString("For complete flag documentation, exit codes, and compose patterns:\n")
	b.WriteString("https://vrk.sh/skills.md\n")

	path := filepath.Join(outDir, "llms.txt")
	return os.WriteFile(path, []byte(b.String()), 0644)
}

// GenerateToolData writes one JSON file per tool into outDir (for Hugo data templates).
func GenerateToolData(tools []schema.Tool, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	for _, t := range tools {
		data := map[string]any{
			"name":         t.Name,
			"tagline":      t.Tagline,
			"group":        t.Group,
			"mcp_callable": t.MCPCallable,
			"example":      t.Example,
		}
		raw, err := json.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshalling %s: %w", t.Name, err)
		}
		path := filepath.Join(outDir, t.Name+".json")
		if err := os.WriteFile(path, raw, 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	return nil
}

// manifestEntry is one tool in the manifest.
type manifestEntry struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// manifestFile is the top-level manifest structure.
type manifestFile struct {
	Version string          `json:"version"`
	Tools   []manifestEntry `json:"tools"`
}

// UpdateManifest merges schema tool entries into an existing manifest.json.
// Existing entries not in the schema set are preserved.
func UpdateManifest(tools []schema.Tool, path string) error {
	// Read existing manifest if present
	var m manifestFile
	existing, err := os.ReadFile(path)
	if err == nil {
		_ = json.Unmarshal(existing, &m)
	}
	if m.Version == "" {
		m.Version = "0.1.0"
	}

	// Build map of schema tools (overrides)
	overrides := make(map[string]string)
	for _, t := range tools {
		overrides[t.Name] = t.Tagline
	}

	// Update existing entries, track which schema tools are already present
	seen := make(map[string]bool)
	for i, e := range m.Tools {
		if desc, ok := overrides[e.Name]; ok {
			m.Tools[i].Description = desc
			seen[e.Name] = true
		}
	}

	// Append new schema tools not already in manifest
	for _, t := range tools {
		if !seen[t.Name] {
			m.Tools = append(m.Tools, manifestEntry{
				Name:        t.Name,
				Description: t.Tagline,
			})
		}
	}

	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
