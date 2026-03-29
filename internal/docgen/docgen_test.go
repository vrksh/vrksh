package docgen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vrksh/vrksh/internal/schema"
)

// writeTool writes a valid tool YAML to dir.
func writeTool(t *testing.T, dir, name, group string) {
	t.Helper()
	yaml := `name: ` + name + `
tagline: The ` + name + ` tool tagline.
description: |
  The ` + name + ` tool description.
problem: >
  You need to ` + name + ` things but the existing tools are unreliable.
before: |
  python -c "print('old way')"
after: "cat input.txt | vrk ` + name + `"
group: ` + group + `
example: cat input.txt | vrk ` + name + ` --json
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: Emit JSON output
  - flag: --budget
    short: ""
    type: int
    description: Token budget limit
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Runtime error
  - code: 2
    meaning: Usage error
og_image:
  headline: "The ` + name + ` tool tagline."
  code: "cat input.txt | vrk ` + name + ` --json"
mcp_callable: true
`
	err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(yaml), 0644)
	if err != nil {
		t.Fatalf("writing %s.yaml: %v", name, err)
	}
}

func TestGenerateToolDoc(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok", "v1")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := GenerateToolDocs(tools, "", outDir); err != nil {
		t.Fatalf("GenerateToolDocs: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "tok.md"))
	if err != nil {
		t.Fatalf("reading tok.md: %v", err)
	}
	s := string(content)

	// Check frontmatter
	if !strings.Contains(s, `title: "vrk tok"`) {
		t.Error("missing title frontmatter")
	}
	if !strings.Contains(s, `tool: tok`) {
		t.Error("missing tool frontmatter")
	}
	if !strings.Contains(s, `group: v1`) {
		t.Error("missing group frontmatter")
	}
	if !strings.Contains(s, `mcp_callable: true`) {
		t.Error("missing mcp_callable frontmatter")
	}

	// Check generated marker
	if !strings.Contains(s, "<!-- generated") {
		t.Error("missing <!-- generated --> marker")
	}

	// Check problem statement
	if !strings.Contains(s, "## The problem") {
		t.Error("missing problem section")
	}
	if !strings.Contains(s, "You need to tok things") {
		t.Error("missing problem content")
	}

	// Contract section should NOT be present (removed)
	if strings.Contains(s, "## Contract") {
		t.Error("Contract section should not be present")
	}

	// Check exit codes table
	if !strings.Contains(s, "## Exit codes") {
		t.Error("missing Exit codes section")
	}
	if !strings.Contains(s, "| Code | Meaning |") {
		t.Error("missing exit codes table header")
	}
	if !strings.Contains(s, "| 0 | Success |") {
		t.Error("missing exit code 0 in table")
	}

	// Check flags table
	if !strings.Contains(s, "## Flags") {
		t.Error("missing Flags section")
	}
	if !strings.Contains(s, "`--json`") {
		t.Error("missing --json in flags table")
	}
	if !strings.Contains(s, "`--budget`") {
		t.Error("missing --budget in flags table")
	}

	// Check example
	if !strings.Contains(s, "## Example") {
		t.Error("missing Example section")
	}
	if !strings.Contains(s, "vrk tok --json") {
		t.Error("missing example command")
	}

	// Check before/after section
	if !strings.Contains(s, "## Before and after") {
		t.Error("missing before/after section")
	}
	if !strings.Contains(s, "**Before**") {
		t.Error("missing Before label")
	}
	if !strings.Contains(s, "**After**") {
		t.Error("missing After label")
	}

	// Check about section
	if !strings.Contains(s, "## About") {
		t.Error("missing About section")
	}
	if !strings.Contains(s, "The tok tool description.") {
		t.Error("missing description in About section")
	}

	// Check section order: about → problem → before/after → example → exit codes → flags
	posAbout := strings.Index(s, "## About")
	posProblem := strings.Index(s, "## The problem")
	posBeforeAfter := strings.Index(s, "## Before and after")
	posExample := strings.Index(s, "## Example")
	posExit := strings.Index(s, "## Exit codes")
	posFlags := strings.Index(s, "## Flags")
	if posAbout >= posProblem {
		t.Error("about must come before problem")
	}
	if posProblem >= posBeforeAfter {
		t.Error("problem must come before before/after")
	}
	if posBeforeAfter >= posExample {
		t.Error("before/after must come before example")
	}
	if posExample >= posExit {
		t.Error("example must come before exit codes")
	}
	if posExit >= posFlags {
		t.Error("exit codes must come before flags")
	}
}

func TestGenerateToolDocWithNotes(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok", "v1")

	notes := "## Gotchas\n\n- Token counts are approximate for Claude models.\n"
	err := os.WriteFile(filepath.Join(schemaDir, "tok.notes.md"), []byte(notes), 0644)
	if err != nil {
		t.Fatalf("writing notes: %v", err)
	}

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := GenerateToolDocs(tools, schemaDir, outDir); err != nil {
		t.Fatalf("GenerateToolDocs: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "tok.md"))
	if err != nil {
		t.Fatalf("reading tok.md: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "Token counts are approximate for Claude models.") {
		t.Error("notes content not appended")
	}
	if !strings.Contains(s, "<!-- notes") {
		t.Error("missing notes marker")
	}
}

func TestGenerateToolDocWithoutNotes(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok", "v1")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	// No notes file - notesDir is schemaDir but no .notes.md exists
	if err := GenerateToolDocs(tools, schemaDir, outDir); err != nil {
		t.Fatalf("GenerateToolDocs: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "tok.md"))
	if err != nil {
		t.Fatalf("reading tok.md: %v", err)
	}
	s := string(content)

	// Should not have notes marker when no notes exist
	if strings.Contains(s, "<!-- notes") {
		t.Error("notes marker present when no notes file exists")
	}
}

func TestGenerateSkillsMD(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok", "v1")
	writeTool(t, schemaDir, "prompt", "v1")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := GenerateSkillsMD(tools, outDir); err != nil {
		t.Fatalf("GenerateSkillsMD: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "skills.md"))
	if err != nil {
		t.Fatalf("reading skills.md: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "## prompt") {
		t.Error("missing prompt section")
	}
	if !strings.Contains(s, "## tok") {
		t.Error("missing tok section")
	}
	// Check flags table present
	if !strings.Contains(s, "`--json`") {
		t.Error("missing flag in skills.md")
	}
	// Check exit codes present
	if !strings.Contains(s, "Exit 0") || !strings.Contains(s, "Exit 1") {
		t.Error("missing exit codes in skills.md")
	}
}

func TestGenerateSkillsMDTierOrdered(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	// Write tools in different groups, out of tier order
	writeTool(t, schemaDir, "later", "v2")
	writeTool(t, schemaDir, "first", "v1")
	writeTool(t, schemaDir, "last", "v3")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := GenerateSkillsMD(tools, outDir); err != nil {
		t.Fatalf("GenerateSkillsMD: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "skills.md"))
	if err != nil {
		t.Fatalf("reading skills.md: %v", err)
	}
	s := string(content)

	// v1 tool should appear before v2, v2 before v3
	posV1 := strings.Index(s, "## first")
	posV2 := strings.Index(s, "## later")
	posV3 := strings.Index(s, "## last")

	if posV1 == -1 || posV2 == -1 || posV3 == -1 {
		t.Fatal("missing tool sections in skills.md")
	}
	if posV1 >= posV2 {
		t.Error("v1 tool should appear before v2 tool")
	}
	if posV2 >= posV3 {
		t.Error("v2 tool should appear before v3 tool")
	}
}

func TestGenerateAgentsMD(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok", "v1")
	writeTool(t, schemaDir, "jwt", "v1")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := GenerateAgentsMD(tools, outDir); err != nil {
		t.Fatalf("GenerateAgentsMD: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "agents.md"))
	if err != nil {
		t.Fatalf("reading agents.md: %v", err)
	}
	s := string(content)

	// Should have one line per tool
	if !strings.Contains(s, "jwt") {
		t.Error("missing jwt in agents.md")
	}
	if !strings.Contains(s, "tok") {
		t.Error("missing tok in agents.md")
	}
}

func TestGenerateLLMsTxt(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok", "v1")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := GenerateLLMsTxt(tools, outDir); err != nil {
		t.Fatalf("GenerateLLMsTxt: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "llms.txt"))
	if err != nil {
		t.Fatalf("reading llms.txt: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, "vrk") {
		t.Error("missing vrk mention in llms.txt")
	}
	if !strings.Contains(s, "tok") {
		t.Error("missing tok in llms.txt")
	}
	// Should point to skills.md for full reference
	if !strings.Contains(s, "skills.md") {
		t.Error("missing skills.md pointer in llms.txt")
	}
}

func TestGenerateToolData(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok", "v1")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := GenerateToolData(tools, outDir); err != nil {
		t.Fatalf("GenerateToolData: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outDir, "tok.json"))
	if err != nil {
		t.Fatalf("reading tok.json: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `"name":"tok"`) && !strings.Contains(s, `"name": "tok"`) {
		t.Error("missing name field in tool data JSON")
	}
	if !strings.Contains(s, "tagline") {
		t.Error("missing tagline field in tool data JSON")
	}
}

func TestGenerateManifest(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok", "v1")
	writeTool(t, schemaDir, "jwt", "v1")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := UpdateManifest(tools, manifestPath); err != nil {
		t.Fatalf("UpdateManifest: %v", err)
	}

	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatalf("reading manifest.json: %v", err)
	}
	s := string(content)

	if !strings.Contains(s, `"name"`) {
		t.Error("missing name in manifest")
	}
	if !strings.Contains(s, "tok") {
		t.Error("missing tok in manifest")
	}
	if !strings.Contains(s, "jwt") {
		t.Error("missing jwt in manifest")
	}
}

func TestUpdateManifestPreservesExisting(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	// Write an existing manifest with an extra tool
	existing := `{
  "version": "0.1.0",
  "tools": [
    {"name": "epoch", "description": "Timestamp converter"},
    {"name": "tok", "description": "OLD description"}
  ]
}
`
	manifestPath := filepath.Join(outDir, "manifest.json")
	if err := os.WriteFile(manifestPath, []byte(existing), 0644); err != nil {
		t.Fatal(err)
	}

	writeTool(t, schemaDir, "tok", "v1")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := UpdateManifest(tools, manifestPath); err != nil {
		t.Fatalf("UpdateManifest: %v", err)
	}

	content, err := os.ReadFile(manifestPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(content)

	// epoch should be preserved
	if !strings.Contains(s, "epoch") {
		t.Error("existing tool 'epoch' was lost during merge")
	}
	// tok description should be updated from schema
	if strings.Contains(s, "OLD description") {
		t.Error("tok description was not updated from schema")
	}
	if !strings.Contains(s, "The tok tool tagline.") {
		t.Error("tok description not updated to schema tagline")
	}
}
