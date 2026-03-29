// Package oggen renders Open Graph images for vrksh tool pages.
//
// Design system (vrk.sh site palette):
//   - Background: #0D0D0D (--color-bg)
//   - Surface: #161616 (--color-surface, terminal block)
//   - Border: #2A2A2A (--color-border)
//   - Text: #E8E8E8 (--color-text)
//   - Muted: #707070 (--color-muted)
//   - Accent: #6EE7B7 (--color-accent, commands + highlights)
//   - Fonts: Urbanist (headings), JetBrains Mono (code/brand)
//   - Layout: left-aligned, vertically centered, terminal block
package oggen

import (
	"embed"
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/fogleman/gg"
	"github.com/golang/freetype/truetype"
	"github.com/vrksh/vrksh/internal/schema"
	"golang.org/x/image/font"
)

//go:embed fonts/*.ttf
var fontsFS embed.FS

const (
	imgW = 1200
	imgH = 630
	pad  = 80.0
)

var (
	bgCl      = color.RGBA{0x0D, 0x0D, 0x0D, 0xFF} // --color-bg
	surfaceCl = color.RGBA{0x16, 0x16, 0x16, 0xFF} // --color-surface
	accentCl  = color.RGBA{0x6E, 0xE7, 0xB7, 0xFF} // --color-accent
	textCl    = color.RGBA{0xE8, 0xE8, 0xE8, 0xFF} // --color-text
	mutedCl   = color.RGBA{0x70, 0x70, 0x70, 0xFF} // --color-muted
	borderCl  = color.RGBA{0x2A, 0x2A, 0x2A, 0xFF} // --color-border
	pillBgCl  = color.RGBA{0x1C, 0x2F, 0x28, 0xFF} // rgba(110,231,183,0.15) on #0D0D0D
	pillTxCl  = color.RGBA{0x6E, 0xE7, 0xB7, 0xFF} // accent green text, matches .badge-v1
)

func setFont(dc *gg.Context, name string, size float64) error {
	data, err := fontsFS.ReadFile("fonts/" + name)
	if err != nil {
		return fmt.Errorf("reading font %s: %w", name, err)
	}
	f, err := truetype.Parse(data)
	if err != nil {
		return fmt.Errorf("parsing font %s: %w", name, err)
	}
	dc.SetFontFace(truetype.NewFace(f, &truetype.Options{
		Size: size, DPI: 72, Hinting: font.HintingFull,
	}))
	return nil
}

// drawBg fills the canvas with the background color.
func drawBg(dc *gg.Context) {
	dc.SetColor(bgCl)
	dc.Clear()
}

// drawLetterspaced draws uppercase letterspaced text (no trailing period).
func drawLetterspaced(dc *gg.Context, text string, x, y, spacing float64) {
	upper := strings.ToUpper(strings.TrimRight(text, "."))
	for _, ch := range upper {
		s := string(ch)
		dc.DrawString(s, x, y)
		cw, _ := dc.MeasureString(s)
		x += cw + spacing
	}
}

// drawTerminalBlock draws a rounded-rect terminal-style block.
func drawTerminalBlock(dc *gg.Context, x, y, bw, bh, radius float64) {
	dc.SetColor(surfaceCl)
	dc.DrawRoundedRectangle(x, y, bw, bh, radius)
	dc.Fill()

	dc.SetColor(borderCl)
	dc.SetLineWidth(1.2)
	dc.DrawRoundedRectangle(x, y, bw, bh, radius)
	dc.Stroke()
}

// drawPill draws a small rounded pill badge with uppercase letterspaced text.
func drawPill(dc *gg.Context, label string, x, y float64) error {
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 14); err != nil {
		return err
	}
	upper := strings.ToUpper(label)
	spacing := 3.0

	// Measure letterspaced width
	totalW := 0.0
	for _, ch := range upper {
		cw, _ := dc.MeasureString(string(ch))
		totalW += cw + spacing
	}
	totalW -= spacing

	pillW := totalW + 24
	pillH := 26.0
	px := x - pillW
	py := y - pillH/2

	dc.SetColor(pillBgCl)
	dc.DrawRoundedRectangle(px, py, pillW, pillH, 4)
	dc.Fill()

	// Draw letterspaced text inside pill
	dc.SetColor(pillTxCl)
	tx := px + 12
	for _, ch := range upper {
		s := string(ch)
		dc.DrawString(s, tx, y+5)
		cw, _ := dc.MeasureString(s)
		tx += cw + spacing
	}
	return nil
}

