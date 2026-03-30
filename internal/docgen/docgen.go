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

		// Frontmatter (group/mcp_callable kept for site filtering, not rendered as badges)
		b.WriteString("---\n")
		fmt.Fprintf(&b, "title: \"vrk %s\"\n", t.Name)
		desc := t.Tagline
		if t.MetaDescription != "" {
			desc = t.MetaDescription
		}
		fmt.Fprintf(&b, "description: \"%s\"\n", desc)
		if t.OGTitle != "" {
			fmt.Fprintf(&b, "og_title: \"%s\"\n", t.OGTitle)
		}
		fmt.Fprintf(&b, "tool: %s\n", t.Name)
		fmt.Fprintf(&b, "group: %s\n", t.Group)
		fmt.Fprintf(&b, "mcp_callable: %v\n", t.MCPCallable)
		b.WriteString("noindex: false\n")
		b.WriteString("---\n\n")

		// Generated marker
		b.WriteString("<!-- generated - do not edit below this line -->\n\n")

		// About section (first - what this tool does)
		if t.Description != "" {
			b.WriteString("## About\n\n")
			b.WriteString(strings.TrimRight(t.Description, "\n"))
			b.WriteString("\n\n")
		}

		// Problem statement
		if t.Problem != "" {
			b.WriteString("## The problem\n\n")
			b.WriteString(strings.TrimRight(t.Problem, "\n"))
			b.WriteString("\n\n")
		} else {
			fmt.Fprintf(os.Stderr, "warning: %s: missing problem field\n", t.Name)
		}

		// Before / After section
		if t.Before != "" && t.After != "" {
			b.WriteString("## Before and after\n\n")
			b.WriteString("**Before**\n\n")
			b.WriteString("```bash\n")
			b.WriteString(strings.TrimRight(t.Before, "\n"))
			b.WriteString("\n```\n\n")
			b.WriteString("**After**\n\n")
			b.WriteString("```bash\n")
			b.WriteString(strings.TrimRight(t.After, "\n"))
			b.WriteString("\n```\n\n")
		} else {
			fmt.Fprintf(os.Stderr, "warning: %s: missing before/after fields\n", t.Name)
		}

		// Example section
		b.WriteString("## Example\n\n")
		b.WriteString("```bash\n")
		b.WriteString(t.Example)
		b.WriteString("\n```\n\n")

		// Exit codes as table (aligned)
		b.WriteString("## Exit codes\n\n")
		sortedCodes := make([]schema.ExitCode, len(t.ExitCodes))
		copy(sortedCodes, t.ExitCodes)
		sort.Slice(sortedCodes, func(i, j int) bool {
			return sortedCodes[i].Code < sortedCodes[j].Code
		})
		meanW := len("Meaning")
		for _, ec := range sortedCodes {
			if len(ec.Meaning) > meanW {
				meanW = len(ec.Meaning)
			}
		}
		fmt.Fprintf(&b, "| Code | %-*s |\n", meanW, "Meaning")
		fmt.Fprintf(&b, "|------|-%s-|\n", strings.Repeat("-", meanW))
		for _, ec := range sortedCodes {
			fmt.Fprintf(&b, "| %d    | %-*s |\n", ec.Code, meanW, ec.Meaning)
		}
		b.WriteString("\n")

		// Flags section (aligned)
		b.WriteString("## Flags\n\n")
		flagW, shortW, typeW, descW := len("Flag"), len("Short"), len("Type"), len("Description")
		for _, f := range t.Flags {
			w := len(f.Flag) + 2 // backticks
			if w > flagW {
				flagW = w
			}
			sw := len(f.Short)
			if f.Short == "" {
				sw = 1
			}
			if sw > shortW {
				shortW = sw
			}
			if len(f.Type) > typeW {
				typeW = len(f.Type)
			}
			if len(f.Description) > descW {
				descW = len(f.Description)
			}
		}
		fmt.Fprintf(&b, "| %-*s | %-*s | %-*s | %-*s |\n", flagW, "Flag", shortW, "Short", typeW, "Type", descW, "Description")
		fmt.Fprintf(&b, "|-%s-|-%s-|-%s-|-%s-|\n", strings.Repeat("-", flagW), strings.Repeat("-", shortW), strings.Repeat("-", typeW), strings.Repeat("-", descW))
		for _, f := range t.Flags {
			short := f.Short
			if short == "" {
				short = " "
			}
			name := "`" + f.Flag + "`"
			fmt.Fprintf(&b, "| %-*s | %-*s | %-*s | %-*s |\n", flagW, name, shortW, short, typeW, f.Type, descW, f.Description)
		}
		b.WriteString("\n")

		// Notes (if present)
		if notesDir != "" {
			notesPath := filepath.Join(notesDir, t.Name+".notes.md")
			notesData, err := os.ReadFile(notesPath)
			if err == nil && len(notesData) > 0 {
				b.WriteString("\n<!-- notes - edit in notes/")
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
		fmt.Fprintf(&b, "## %s - %s\n\n", t.Name, t.Tagline)
		fmt.Fprintf(&b, "Group: %s\n\n", t.Group)

		// Flags table (aligned)
		sflagW, sshortW, sdescW := len("Flag"), len("Short"), len("Description")
		for _, f := range t.Flags {
			w := len(f.Flag) + 2
			if w > sflagW {
				sflagW = w
			}
			sw := len(f.Short)
			if f.Short == "" {
				sw = 1
			}
			if sw > sshortW {
				sshortW = sw
			}
			if len(f.Description) > sdescW {
				sdescW = len(f.Description)
			}
		}
		fmt.Fprintf(&b, "| %-*s | %-*s | %-*s |\n", sflagW, "Flag", sshortW, "Short", sdescW, "Description")
		fmt.Fprintf(&b, "|-%s-|-%s-|-%s-|\n", strings.Repeat("-", sflagW), strings.Repeat("-", sshortW), strings.Repeat("-", sdescW))
		for _, f := range t.Flags {
			short := f.Short
			if short == "" {
				short = " "
			}
			name := "`" + f.Flag + "`"
			fmt.Fprintf(&b, "| %-*s | %-*s | %-*s |\n", sflagW, name, sshortW, short, sdescW, f.Description)
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

	// Compute column widths for aligned table
	nameW := len("Tool")
	descW := len("Description")
	for _, t := range tools {
		w := len(t.Name) + 2 // backticks
		if w > nameW {
			nameW = w
		}
		if len(t.Tagline) > descW {
			descW = len(t.Tagline)
		}
	}

	fmt.Fprintf(&b, "| %-*s | %-*s |\n", nameW, "Tool", descW, "Description")
	fmt.Fprintf(&b, "|-%s-|-%s-|\n", strings.Repeat("-", nameW), strings.Repeat("-", descW))
	for _, t := range tools {
		name := "`" + t.Name + "`"
		fmt.Fprintf(&b, "| %-*s | %-*s |\n", nameW, name, descW, t.Tagline)
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
		fmt.Fprintf(&b, "- vrk %s - %s\n", t.Name, t.Tagline)
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
