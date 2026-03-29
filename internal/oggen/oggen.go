// Package oggen renders Open Graph images for vrksh tool pages.
//
// Design mirrors the vrk.sh hero section:
//   - Background: #0D0D0D (--color-bg)
//   - Dot grid: 1px dots on 24px grid, #2A2A2A (--color-dot)
//   - Surface: #161616 (--color-surface, terminal block)
//   - Border: #2A2A2A (--color-border)
//   - Glow: accent green radial around terminal block (cta-glow)
//   - Text: #E8E8E8 (--color-text)
//   - Muted: #707070 (--color-muted)
//   - Accent: #6EE7B7 (--color-accent, commands + highlights)
//   - Fonts: Urbanist (headings), JetBrains Mono (code/brand)
//   - Layout: left-aligned, terminal block, pill badge, tree logo
package oggen

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
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
	surfaceCl = color.RGBA{0x16, 0x16, 0x16, 0xFF} // --color-surface
	accentCl  = color.RGBA{0x6E, 0xE7, 0xB7, 0xFF} // --color-accent
	textCl    = color.RGBA{0xE8, 0xE8, 0xE8, 0xFF} // --color-text
	mutedCl   = color.RGBA{0x70, 0x70, 0x70, 0xFF} // --color-muted
	borderCl  = color.RGBA{0x2A, 0x2A, 0x2A, 0xFF} // --color-border
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

// drawGlow draws a smooth radial green glow behind the terminal block,
// matching the site's .cta-glow: box-shadow 0 0 40px rgba(110,231,183,0.15).
// Uses per-pixel blending to avoid moire artifacts from stacked ellipses.
func drawGlow(dc *gg.Context, cx, cy, radiusX, radiusY float64) {
	// Glow extends beyond the block edges
	spreadX := radiusX * 1.5
	spreadY := radiusY * 1.8
	x0 := int(math.Max(0, cx-spreadX))
	y0 := int(math.Max(0, cy-spreadY))
	x1 := int(math.Min(imgW, cx+spreadX))
	y1 := int(math.Min(imgH, cy+spreadY))

	for py := y0; py < y1; py++ {
		for px := x0; px < x1; px++ {
			// Normalized distance from center (elliptical)
			dx := (float64(px) - cx) / spreadX
			dy := (float64(py) - cy) / spreadY
			d := math.Sqrt(dx*dx + dy*dy)
			if d >= 1.0 {
				continue
			}
			// Gaussian-ish falloff: strong in center, fades to edges
			alpha := 14.0 * math.Exp(-3.0*d*d)
			if alpha < 0.5 {
				continue
			}
			a := uint8(math.Round(alpha))
			// Blend onto existing pixel
			existing := dc.Image().At(px, py)
			er, eg, eb, ea := existing.RGBA()
			// Premultiplied alpha composite: glow color (0x6E,0xE7,0xB7) over existing
			ga := uint32(a)
			nr := (uint32(0x6E)*ga*257 + er*(255-ga)*257/255) / 257
			ng := (uint32(0xE7)*ga*257 + eg*(255-ga)*257/255) / 257
			nb := (uint32(0xB7)*ga*257 + eb*(255-ga)*257/255) / 257
			na := ga*257 + ea*(255-ga)/255
			dc.SetColor(color.RGBA{
				R: uint8(nr >> 8),
				G: uint8(ng >> 8),
				B: uint8(nb >> 8),
				A: uint8(na >> 8),
			})
			dc.SetPixel(px, py)
		}
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
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 14); err != nil {
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

	// Tree logo scaled to 28px, positioned top-left
	logoSize := 28
	logoDc := gg.NewContext(logoSize, logoSize)
	logoDc.Scale(
		float64(logoSize)/float64(logo.Bounds().Dx()),
		float64(logoSize)/float64(logo.Bounds().Dy()),
	)
	logoDc.DrawImage(logo, 0, 0)
	dc.DrawImage(logoDc.Image(), int(pad), 52)

	// "vrk.sh" next to logo
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 22); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("vrk.sh", pad+float64(logoSize)+10, 75)

	// Pill badge top-right
	if err := drawPill(dc, pillLabel, float64(imgW)-pad, 68); err != nil {
		return err
	}

	return nil
}

