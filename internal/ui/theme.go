// Package ui implements Neromancer's pure-Go GPU-rendered native user interface
// built on github.com/egoist/mygo/ui.
package ui

import (
	"math"

	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/domain"
)

// Warm Atelier Paper palette tokens.
var (
	ColorCanvasBg      = ui.Hex("#F6F2EA")
	ColorCardBg        = ui.Hex("#FCF9F4")
	ColorSoftPanel     = ui.Hex("#EDE7DB")
	ColorBorderSoft    = ui.Hex("#DDD5C5")
	ColorInkPrimary    = ui.Hex("#1C1A17")
	ColorInkSecondary  = ui.Hex("#5C574F")
	ColorInkMuted      = ui.Hex("#948D82")
	ColorBrandAccent   = ui.Hex("#2B4C7E")
	ColorBrandHover    = ui.Hex("#213B63")
	ColorBrandPressed  = ui.Hex("#182C4B")
	ColorVermilion     = ui.Hex("#D9483B")
	ColorWarmHighlight = ui.Hex("#F5E9D0")
	ColorDangerSoft    = ui.Hex("#FBE4E1")
	ColorDangerInk     = ui.Hex("#B83227")
	ColorSuccessSoft   = ui.Hex("#E3EFE7")
	ColorSuccessInk    = ui.Hex("#246A46")
)

// SurfacePalette defines the surface, ink, and frosted-glass color tokens.
type SurfacePalette struct {
	Canvas       ui.Color
	Card         ui.Color
	SoftPanel    ui.Color
	Border       ui.Color
	Ink          ui.Color
	InkSecondary ui.Color
	InkMuted     ui.Color
	Accent       ui.Color
	AccentText   ui.Color
	GlassTint    ui.Color
	GlassBorder  ui.Color
}

// DefaultPalette returns the Warm Atelier Paper surface palette.
func DefaultPalette() SurfacePalette {
	return SurfacePalette{
		Canvas:       ColorCanvasBg,
		Card:         ColorCardBg,
		SoftPanel:    ColorSoftPanel,
		Border:       ColorBorderSoft,
		Ink:          ColorInkPrimary,
		InkSecondary: ColorInkSecondary,
		InkMuted:     ColorInkMuted,
		Accent:       ColorBrandAccent,
		AccentText:   ui.Hex("#FFFFFF"),
		GlassTint:    ColorCardBg.Alpha(0.78),
		GlassBorder:  ui.Hex("#FFFFFF").Alpha(0.65),
	}
}

// OrbColorSet defines harmonious mineral glass colorway tokens for an OrbTheme.
type OrbColorSet struct {
	Core      ui.Color
	Mid       ui.Color
	Rim       ui.Color
	Aura      ui.Color
	CardTint  ui.Color
	BadgeBg   ui.Color
	BadgeText ui.Color
}

// ColorsForTheme returns the mineral glass color palette for a Memory Orb theme.
func ColorsForTheme(t domain.OrbTheme) OrbColorSet {
	switch t {
	case domain.ThemeSky:
		return OrbColorSet{
			Core:      ui.Hex("#E6F1FC"),
			Mid:       ui.Hex("#4A90D9"),
			Rim:       ui.Hex("#1E5CA8"),
			Aura:      ui.Hex("#7CB3EC").Alpha(0.34),
			CardTint:  ui.Hex("#F1F6FC"),
			BadgeBg:   ui.Hex("#E3EEFA"),
			BadgeText: ui.Hex("#1E4D8C"),
		}
	case domain.ThemeAmber:
		return OrbColorSet{
			Core:      ui.Hex("#FDF3D7"),
			Mid:       ui.Hex("#E5A93B"),
			Rim:       ui.Hex("#B97414"),
			Aura:      ui.Hex("#F0C66E").Alpha(0.36),
			CardTint:  ui.Hex("#FCF7EA"),
			BadgeBg:   ui.Hex("#F9ECCB"),
			BadgeText: ui.Hex("#8F570B"),
		}
	case domain.ThemeMint:
		return OrbColorSet{
			Core:      ui.Hex("#E0F5EE"),
			Mid:       ui.Hex("#3BB296"),
			Rim:       ui.Hex("#177E67"),
			Aura:      ui.Hex("#79D0BA").Alpha(0.34),
			CardTint:  ui.Hex("#EFF9F5"),
			BadgeBg:   ui.Hex("#DCF2EA"),
			BadgeText: ui.Hex("#126351"),
		}
	case domain.ThemeLavender:
		return OrbColorSet{
			Core:      ui.Hex("#EFEAFB"),
			Mid:       ui.Hex("#8B74D6"),
			Rim:       ui.Hex("#5A44AA"),
			Aura:      ui.Hex("#B4A4E8").Alpha(0.34),
			CardTint:  ui.Hex("#F5F2FC"),
			BadgeBg:   ui.Hex("#EAE3FA"),
			BadgeText: ui.Hex("#4A3691"),
		}
	default: // ThemeCoral
		return OrbColorSet{
			Core:      ui.Hex("#FFEAE6"),
			Mid:       ui.Hex("#EA6A5E"),
			Rim:       ui.Hex("#C93B2E"),
			Aura:      ui.Hex("#F28A7E").Alpha(0.34),
			CardTint:  ui.Hex("#FDF2EF"),
			BadgeBg:   ui.Hex("#FCE4DF"),
			BadgeText: ui.Hex("#A62C21"),
		}
	}
}

