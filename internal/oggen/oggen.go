// Package oggen renders Open Graph images for vrksh tool pages.
//
// Design mirrors the vrk.sh hero section:
//   - Background: #0D0D0D (--color-bg)
//   - Dot grid: 1px dots on 24px grid, #2A2A2A (--color-dot)
//   - Text: #E8E8E8 (--color-text), 96pt tool name
//   - Muted: #707070 (--color-muted), 32pt tagline
//   - Accent: #6EE7B7 (--color-accent), 48pt command
//   - Fonts: Urbanist (headings), IBM Plex Mono (code/brand)
//   - Layout: left-aligned, large text optimized for social preview readability
package oggen

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
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

//go:embed logo.png
var logoPNG []byte

const (
	imgW = 1200
	imgH = 630
	pad  = 80.0
)

var (
	bgCl      = color.RGBA{0x0D, 0x0D, 0x0D, 0xFF} // --color-bg
	accentCl  = color.RGBA{0x6E, 0xE7, 0xB7, 0xFF} // --color-accent
	textCl    = color.RGBA{0xE8, 0xE8, 0xE8, 0xFF} // --color-text
	mutedCl   = color.RGBA{0x70, 0x70, 0x70, 0xFF} // --color-muted
	dotCl     = color.RGBA{0x2A, 0x2A, 0x2A, 0xFF} // --color-dot
	pillBgCl  = color.RGBA{0x1C, 0x2F, 0x28, 0xFF} // rgba(110,231,183,0.15) on #0D0D0D
	pillTxCl  = color.RGBA{0x6E, 0xE7, 0xB7, 0xFF} // accent green text
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

// drawDotGrid draws a subtle dot grid across the canvas, matching the
// site's .dot-grid CSS: radial-gradient 1px dots on a 24px grid.
func drawDotGrid(dc *gg.Context) {
	dc.SetColor(dotCl)
	for y := 12.0; y < imgH; y += 24 {
		for x := 12.0; x < imgW; x += 24 {
			dc.DrawCircle(x, y, 0.8)
			dc.Fill()
		}
	}
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

// drawPill draws a small rounded pill badge with uppercase letterspaced text.
func drawPill(dc *gg.Context, label string, x, y float64) error {
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 14); err != nil {
		return err
	}
	upper := strings.ToUpper(label)
	spacing := 3.0

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

// loadLogo decodes the embedded vrksh tree logo PNG.
func loadLogo() (image.Image, error) {
	return png.Decode(bytes.NewReader(logoPNG))
}

// drawBrandBar draws the top bar: tree logo + "vrk.sh" left, pill badge right.
func drawBrandBar(dc *gg.Context, pillLabel string) error {
	logo, err := loadLogo()
	if err != nil {
		return fmt.Errorf("loading logo: %w", err)
	}

	// Tree logo scaled to 36px, positioned top-left
	logoSize := 36
	logoDc := gg.NewContext(logoSize, logoSize)
	logoDc.Scale(
		float64(logoSize)/float64(logo.Bounds().Dx()),
		float64(logoSize)/float64(logo.Bounds().Dy()),
	)
	logoDc.DrawImage(logo, 0, 0)
	dc.DrawImage(logoDc.Image(), int(pad), 46)

	// "vrk.sh" next to logo
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 32); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("vrk.sh", pad+float64(logoSize)+12, 78)

	// Pill badge top-right
	if err := drawPill(dc, pillLabel, float64(imgW)-pad, 68); err != nil {
		return err
	}

	return nil
}

// truncateToFit truncates text to fit within maxW pixels at the current font,
// appending "..." when truncation is needed.
func truncateToFit(dc *gg.Context, text string, maxW float64) string {
	w, _ := dc.MeasureString(text)
	if w <= maxW {
		return text
	}
	for i := len(text) - 1; i > 0; i-- {
		candidate := text[:i] + "..."
		w, _ = dc.MeasureString(candidate)
		if w <= maxW {
			return candidate
		}
	}
	return "..."
}

// renderCommand draws Variant 1: Command Highlight (per-tool).
//
// Layout optimized for readability at social preview sizes (~500px wide):
//   - Top bar: tree logo + "vrk.sh" left, group pill badge right
//   - Dot grid background
//   - "vrk <toolname>" in Urbanist Medium 96pt, white
//   - "$ command" in IBM Plex Mono 48pt, accent green
//   - Tagline in Urbanist Regular 32pt, muted
//   - "vrk.sh" domain bottom-right, 28pt
func renderCommand(dc *gg.Context, toolName, tagline, command, headline, category string) error {
	drawBg(dc)
	drawDotGrid(dc)

	// Brand bar
	if err := drawBrandBar(dc, category); err != nil {
		return err
	}

	// Tool name - Urbanist Medium, 96pt, #E8E8E8
	if err := setFont(dc, "Urbanist-Medium.ttf", 96); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("vrk "+toolName, pad, 230)

	// Command - IBM Plex Mono, 48pt, accent green (#6EE7B7)
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 48); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	maxW := float64(imgW) - 2*pad
	cmd := truncateToFit(dc, "$ "+command, maxW)
	dc.DrawString(cmd, pad, 340)

	// Tagline - Urbanist Regular, 38pt, #A0A0A0
	if err := setFont(dc, "Urbanist-Regular.ttf", 38); err != nil {
		return err
	}
	dc.SetColor(color.RGBA{0xA0, 0xA0, 0xA0, 0xFF})
	dc.DrawString(tagline, pad, 425)

	// Domain - IBM Plex Mono, 36pt, #A0A0A0, bottom-right
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 36); err != nil {
		return err
	}
	dc.SetColor(color.RGBA{0xA0, 0xA0, 0xA0, 0xFF})
	w, _ := dc.MeasureString("vrk.sh")
	dc.DrawString("vrk.sh", float64(imgW)-pad-w, float64(imgH)-40)

	return nil
}

