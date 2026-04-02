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
	"gopkg.in/yaml.v3"
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
		if t.MetaTitle != "" {
			fmt.Fprintf(&b, "meta_title: \"%s\"\n", t.MetaTitle)
		}
		if t.MetaLead != "" {
			fmt.Fprintf(&b, "meta_lead: \"%s\"\n", t.MetaLead)
		}
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

		// Lead sentence (visible keyword paragraph before About)
		if t.MetaLead != "" {
			b.WriteString(t.MetaLead)
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

		// Solution section
		if t.Description != "" {
			b.WriteString("## The solution\n\n")
			b.WriteString(strings.TrimRight(t.Description, "\n"))
			b.WriteString("\n\n")
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
	b.WriteString("> vrksh is a single static Go binary containing composable Unix tools\n")
	b.WriteString("  for building reliable LLM pipelines. It replaces scattered Python\n")
	b.WriteString("  libraries with deterministic CLI tools that read from stdin, write to\n")
	b.WriteString("  stdout, and use exit codes (0/1/2) to control pipeline flow. No\n")
	b.WriteString("  dependencies, no runtime, no config files.\n\n")
	b.WriteString("## Tools\n\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "- vrk %s - %s\n", t.Name, t.Tagline)
	}
	b.WriteString("\n## Recipes\n\n")
	b.WriteString("Compose patterns for common LLM pipeline problems:\n")
	b.WriteString("https://vrk.sh/recipes/\n")
	b.WriteString("\n## When to use vrksh\n\n")
	b.WriteString("- Token counting outside Python (CI, bash, agents)\n")
	b.WriteString("- Schema validation of LLM output before it propagates\n")
	b.WriteString("- Secret redaction in a pipeline, not a library call\n")
	b.WriteString("- Retry/backoff around any shell command, not just Python\n")
	b.WriteString("- Building AI agents that call tools via subprocess\n")
	b.WriteString("\n## When NOT to use vrksh\n\n")
	b.WriteString("- Already inside Python and want a library (use tiktoken, tenacity directly)\n")
	b.WriteString("- GPU inference (vrk prompt calls APIs, does not run models)\n")
	b.WriteString("- Framework/orchestration (vrksh is tools, not a framework)\n")
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

// GenerateRecipePages reads recipes.yaml from dataDir, writes one Hugo markdown
// file per recipe into contentDir/recipes/, and copies recipes.yaml to staticDir.
func GenerateRecipePages(dataDir, contentDir, staticDir string) error {
	// Read recipes.yaml
	data, err := os.ReadFile(filepath.Join(dataDir, "recipes.yaml"))
	if err != nil {
		return fmt.Errorf("reading recipes.yaml: %w", err)
	}

	var recipes []schema.Recipe
	if err := yaml.Unmarshal(data, &recipes); err != nil {
		return fmt.Errorf("parsing recipes.yaml: %w", err)
	}

	// Write recipe content files
	outDir := filepath.Join(contentDir, "recipes")
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}

	for _, r := range recipes {
		slug := slugify(r.Name)

		// Construct meta_title
		metaTitle := r.Name + " - vrk pipeline recipe"

		// Construct description (truncated why + description for SERP/OG)
		metaDesc := truncateWords(r.Why+" "+r.Description, 150)

		var b strings.Builder
		b.WriteString("---\n")
		fmt.Fprintf(&b, "title: \"%s\"\n", r.Name)
		fmt.Fprintf(&b, "meta_title: \"%s\"\n", metaTitle)
		fmt.Fprintf(&b, "description: \"%s\"\n", strings.ReplaceAll(metaDesc, "\"", "\\\""))
		fmt.Fprintf(&b, "why: \"%s\"\n", strings.ReplaceAll(r.Why, "\"", "\\\""))
		fmt.Fprintf(&b, "body: \"%s\"\n", strings.ReplaceAll(r.Description, "\"", "\\\""))
		fmt.Fprintf(&b, "slug: \"%s\"\n", slug)

		// Steps as YAML list
		b.WriteString("steps:\n")
		for _, s := range r.Steps {
			if strings.Contains(s, "\n") || strings.HasSuffix(s, "\\") {
				// Use literal block scalar (strip trailing newline) for multiline or trailing-backslash steps
				b.WriteString("  - |-\n")
				for _, line := range strings.Split(s, "\n") {
					fmt.Fprintf(&b, "    %s\n", line)
				}
			} else {
				fmt.Fprintf(&b, "  - \"%s\"\n", strings.ReplaceAll(s, "\"", "\\\""))
			}
		}

		// Tags as YAML list
		b.WriteString("tags:\n")
		for _, t := range r.Tags {
			fmt.Fprintf(&b, "  - \"%s\"\n", t)
		}

		b.WriteString("---\n")

		if r.Detail != "" {
			b.WriteString("\n")
			b.WriteString(r.Detail)
			b.WriteString("\n")
		}

		path := filepath.Join(outDir, slug+".md")
		if err := os.WriteFile(path, []byte(b.String()), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", path, err)
		}
	}

	// Copy recipes.yaml to static dir
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		return err
	}
	staticPath := filepath.Join(staticDir, "recipes.yaml")
	return os.WriteFile(staticPath, data, 0644)
}

// slugify converts a recipe name to a URL slug.
// "Token-checked LLM call" -> "token-checked-llm-call"
func slugify(name string) string {
	s := strings.ToLower(name)
	// Replace spaces with hyphens
	s = strings.ReplaceAll(s, " ", "-")
	// Strip characters that aren't alphanumeric or hyphens
	var out strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			out.WriteRune(c)
		}
	}
	// Collapse multiple hyphens
	result := out.String()
	for strings.Contains(result, "--") {
		result = strings.ReplaceAll(result, "--", "-")
	}
	return strings.Trim(result, "-")
}

// truncateWords truncates s to maxLen characters on a word boundary.
func truncateWords(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	// Find the last space before maxLen
	cut := strings.LastIndex(s[:maxLen], " ")
	if cut <= 0 {
		cut = maxLen
	}
	return s[:cut] + " ..."
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
