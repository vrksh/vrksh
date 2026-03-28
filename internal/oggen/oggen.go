// Package oggen renders Open Graph images for vrksh tool pages.
//
// Design system (abhinav.co dark palette):
//   - Background: #1A1F35 with subtle radial glow
//   - Command: #41C7C7 (cyan), DM Mono 30px
//   - Tool name: #60A5FA (blue), Urbanist Medium 42px
//   - Subtitle: #C0C8D0 uppercase letterspaced, Urbanist Regular 18px
//   - Comment: #6B7280 (dimmer than command), DM Mono 16px
//   - Brand: #BEC8D4, DM Mono 24px, underlined
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
	bg        = color.RGBA{0x1A, 0x1F, 0x35, 0xFF} // dark navy
	bgSec     = color.RGBA{0x0F, 0x17, 0x2A, 0xFF} // terminal block bg
	cyan      = color.RGBA{0x41, 0xC7, 0xC7, 0xFF} // command text
	blue      = color.RGBA{0x60, 0xA5, 0xFA, 0xFF} // tool name
	subtitle  = color.RGBA{0xC0, 0xC8, 0xD0, 0xFF} // subtitle text
	brandCl   = color.RGBA{0xBE, 0xC8, 0xD4, 0xFF} // brand
	commentCl = color.RGBA{0x6B, 0x72, 0x80, 0xFF} // comment — dimmer than command
	borderCl  = color.RGBA{0x94, 0xA3, 0xB8, 0x50} // terminal border — more visible
	pillBg   = color.RGBA{0x41, 0xC7, 0xC7, 0xFF} // pill bg — same cyan as highlight
	pillText = color.RGBA{0x0F, 0x17, 0x2A, 0xFF} // pill text — dark on bright
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
	dc.SetColor(bg)
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
	dc.SetColor(bgSec)
	dc.DrawRoundedRectangle(x, y, bw, bh, radius)
	dc.Fill()

	dc.SetColor(borderCl)
	dc.SetLineWidth(1.2)
	dc.DrawRoundedRectangle(x, y, bw, bh, radius)
	dc.Stroke()
}

// drawPill draws a small rounded pill badge with uppercase letterspaced text.
func drawPill(dc *gg.Context, label string, x, y float64) error {
	if err := setFont(dc, "DMMono-Regular.ttf", 14); err != nil {
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

	dc.SetColor(pillBg)
	dc.DrawRoundedRectangle(px, py, pillW, pillH, 4)
	dc.Fill()

	// Draw letterspaced text inside pill
	dc.SetColor(pillText)
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

	// Brand — top-left, mono, underlined
	if err := setFont(dc, "DMMono-Regular.ttf", 24); err != nil {
		return err
	}
	dc.SetColor(brandCl)
	dc.DrawString("vrk.sh", pad, 75)
	bw, _ := dc.MeasureString("vrk.sh")
	dc.SetLineWidth(1.5)
	dc.DrawLine(pad, 82, pad+bw, 82)
	dc.Stroke()

	// Tool name — blue, Urbanist (slightly smaller to reduce gap with subtitle)
	if err := setFont(dc, "Urbanist-Medium.ttf", 42); err != nil {
		return err
	}
	dc.SetColor(blue)
	dc.DrawString("vrk "+toolName, pad, 160)

	// Subtitle — uppercase, letterspaced (bumped from 16 to 18 to reduce ratio jump)
	if err := setFont(dc, "Urbanist-Regular.ttf", 18); err != nil {
		return err
	}
	dc.SetColor(subtitle)
	drawLetterspaced(dc, headline, pad, 200, 2.5)

	// Terminal block — pushed down to fill vertical space
	blockPadLeft := pad + 4 // extra inner padding
	blockX := pad - 20
	blockY := 280.0
	blockW := float64(imgW) - 2*(pad-20)
	blockH := 150.0
	drawTerminalBlock(dc, blockX, blockY, blockW, blockH, 8)

	// $ command inside block
	if err := setFont(dc, "DMMono-Regular.ttf", 30); err != nil {
		return err
	}
	dc.SetColor(cyan)
	dc.DrawString("$ "+command, blockPadLeft, 345)

	// Comment — dimmer, reads as annotation not output
	if err := setFont(dc, "DMMono-Regular.ttf", 16); err != nil {
		return err
	}
	dc.SetColor(commentCl)
	dc.DrawString("# "+tagline, blockPadLeft, 400)

	// Pill badge — top-right corner
	if err := drawPill(dc, "tool", float64(imgW)-pad, 75); err != nil {
		return err
	}

	return nil
}

// renderPipeline draws Variant 2: Pipeline (default/landing page).
func renderPipeline(dc *gg.Context, pipeline, tagline string) error {
	drawBg(dc)

	// Brand — no redundant H1. Just the nav label with "/ home"
	if err := setFont(dc, "DMMono-Regular.ttf", 24); err != nil {
		return err
	}
	dc.SetColor(brandCl)
	dc.DrawString("vrk.sh", pad, 75)
	bw, _ := dc.MeasureString("vrk.sh")
	dc.SetLineWidth(1.5)
	dc.DrawLine(pad, 82, pad+bw, 82)
	dc.Stroke()

	// Title — different from brand: full tagline in blue
	if err := setFont(dc, "Urbanist-Medium.ttf", 42); err != nil {
		return err
	}
	dc.SetColor(blue)
	dc.DrawString("Unix tools for AI pipelines", pad, 160)

	// Subtitle — uppercase
	if err := setFont(dc, "Urbanist-Regular.ttf", 18); err != nil {
		return err
	}
	dc.SetColor(subtitle)
	drawLetterspaced(dc, "One binary. No dependencies. Composable", pad, 200, 2.5)

	// Terminal block — pushed down
	blockPadLeft := pad + 4
	blockX := pad - 20
	blockY := 280.0
	blockW := float64(imgW) - 2*(pad-20)
	blockH := 150.0
	drawTerminalBlock(dc, blockX, blockY, blockW, blockH, 8)

	if err := setFont(dc, "DMMono-Regular.ttf", 26); err != nil {
		return err
	}
	dc.SetColor(cyan)
	dc.DrawString("$ "+pipeline, blockPadLeft, 345)

	// Comment
	if err := setFont(dc, "DMMono-Regular.ttf", 16); err != nil {
		return err
	}
	dc.SetColor(commentCl)
	dc.DrawString("# fetch -> summarize -> store", blockPadLeft, 400)

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

