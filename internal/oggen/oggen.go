// Package oggen renders Open Graph images for vrksh tool pages.
//
// Design spec:
//   - Background: #0A0A0A
//   - Dot grid: 1px circles on 24px grid, #1A1A1A (low opacity)
//   - Top-left: "vrk" in JetBrains Mono, muted #666666, small
//   - Center-left: "vrk <toolname>" in JetBrains Mono, large, white #FFFFFF
//   - Below tool name: hero_command (og_image.code), JetBrains Mono, medium, accent #63EB96
//   - Bottom-right: "vrk.sh" in JetBrains Mono, small, muted #444444
//   - Bottom-left: vrksh tree logo, small, low opacity
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
	bgCl     = color.RGBA{0x0A, 0x0A, 0x0A, 0xFF} // #0A0A0A
	dotCl    = color.RGBA{0x1A, 0x1A, 0x1A, 0xFF} // dot grid - subtle
	whiteCl  = color.RGBA{0xFF, 0xFF, 0xFF, 0xFF} // #FFFFFF - tool name
	accentCl = color.RGBA{0x63, 0xEB, 0x96, 0xFF} // #63EB96 - hero command
	mutedCl  = color.RGBA{0x66, 0x66, 0x66, 0xFF} // #666666 - top-left "vrk"
	footerCl = color.RGBA{0x44, 0x44, 0x44, 0xFF} // #444444 - bottom-right "vrk.sh"
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

// drawDotGrid draws a subtle dot grid across the entire canvas.
// Matches the site's .dot-grid: 1px dots on a 24px grid.
func drawDotGrid(dc *gg.Context) {
	dc.SetColor(dotCl)
	for y := 12.0; y < imgH; y += 24 {
		for x := 12.0; x < imgW; x += 24 {
			dc.DrawCircle(x, y, 0.8)
			dc.Fill()
		}
	}
}

// loadLogo decodes the embedded logo PNG.
func loadLogo() (image.Image, error) {
	return png.Decode(bytes.NewReader(logoPNG))
}

// renderCommand draws the per-tool OG image.
//
// Layout:
//
//	top-left:     "vrk" small muted
//	center-left:  "vrk <toolname>" large white
//	below:        hero_command medium accent green
//	bottom-left:  tree logo, small, low opacity
//	bottom-right: "vrk.sh" small muted
func renderCommand(dc *gg.Context, toolName, command string) error {
	// Background
	dc.SetColor(bgCl)
	dc.Clear()

	// Dot grid
	drawDotGrid(dc)

	// Top-left: "vrk" small muted
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 20); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("vrk", pad, 60)

	// Center-left: "vrk <toolname>" large white
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 56); err != nil {
		return err
	}
	dc.SetColor(whiteCl)
	toolLine := "vrk " + toolName
	dc.DrawString(toolLine, pad, imgH/2-20)

	// Below tool name: hero_command in accent green
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 24); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	dc.DrawString(command, pad, imgH/2+30)

	// Bottom-left: tree logo, scaled down and semi-transparent
	logo, err := loadLogo()
	if err != nil {
		return fmt.Errorf("loading logo: %w", err)
	}
	logoSize := 40
	logoDc := gg.NewContext(logoSize, logoSize)
	logoDc.Scale(float64(logoSize)/float64(logo.Bounds().Dx()), float64(logoSize)/float64(logo.Bounds().Dy()))
	logoDc.DrawImage(logo, 0, 0)
	// Draw the scaled logo with reduced opacity
	dc.Push()
	dc.DrawImage(logoDc.Image(), int(pad), imgH-60-logoSize/2)
	dc.Pop()

	// Bottom-right: "vrk.sh" small muted
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 18); err != nil {
		return err
	}
	dc.SetColor(footerCl)
	w, _ := dc.MeasureString("vrk.sh")
	dc.DrawString("vrk.sh", float64(imgW)-pad-w, imgH-50)

	return nil
}

// renderPipeline draws the default/install OG image.
func renderPipeline(dc *gg.Context, command, subtitle string) error {
	// Background
	dc.SetColor(bgCl)
	dc.Clear()

	// Dot grid
	drawDotGrid(dc)

	// Top-left: "vrk" small muted
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 20); err != nil {
		return err
	}
	dc.SetColor(mutedCl)
	dc.DrawString("vrk", pad, 60)

	// Center-left: title large white
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 48); err != nil {
		return err
	}
	dc.SetColor(whiteCl)
	dc.DrawString("Unix tools for AI pipelines", pad, imgH/2-20)

	// Below: subtitle / command in accent green
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 22); err != nil {
		return err
	}
	dc.SetColor(accentCl)
	dc.DrawString(command, pad, imgH/2+30)

	// Bottom-left: tree logo
	logo, err := loadLogo()
	if err != nil {
		return fmt.Errorf("loading logo: %w", err)
	}
	logoSize := 40
	logoDc := gg.NewContext(logoSize, logoSize)
	logoDc.Scale(float64(logoSize)/float64(logo.Bounds().Dx()), float64(logoSize)/float64(logo.Bounds().Dy()))
	logoDc.DrawImage(logo, 0, 0)
	dc.DrawImage(logoDc.Image(), int(pad), imgH-60-logoSize/2)

	// Bottom-right: "vrk.sh" small muted
	if err := setFont(dc, "JetBrainsMono-Regular.ttf", 18); err != nil {
		return err
	}
	dc.SetColor(footerCl)
	w, _ := dc.MeasureString("vrk.sh")
	dc.DrawString("vrk.sh", float64(imgW)-pad-w, imgH-50)

	return nil
}

// RenderAll generates one OG PNG per tool.
func RenderAll(tools []schema.Tool, outDir string) error {
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return err
	}
	for _, t := range tools {
		dc := gg.NewContext(imgW, imgH)
		if err := renderCommand(dc, t.Name, t.OGImage.Code); err != nil {
			return fmt.Errorf("rendering %s: %w", t.Name, err)
		}
		if err := dc.SavePNG(filepath.Join(outDir, t.Name+".png")); err != nil {
			return fmt.Errorf("saving %s: %w", t.Name, err)
		}
	}
	return nil
}

// RenderDefault generates the landing page OG image.
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

// RenderInstall generates the install page OG image.
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