// renderCommand draws Variant 1: Command Highlight (per-tool).
// Content block is vertically centered in the frame.
func renderCommand(dc *gg.Context, toolName, tagline, command, headline string) error {
	drawBg(dc)

	// Brand - top-left, mono, underlined
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 24); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("vrk.sh", pad, 75)
	bw, _ := dc.MeasureString("vrk.sh")
	dc.SetLineWidth(1.5)
	dc.DrawLine(pad, 82, pad+bw, 82)
	dc.Stroke()

	// Tool name - accent, Urbanist Medium
	if err := setFont(dc, "Urbanist-Medium.ttf", 42); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	dc.DrawString("vrk "+toolName, pad, 160)

	// Subtitle - Urbanist Regular
	if err := setFont(dc, "Urbanist-Regular.ttf", 26); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString(headline, pad, 210)

	// Terminal block
	blockPadLeft := pad + 4
	blockX := pad - 20
	blockY := 280.0
	blockW := float64(imgW) - 2*(pad-20)
	blockH := 150.0
	drawTerminalBlock(dc, blockX, blockY, blockW, blockH, 8)

	// $ command inside block
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 30); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	dc.DrawString("$ "+command, blockPadLeft, 345)

	// Tagline - JetBrains Mono, monospace comment
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 18); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("# "+tagline, blockPadLeft, 403)

	// Pill badge - top-right corner
	if err := drawPill(dc, "tool", float64(imgW)-pad, 75); err != nil {
		return err
	}

	return nil
}

// renderPipeline draws Variant 2: Pipeline (default/landing page).
func renderPipeline(dc *gg.Context, pipeline, tagline string) error {
	drawBg(dc)

	// Brand
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 24); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("vrk.sh", pad, 75)
	bw, _ := dc.MeasureString("vrk.sh")
	dc.SetLineWidth(1.5)
	dc.DrawLine(pad, 82, pad+bw, 82)
	dc.Stroke()

	// Title - Urbanist Medium
	if err := setFont(dc, "Urbanist-Medium.ttf", 42); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("Unix tools for AI pipelines", pad, 160)

	// Subtitle - Urbanist Regular
	if err := setFont(dc, "Urbanist-Regular.ttf", 28); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("One binary. No dependencies. Composable.", pad, 212)

	// Terminal block
	blockPadLeft := pad + 4
	blockX := pad - 20
	blockY := 280.0
	blockW := float64(imgW) - 2*(pad-20)
	blockH := 150.0
	drawTerminalBlock(dc, blockX, blockY, blockW, blockH, 8)

	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 26); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	dc.DrawString("$ "+pipeline, blockPadLeft, 345)

	// Comment - mono
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 20); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("# fetch -> summarize -> store", blockPadLeft, 403)

	return nil
}

// RenderAll generates one OG PNG per tool (Variant 1).
func RenderAll(tools []schema.Tool, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	for _, t := range tools {
		dc := gg.NewContext(imgW, imgH)
		if err := renderCommand(dc, t.Name, t.Tagline, t.OGImage.Code, t.OGImage.Headline); err != nil {
			return fmt.Errorf("rendering %s: %w", t.Name, err)
		}
		if err := dc.SavePNG(filepath.Join(outDir, t.Name+".png")); err != nil {
			return fmt.Errorf("saving %s: %w", t.Name, err)
		}
	}
	return nil
}

// RenderDefault generates the landing page OG image (Variant 2).
func RenderDefault(outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	dc := gg.NewContext(imgW, imgH)
	if err := renderPipeline(dc,
		"vrk grab URL | vrk prompt | vrk kv set summary",
		"Unix tools for AI pipelines",
	); err != nil {
		return err
	}
	return dc.SavePNG(filepath.Join(outDir, "default.png"))
}