// NeromancerTheme builds the Warm Atelier Paper theme for MyGo native widgets.
func NeromancerTheme() *ui.Theme {
	pal := DefaultPalette()
	t := ui.LightTheme()
	t.SurfacePressed = ui.Hex("#E4DDD0")
	t.AccentHover = ColorBrandHover
	t.AccentPressed = ColorBrandPressed
	t.Danger = ColorVermilion
	t.Success = ColorSuccessInk
	t.Warning = ui.Hex("#B97414")
	t.Dark = false
	t.Background = pal.Canvas
	t.Surface = pal.Card
	t.SurfaceHover = pal.SoftPanel
	t.Border = pal.Border
	t.Text = pal.Ink
	t.TextMuted = pal.InkSecondary
	t.Accent = pal.Accent
	t.AccentText = pal.AccentText
	t.InactiveAccent = pal.Accent.Alpha(0.72)
	t.Selection = pal.Accent.Alpha(0.24)
	t.Focus = pal.Accent.Alpha(0.55)
	t.Radius = 12
	t.Spacing = 4
	t.FontSize = 14
	return t
}

// DrawMemoryOrb paints a luminous glass Memory Orb onto canvas r using the Kage shader pipeline.
func DrawMemoryOrb(p *ui.Painter, r ui.Rect, theme domain.OrbTheme, weight float64, motif string, selected bool) {
	DrawAnimatedMemoryOrb(p, r, theme, weight, 0.35, 0.25, motif, selected)
}

// DrawAnimatedMemoryOrb paints a phase-animated luminous glass Memory Orb via Kage shader.
func DrawAnimatedMemoryOrb(p *ui.Painter, r ui.Rect, theme domain.OrbTheme, weight float64, phase, energy float32, motif string, selected bool) {
	DrawShaderOrb(p, r, theme, weight, phase, energy, motif, selected)
}

func drawOrbMotif(p *ui.Painter, cx, cy, scale float32, motif string, col ui.Color) {
	switch motif {
	case "petals":
		for i := 0; i < 4; i++ {
			angle := float64(i) * math.Pi / 2.0
			px := cx + float32(math.Cos(angle))*scale*0.36
			py := cy + float32(math.Sin(angle))*scale*0.36
			var petal ui.Path
			petal.Circle(px, py, max(2, scale*0.32))
			p.StrokePath(&petal, 1.3, col)
		}
	case "orbit":
		var ring ui.Path
		ring.Circle(cx, cy, scale*0.72)
		p.StrokePath(&ring, 1.3, col)
		var dot ui.Path
		dot.Circle(cx+scale*0.52, cy-scale*0.48, max(2.2, scale*0.15))
		p.FillPath(&dot, col)
	case "wave":
		var wave ui.Path
		wave.MoveTo(cx-scale*0.8, cy)
		wave.QuadTo(cx-scale*0.4, cy-scale*0.55, cx, cy)
		wave.QuadTo(cx+scale*0.4, cy+scale*0.55, cx+scale*0.8, cy)
		p.StrokePath(&wave, 1.6, col)
	case "prism":
		var tri ui.Path
		tri.MoveTo(cx, cy-scale*0.65)
		tri.LineTo(cx+scale*0.60, cy+scale*0.45)
		tri.LineTo(cx-scale*0.60, cy+scale*0.45)
		tri.Close()
		p.StrokePath(&tri, 1.4, col)
	default: // constellation / sprout
		pts := [][2]float32{
			{cx - scale*0.45, cy + scale*0.25},
			{cx - scale*0.05, cy - scale*0.35},
			{cx + scale*0.48, cy - scale*0.10},
		}
		p.StrokePolyline(pts, 1.3, col)
		for _, pt := range pts {
			var star ui.Path
			star.Circle(pt[0], pt[1], max(1.8, scale*0.14))
			p.FillPath(&star, col)
		}
	}
}

// DrawNavSparkIcon paints a clean orb/spark icon.
func DrawNavSparkIcon(p *ui.Painter, r ui.Rect, col ui.Color) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	rad := min(r.W, r.H) * 0.34
	var circle ui.Path
	circle.Circle(cx, cy, rad)
	p.StrokePath(&circle, 1.8, col)
	var core ui.Path
	core.Circle(cx-rad*0.28, cy-rad*0.28, max(2, rad*0.28))
	p.FillPath(&core, col)
}

// DrawNavMindIcon paints a constellation/directory node icon.
func DrawNavMindIcon(p *ui.Painter, r ui.Rect, col ui.Color) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	s := min(r.W, r.H) * 0.34
	pts := [][2]float32{
		{cx - s, cy + s*0.6},
		{cx, cy - s*0.8},
		{cx + s, cy + s*0.5},
	}
	p.StrokePolyline(pts, 1.7, col)
	for _, pt := range pts {
		var dot ui.Path
		dot.Circle(pt[0], pt[1], 2.6)
		p.FillPath(&dot, col)
	}
}

// DrawNavRoamIcon paints an orbit/collision icon.
func DrawNavRoamIcon(p *ui.Painter, r ui.Rect, col ui.Color) {
	cx, cy := r.X+r.W/2, r.Y+r.H/2
	s := min(r.W, r.H) * 0.35
	var c1, c2 ui.Path
	c1.Circle(cx-s*0.28, cy, s*0.56)
	c2.Circle(cx+s*0.28, cy, s*0.56)
	p.StrokePath(&c1, 1.7, col)
	p.StrokePath(&c2, 1.7, col)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