// renderPipeline draws Variant 2: Pipeline (default/install page).
// Font sizes scaled proportionally with Variant 1 for social preview readability.
func renderPipeline(dc *gg.Context, command, subtitle, comment string) error {
	drawBg(dc)
	drawDotGrid(dc)

	// Brand bar (no pill for landing pages)
	logo, err := loadLogo()
	if err != nil {
		return fmt.Errorf("loading logo: %w", err)
	}
	logoSize := 36
	logoDc := gg.NewContext(logoSize, logoSize)
	logoDc.Scale(
		float64(logoSize)/float64(logo.Bounds().Dx()),
		float64(logoSize)/float64(logo.Bounds().Dy()),
	)
	logoDc.DrawImage(logo, 0, 0)
	dc.DrawImage(logoDc.Image(), int(pad), 46)

	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 32); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("vrk.sh", pad+float64(logoSize)+12, 78)

	// Title - Urbanist Medium, 64pt
	if err := setFont(dc, "Urbanist-Medium.ttf", 64); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("Unix tools for AI pipelines", pad, 180)

	// Subtitle - Urbanist Regular, 36pt
	if err := setFont(dc, "Urbanist-Regular.ttf", 36); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString(subtitle, pad, 240)

	// Command - IBM Plex Mono, 40pt, accent green
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 40); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	maxW := float64(imgW) - 2*pad
	cmd := truncateToFit(dc, "$ "+command, maxW)
	dc.DrawString(cmd, pad, 340)

	// Comment - IBM Plex Mono, 32pt, #A0A0A0
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 32); err != nil {
		return err
	}
	dc.SetColor(color.RGBA{0xA0, 0xA0, 0xA0, 0xFF})
	dc.DrawString("# "+comment, pad, 405)

	// Domain - IBM Plex Mono, 36pt, #A0A0A0, bottom-right
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 36); err != nil {
		return err
	}
	dc.SetColor(color.RGBA{0xA0, 0xA0, 0xA0, 0xFF})
	w, _ := dc.MeasureString("vrk.sh")
	dc.DrawString("vrk.sh", float64(imgW)-pad-w, float64(imgH)-40)

	return nil
}

// RenderAll generates one OG PNG per tool (Variant 1).
func RenderAll(tools []schema.Tool, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	for _, t := range tools {
		dc := gg.NewContext(imgW, imgH)
		if err := renderCommand(dc, t.Name, t.Tagline, t.OGImage.Code, t.OGImage.Headline, t.Category); err != nil {
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
		"One binary. No dependencies. Composable.",
		"fetch -> summarize -> store",
	); err != nil {
		return err
	}
	return dc.SavePNG(filepath.Join(outDir, "default.png"))
}

// RenderInstall generates the install page OG image (Variant 2).
func RenderInstall(outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	dc := gg.NewContext(imgW, imgH)
	if err := renderPipeline(dc,
		"curl -fsSL vrk.sh/install.sh | sh",
		"Install vrksh in one command",
		"one binary, zero dependencies",
	); err != nil {
		return err
	}
	return dc.SavePNG(filepath.Join(outDir, "install.png"))
}
