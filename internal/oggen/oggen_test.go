package oggen

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/vrksh/vrksh/internal/schema"
)

func writeTool(t *testing.T, dir, name string) {
	t.Helper()
	yaml := `name: ` + name + `
tagline: The ` + name + ` tool tagline.
description: Test tool.
group: v1
category: core
example: cat input.txt | vrk ` + name + ` --json
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
  headline: "The ` + name + ` tool headline."
  code: "cat input.txt | vrk ` + name + ` --json"
mcp_callable: true
`
	err := os.WriteFile(filepath.Join(dir, name+".yaml"), []byte(yaml), 0644)
	if err != nil {
		t.Fatalf("writing %s.yaml: %v", name, err)
	}
}

func TestRenderToolPNG(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := RenderAll(tools, outDir); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}

	path := filepath.Join(outDir, "tok.png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading tok.png: %v", err)
	}

	// Check PNG magic bytes
	if !bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		t.Error("tok.png does not have valid PNG magic bytes")
	}

	// Decode and check dimensions
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoding tok.png: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("expected 1200x630, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderDefaultPNG(t *testing.T) {
	outDir := t.TempDir()

	if err := RenderDefault(outDir); err != nil {
		t.Fatalf("RenderDefault: %v", err)
	}

	path := filepath.Join(outDir, "default.png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading default.png: %v", err)
	}

	// Check PNG magic bytes
	if !bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		t.Error("default.png does not have valid PNG magic bytes")
	}

	// Decode and check dimensions
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoding default.png: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("expected 1200x630, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderInstallPNG(t *testing.T) {
	outDir := t.TempDir()

	if err := RenderInstall(outDir); err != nil {
		t.Fatalf("RenderInstall: %v", err)
	}

	path := filepath.Join(outDir, "install.png")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading install.png: %v", err)
	}

	if !bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}) {
		t.Error("install.png does not have valid PNG magic bytes")
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoding install.png: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1200 || bounds.Dy() != 630 {
		t.Errorf("expected 1200x630, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestRenderedPNGNotSolidColor(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := RenderAll(tools, outDir); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(outDir, "tok.png"))
	if err != nil {
		t.Fatalf("reading tok.png: %v", err)
	}

	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("decoding tok.png: %v", err)
	}

	// Sample pixels densely — if all identical, font rendering failed.
	// Text is left-aligned at x=80, so sample the text region thoroughly.
	bounds := img.Bounds()
	firstR, firstG, firstB, _ := img.At(0, 0).RGBA()
	allSame := true
	for y := 0; y < bounds.Dy(); y += 10 {
		for x := 0; x < bounds.Dx(); x += 10 {
			r, g, b, _ := img.At(x, y).RGBA()
			if r != firstR || g != firstG || b != firstB {
				allSame = false
				break
			}
		}
		if !allSame {
			break
		}
	}

	if allSame {
		t.Error("rendered PNG appears to be a solid color — font rendering likely failed")
	}
}

func TestRenderMultipleTools(t *testing.T) {
	schemaDir := t.TempDir()
	outDir := t.TempDir()

	writeTool(t, schemaDir, "tok")
	writeTool(t, schemaDir, "jwt")

	tools, err := schema.LoadDir(schemaDir)
	if err != nil {
		t.Fatalf("LoadDir: %v", err)
	}

	if err := RenderAll(tools, outDir); err != nil {
		t.Fatalf("RenderAll: %v", err)
	}

	for _, name := range []string{"tok.png", "jwt.png"} {
		if _, err := os.Stat(filepath.Join(outDir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}
