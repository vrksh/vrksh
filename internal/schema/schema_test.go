package schema

import (
	"os"
	"path/filepath"
	"testing"
)

// validToolYAML returns a complete, valid tool YAML string.
func validToolYAML() string {
	return `name: tok
tagline: Count tokens. Guard LLM calls with a budget.
description: |
  Token counter and budget guard. Uses cl100k_base by default.
group: v1
category: core
example: cat prompt.txt | vrk tok --budget 4000 --fail
flags:
  - flag: --budget
    short: ""
    type: int
    description: Exit 1 if token count exceeds N
  - flag: --json
    short: "-j"
    type: bool
    description: Emit JSON with token count and metadata
exit_codes:
  - code: 0
    meaning: Success, within budget
  - code: 1
    meaning: Over budget or I/O error
  - code: 2
    meaning: Usage error
og_image:
  headline: "Count tokens. Guard LLM calls with a budget."
  code: "cat prompt.txt | vrk tok --budget 4000 --fail"
mcp_callable: true
`
}

func writeYAML(t *testing.T, dir, name, content string) {
	t.Helper()
	err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644)
	if err != nil {
		t.Fatalf("writing %s: %v", name, err)
	}
}

func TestLoadAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "tok.yaml", validToolYAML())

	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}

	tok := tools[0]
	if tok.Name != "tok" {
		t.Errorf("name: got %q, want %q", tok.Name, "tok")
	}
	if tok.Tagline != "Count tokens. Guard LLM calls with a budget." {
		t.Errorf("tagline: got %q", tok.Tagline)
	}
	if tok.Group != "v1" {
		t.Errorf("group: got %q, want %q", tok.Group, "v1")
	}
	if !tok.MCPCallable {
		t.Error("mcp_callable: expected true")
	}
	if len(tok.Flags) != 2 {
		t.Errorf("flags: got %d, want 2", len(tok.Flags))
	}
	if tok.Flags[0].Flag != "--budget" {
		t.Errorf("flag[0]: got %q, want %q", tok.Flags[0].Flag, "--budget")
	}
	if tok.OGImage.Headline != "Count tokens. Guard LLM calls with a budget." {
		t.Errorf("og_image.headline: got %q", tok.OGImage.Headline)
	}
	if tok.OGImage.Code != "cat prompt.txt | vrk tok --budget 4000 --fail" {
		t.Errorf("og_image.code: got %q", tok.OGImage.Code)
	}
}

func TestLoadMultipleFiles(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "tok.yaml", validToolYAML())

	// Write a second valid YAML with different name
	jwt := `name: jwt
tagline: Decode and inspect JWTs.
description: JWT decoder.
group: v1
category: core
example: vrk jwt eyJhbGciOiJIUzI1NiJ9...
flags:
  - flag: --claim
    short: "-c"
    type: string
    description: Extract a single claim
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Invalid token
  - code: 2
    meaning: Usage error
og_image:
  headline: "Decode and inspect JWTs."
  code: "vrk jwt eyJhbGciOiJIUzI1NiJ9..."
mcp_callable: true
`
	writeYAML(t, dir, "jwt.yaml", jwt)

	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("expected 2 tools, got %d", len(tools))
	}
}

func TestLoadIgnoresNonYAML(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "tok.yaml", validToolYAML())
	writeYAML(t, dir, "tok.notes.md", "some notes")
	writeYAML(t, dir, "readme.txt", "not a tool")

	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(tools) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(tools))
	}
}

func TestValidateAcceptsValidTool(t *testing.T) {
	dir := t.TempDir()
	writeYAML(t, dir, "tok.yaml", validToolYAML())

	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err != nil {
		t.Errorf("Validate rejected valid tool: %v", err)
	}
}

func TestValidateRejectsMissingName(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: ""
tagline: Some tagline.
description: Some description.
group: v1
category: core
example: vrk foo bar
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: "Some headline"
  code: "vrk foo bar"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for missing name")
	}
}

func TestValidateRejectsMissingTagline(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: ""
description: Some description.
group: v1
category: core
example: vrk tok foo
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: "Some headline"
  code: "vrk tok foo"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for missing tagline")
	}
}

func TestValidateRejectsMissingExample(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: Some tagline.
description: Some description.
group: v1
category: core
example: ""
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: "Some headline"
  code: "vrk tok foo"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for missing example")
	}
}

func TestValidateRejectsMissingOGHeadline(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: Some tagline.
description: Some description.
group: v1
category: core
example: vrk tok foo
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: ""
  code: "vrk tok foo"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for missing og_image.headline")
	}
}

func TestValidateRejectsMissingOGCode(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: Some tagline.
description: Some description.
group: v1
category: core
example: vrk tok foo
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: "Some headline"
  code: ""
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for missing og_image.code")
	}
}

func TestValidateRejectsInvalidGroup(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: Some tagline.
description: Some description.
group: v4
example: vrk tok foo
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: "Some headline"
  code: "vrk tok foo"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for invalid group v4")
	}
}

func TestValidateRejectsMissingExitCode0(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: Some tagline.
description: Some description.
group: v1
category: core
example: vrk tok foo
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 1
    meaning: Error
og_image:
  headline: "Some headline"
  code: "vrk tok foo"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for missing exit code 0")
	}
}

func TestValidateRejectsMissingExitCode1(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: Some tagline.
description: Some description.
group: v1
category: core
example: vrk tok foo
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
og_image:
  headline: "Some headline"
  code: "vrk tok foo"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for missing exit code 1")
	}
}

func TestValidateRejectsEmptyFlags(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: Some tagline.
description: Some description.
group: v1
category: core
example: vrk tok foo
flags: []
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: "Some headline"
  code: "vrk tok foo"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for empty flags")
	}
}

func TestValidateRejectsOGCodeNotShellCommand(t *testing.T) {
	dir := t.TempDir()
	yaml := `name: tok
tagline: Some tagline.
description: Some description.
group: v1
category: core
example: vrk tok foo
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: "Some headline"
  code: "just some random text"
mcp_callable: true
`
	writeYAML(t, dir, "bad.yaml", yaml)
	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if err := Validate(tools[0]); err == nil {
		t.Error("expected validation error for og_image.code not a shell command")
	}
}

func TestLoadDirSortsAlphabetically(t *testing.T) {
	dir := t.TempDir()
	// Write tools in reverse alphabetical order
	makeMinimal := func(name string) string {
		return `name: ` + name + `
tagline: Test tool.
description: Test.
group: v1
category: core
example: vrk ` + name + ` foo
flags:
  - flag: --json
    short: "-j"
    type: bool
    description: JSON output
exit_codes:
  - code: 0
    meaning: Success
  - code: 1
    meaning: Error
og_image:
  headline: "Test."
  code: "vrk ` + name + ` foo"
mcp_callable: true
`
	}
	writeYAML(t, dir, "z.yaml", makeMinimal("z"))
	writeYAML(t, dir, "a.yaml", makeMinimal("a"))
	writeYAML(t, dir, "m.yaml", makeMinimal("m"))

	tools, err := LoadDir(dir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}
	if len(tools) != 3 {
		t.Fatalf("expected 3 tools, got %d", len(tools))
	}
	if tools[0].Name != "a" || tools[1].Name != "m" || tools[2].Name != "z" {
		t.Errorf("expected sorted order [a, m, z], got [%s, %s, %s]",
			tools[0].Name, tools[1].Name, tools[2].Name)
	}
}