// renderCommand draws Variant 1: Command Highlight (per-tool).
//
// Layout (mirrors the vrk.sh hero section):
//   - Top bar: tree logo + "vrk.sh" left, group pill badge right
//   - Dot grid background
//   - "vrk <toolname>" in Urbanist Medium, accent green
//   - Headline in Urbanist Regular, white
//   - Terminal block with glow: "$ command" in accent, "# tagline" in muted
func renderCommand(dc *gg.Context, toolName, tagline, command, headline, category string) error {
	drawBg(dc)
	drawDotGrid(dc)

	// Brand bar
	if err := drawBrandBar(dc, category); err != nil {
		return err
	}

	// Tool name - accent, Urbanist Medium
	if err := setFont(dc, "Urbanist-Medium.ttf", 42); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	dc.DrawString("vrk "+toolName, pad, 160)

	// Headline - Urbanist Regular, white
	if err := setFont(dc, "Urbanist-Regular.ttf", 26); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString(headline, pad, 210)

	// Terminal block with glow
	blockPadLeft := pad + 4
	blockX := pad - 20
	blockY := 265.0
	blockW := float64(imgW) - 2*(pad-20)
	blockH := 150.0

	drawGlow(dc, blockX+blockW/2, blockY+blockH/2, blockW/2, blockH/2)
	drawTerminalBlock(dc, blockX, blockY, blockW, blockH, 8)

	// "$ command" inside block
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 28); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	dc.DrawString("$ "+command, blockPadLeft, 330)

	// "# tagline" inside block
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 18); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("# "+tagline, blockPadLeft, 388)

	// Bottom-right: "vrk.sh" URL
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 16); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	w, _ := dc.MeasureString("vrk.sh")
	dc.DrawString("vrk.sh", float64(imgW)-pad-w, imgH-40)

	return nil
}

// renderPipeline draws Variant 2: Pipeline (default/install page).
func renderPipeline(dc *gg.Context, command, subtitle string) error {
	drawBg(dc)
	drawDotGrid(dc)

	// Brand bar (no pill for landing pages)
	logo, err := loadLogo()
	if err != nil {
		return fmt.Errorf("loading logo: %w", err)
	}
	logoSize := 28
	logoDc := gg.NewContext(logoSize, logoSize)
	logoDc.Scale(
		float64(logoSize)/float64(logo.Bounds().Dx()),
		float64(logoSize)/float64(logo.Bounds().Dy()),
	)
	logoDc.DrawImage(logo, 0, 0)
	dc.DrawImage(logoDc.Image(), int(pad), 52)

	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 22); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("vrk.sh", pad+float64(logoSize)+10, 75)

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
	dc.DrawString(subtitle, pad, 212)

	// Terminal block with glow
	blockPadLeft := pad + 4
	blockX := pad - 20
	blockY := 280.0
	blockW := float64(imgW) - 2*(pad-20)
	blockH := 150.0

	drawGlow(dc, blockX+blockW/2, blockY+blockH/2, blockW/2, blockH/2)
	drawTerminalBlock(dc, blockX, blockY, blockW, blockH, 8)

	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 26); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	dc.DrawString("$ "+command, blockPadLeft, 345)

	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 20); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("# fetch -> summarize -> store", blockPadLeft, 403)

	// Bottom-right: "vrk.sh" URL
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 16); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	w, _ := dc.MeasureString("vrk.sh")
	dc.DrawString("vrk.sh", float64(imgW)-pad-w, imgH-40)

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
	); err != nil {
		return err
	}
	return dc.SavePNG(filepath.Join(outDir, "install.png"))
}
