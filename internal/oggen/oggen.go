// Package oggen renders Open Graph images for vrksh tool pages.
//
// Design mirrors the vrk.sh hero section:
//   - Background: #0D0D0D (--color-bg)
//   - Dot grid: 1px dots on 24px grid, #2A2A2A (--color-dot)
//   - Bright text: #F0F0F0, large sizes optimized for Twitter card readability
//   - Accent: #6EE7B7 (--color-accent), commands in green
//   - Fonts: Urbanist (headings), IBM Plex Mono (code/brand)
//   - Layout: left-aligned, generous spacing, everything large and readable
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
	pad  = 72.0
)

var (
	bgCl     = color.RGBA{0x0D, 0x0D, 0x0D, 0xFF}
	accentCl = color.RGBA{0x6E, 0xE7, 0xB7, 0xFF}
	textCl   = color.RGBA{0xF0, 0xF0, 0xF0, 0xFF} // brighter than before
	softCl   = color.RGBA{0xB0, 0xB0, 0xB0, 0xFF} // brighter muted
	dotCl    = color.RGBA{0x2A, 0x2A, 0x2A, 0xFF}
	pillBgCl = color.RGBA{0x1C, 0x2F, 0x28, 0xFF}
	pillTxCl = color.RGBA{0x6E, 0xE7, 0xB7, 0xFF}
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

func drawBg(dc *gg.Context) {
	dc.SetColor(bgCl)
	dc.Clear()
}

func drawDotGrid(dc *gg.Context) {
	dc.SetColor(dotCl)
	for y := 12.0; y < imgH; y += 24 {
		for x := 12.0; x < imgW; x += 24 {
			dc.DrawCircle(x, y, 0.8)
			dc.Fill()
		}
	}
}

// drawPill draws a rounded pill badge, 18pt, right-aligned at (x, y center).
func drawPill(dc *gg.Context, label string, x, y float64) error {
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 18); err != nil {
		return err
	}
	upper := strings.ToUpper(label)
	spacing := 3.5

	totalW := 0.0
	for _, ch := range upper {
		cw, _ := dc.MeasureString(string(ch))
		totalW += cw + spacing
	}
	totalW -= spacing

	pillW := totalW + 32
	pillH := 34.0
	px := x - pillW
	py := y - pillH/2

	dc.SetColor(pillBgCl)
	dc.DrawRoundedRectangle(px, py, pillW, pillH, 6)
	dc.Fill()

	dc.SetColor(pillTxCl)
	tx := px + 16
	for _, ch := range upper {
		s := string(ch)
		dc.DrawString(s, tx, y+6)
		cw, _ := dc.MeasureString(s)
		tx += cw + spacing
	}
	return nil
}

func loadLogo() (image.Image, error) {
	return png.Decode(bytes.NewReader(logoPNG))
}

// drawBrandBar draws tree logo + "vrk.sh" top-left, optional pill top-right.
func drawBrandBar(dc *gg.Context, pillLabel string) error {
	logo, err := loadLogo()
	if err != nil {
		return fmt.Errorf("loading logo: %w", err)
	}

	logoSize := 52
	logoDc := gg.NewContext(logoSize, logoSize)
	logoDc.Scale(
		float64(logoSize)/float64(logo.Bounds().Dx()),
		float64(logoSize)/float64(logo.Bounds().Dy()),
	)
	logoDc.DrawImage(logo, 0, 0)
	dc.DrawImage(logoDc.Image(), int(pad), 34)

	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 40); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("vrk.sh", pad+float64(logoSize)+16, 78)

	if pillLabel != "" {
		if err := drawPill(dc, pillLabel, float64(imgW)-pad, 62); err != nil {
			return err
		}
	}

	return nil
}

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

// drawSubtext draws "vrk: one binary . small composable tools" centered at bottom.
func drawSubtext(dc *gg.Context) error {
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 26); err != nil {
		return err
	}
	subtext := "vrk:  one binary  \u2022  small composable tools"
	dc.SetColor(softCl)
	textW, _ := dc.MeasureString(subtext)
	x := (float64(imgW) - textW) / 2
	dc.DrawString(subtext, x, float64(imgH)-30)
	return nil
}

// renderCommand draws per-tool OG images (Variant 1).
func renderCommand(dc *gg.Context, toolName, tagline, command, headline, category string) error {
	drawBg(dc)
	drawDotGrid(dc)

	if err := drawBrandBar(dc, category); err != nil {
		return err
	}

	// Tool name - 130pt, bright white
	if err := setFont(dc, "Urbanist-Medium.ttf", 130); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("vrk "+toolName, pad, 220)

	// Command - 44pt, accent green
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 44); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	maxW := float64(imgW) - 2*pad
	cmd := truncateToFit(dc, "$ "+command, maxW)
	dc.DrawString(cmd, pad, 330)

	// Tagline - 36pt, soft gray
	if err := setFont(dc, "Urbanist-Regular.ttf", 36); err != nil {
		return err
	}
	dc.SetColor(softCl)
	dc.DrawString(tagline, pad, 415)

	if err := drawSubtext(dc); err != nil {
		return err
	}

	return nil
}

// renderPipeline draws default/install OG images (Variant 2).
func renderPipeline(dc *gg.Context, command, subtitle, comment string) error {
	drawBg(dc)
	drawDotGrid(dc)

	if err := drawBrandBar(dc, ""); err != nil {
		return err
	}

	// Title - 84pt, two lines, bright white
	if err := setFont(dc, "Urbanist-Medium.ttf", 84); err != nil {
		return err
	}
	dc.SetColor(textCl)
	dc.DrawString("The missing coreutils", pad, 205)
	dc.DrawString("for the agent era", pad, 300)

	// Subtitle - 30pt, soft gray
	if subtitle != "" {
		if err := setFont(dc, "Urbanist-Regular.ttf", 30); err != nil {
			return err
		}
		dc.SetColor(softCl)
		dc.DrawString(subtitle, pad, 335)
	}

	// Command - 34pt, accent green
	cmdY := 390.0
	if subtitle != "" {
		cmdY = 400.0
	}
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 34); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	maxW := float64(imgW) - 2*pad
	cmd := truncateToFit(dc, "$ "+command, maxW)
	dc.DrawString(cmd, pad, cmdY)

	// Comment - 26pt, soft gray
	if err := setFont(dc, "IBMPlexMono-Regular.ttf", 26); err != nil {
		return err
	}
	dc.SetColor(softCl)
	dc.DrawString("# "+comment, pad, cmdY+55)

	if err := drawSubtext(dc); err != nil {
		return err
	}

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
		"vrk grab URL | vrk prompt | vrk kv set out",
		"",
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
		"",
		"install vrksh in one command",
	); err != nil {
		return err
	}
	return dc.SavePNG(filepath.Join(outDir, "install.png"))
}
