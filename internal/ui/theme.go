// Package ui implements Neromancer's pure-Go GPU-rendered native user interface
// built on github.com/egoist/mygo/ui.
package ui

import (
	"math"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/domain"
)

// Unified 6-step typography scale and 4-step corner radius scale.
const (
	FontCaption   float32 = 11.0
	FontMeta      float32 = 12.0
	FontBody      float32 = 13.5
	FontCardTitle float32 = 15.0
	FontTitle     float32 = 17.5
	FontHero      float32 = 20.0
	FontMetric    float32 = 28.0

	RadiusTag     float32 = 6.0
	RadiusControl float32 = 10.0
	RadiusCard    float32 = 16.0
	RadiusPill    float32 = 999.0
)

// Option B Bento Studio palette tokens.
var (
	ColorCanvasBg      = ui.Hex("#F2F3F6")
	ColorCardBg        = ui.Hex("#FFFFFF")
	ColorSoftPanel     = ui.Hex("#F7F8FA")
	ColorSecondaryFill = ui.Hex("#ECEDF2")
	ColorSecondaryHov  = ui.Hex("#E3E6EE")
	ColorSecondaryAct  = ui.Hex("#D9DDE7")
	ColorBorderSoft    = ui.Hex("#E3E5EB")
	ColorInkPrimary    = ui.Hex("#141519")
	ColorInkSecondary  = ui.Hex("#585C66")
	ColorInkMuted      = ui.Hex("#8B909C")
	ColorBrandAccent   = ui.Hex("#141519")
	ColorBrandHover    = ui.Hex("#262932")
	ColorBrandPressed  = ui.Hex("#0B0C0E")
	ColorVermilion     = ui.Hex("#C84B31")
	ColorVermilionHov  = ui.Hex("#B8422A")
	ColorVermilionAct  = ui.Hex("#A63922")
	ColorWarmHighlight = ui.Hex("#FEF5E6")
	ColorDangerSoft    = ui.Hex("#FDEEEB")
	ColorDangerInk     = ui.Hex("#C84B31")
	ColorSuccessSoft   = ui.Hex("#E8F6F0")
	ColorSuccessInk    = ui.Hex("#18795B")
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

// DefaultPalette returns the Option B Bento Studio surface palette.
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
		GlassTint:    ColorCardBg.Alpha(0.86),
		GlassBorder:  ColorBorderSoft,
	}
}

// OrbColorSet defines harmonious mineral colorway tokens for an OrbTheme.
type OrbColorSet struct {
	Core      ui.Color
	Mid       ui.Color
	Rim       ui.Color
	Aura      ui.Color
	CardTint  ui.Color
	BadgeBg   ui.Color
	BadgeText ui.Color
}

// ColorsForTheme returns the crisp mineral accent palette for an idea theme.
func ColorsForTheme(t domain.OrbTheme) OrbColorSet {
	switch t {
	case domain.ThemeSky:
		return OrbColorSet{
			Core:      ui.Hex("#EBF2FE"),
			Mid:       ui.Hex("#2563EB"),
			Rim:       ui.Hex("#1D4ED8"),
			Aura:      ui.Hex("#60A5FA").Alpha(0.30),
			CardTint:  ui.Hex("#F4F8FF"),
			BadgeBg:   ui.Hex("#EBF2FE"),
			BadgeText: ui.Hex("#2563EB"),
		}
	case domain.ThemeAmber:
		return OrbColorSet{
			Core:      ui.Hex("#FEF5E6"),
			Mid:       ui.Hex("#D97706"),
			Rim:       ui.Hex("#B45309"),
			Aura:      ui.Hex("#FBBF24").Alpha(0.30),
			CardTint:  ui.Hex("#FFFBF2"),
			BadgeBg:   ui.Hex("#FEF5E6"),
			BadgeText: ui.Hex("#B45309"),
		}
	case domain.ThemeMint:
		return OrbColorSet{
			Core:      ui.Hex("#E8F6F0"),
			Mid:       ui.Hex("#18795B"),
			Rim:       ui.Hex("#115E46"),
			Aura:      ui.Hex("#34D399").Alpha(0.28),
			CardTint:  ui.Hex("#F2FAF6"),
			BadgeBg:   ui.Hex("#E8F6F0"),
			BadgeText: ui.Hex("#18795B"),
		}
	case domain.ThemeLavender:
		return OrbColorSet{
			Core:      ui.Hex("#F3EEFD"),
			Mid:       ui.Hex("#6D28D9"),
			Rim:       ui.Hex("#5B21B6"),
			Aura:      ui.Hex("#A78BFA").Alpha(0.30),
			CardTint:  ui.Hex("#F8F5FE"),
			BadgeBg:   ui.Hex("#F3EEFD"),
			BadgeText: ui.Hex("#6D28D9"),
		}
	default: // ThemeCoral
		return OrbColorSet{
			Core:      ui.Hex("#FDEEEB"),
			Mid:       ui.Hex("#C84B31"),
			Rim:       ui.Hex("#A63922"),
			Aura:      ui.Hex("#F87171").Alpha(0.28),
			CardTint:  ui.Hex("#FFF6F4"),
			BadgeBg:   ui.Hex("#FDEEEB"),
			BadgeText: ui.Hex("#C84B31"),
		}
	}
}

// NeromancerTheme builds the Option B Bento Studio theme for MyGo native widgets.
func NeromancerTheme() *ui.Theme {
	pal := DefaultPalette()
	t := ui.LightTheme()
	t.SurfacePressed = ui.Hex("#E8EBF0")
	t.AccentHover = ColorBrandHover
	t.AccentPressed = ColorBrandPressed
	t.Danger = ColorDangerInk
	t.Success = ColorSuccessInk
	t.Warning = ui.Hex("#D97706")
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
	t.Selection = pal.Accent.Alpha(0.16)
	t.Focus = pal.Accent.Alpha(0.45)
	t.Radius = RadiusControl
	t.Spacing = 4
	t.FontSize = FontBody
	return t
}

// StyleSurfaceCard applies the unified primary white card elevation, radius, and hairline border.
func StyleSurfaceCard(el ui.Element, pal SurfacePalette) ui.Element {
	return el.
		Radius(RadiusCard).
		Background(pal.Card).
		Border(1, pal.Border.Alpha(0.65)).
		Shadow(0, 2, 10, 0, ui.RGBA(18, 22, 33, 0.035))
}

// SheetHeaderProps configures the unified header rendered at the top of every bottom sheet.
type SheetHeaderProps struct {
	Icon       *ui.SVG
	Title      string
	BadgeText  string
	ExtraBadge func()
	Subtitle   string
	CloseKey   string
	CloseLabel string
	OnClose    func()
}

// RenderSheetHeader renders a consistent sheet header + 1px divider across all bottom sheets.
func RenderSheetHeader(c *ui.Context, pal SurfacePalette, p SheetHeaderProps) {
	closeLbl := p.CloseLabel
	if closeLbl == "" {
		closeLbl = "关闭"
	}
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Column(c).Grow(1).MinWidth(0).Gap(3).Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Wrap().Children(func() {
					if p.Icon != nil {
						ui.Icon(c, p.Icon).Size(17, 17).TextColor(pal.Ink).Shrink(0)
					}
					ui.Text(c, p.Title).FontSize(FontTitle).Bold().TextColor(pal.Ink)
					if p.BadgeText != "" {
						ui.Text(c, p.BadgeText).
							FontSize(FontCaption).FontWeight(600).FontFeatures("tnum").
							Padding(2, 8).Radius(RadiusPill).
							Background(pal.SoftPanel).Border(1, pal.Border.Alpha(0.65)).
							TextColor(pal.InkSecondary).Shrink(0)
					}
					if p.ExtraBadge != nil {
						p.ExtraBadge()
					}
				})
				if sub := strings.TrimSpace(p.Subtitle); sub != "" {
					ui.Text(c, sub).FontSize(FontMeta).TextColor(pal.InkMuted).SingleLine()
				}
			})

			if GhostIconAction(c, p.CloseKey, IconClose, closeLbl, false).Clicked() && p.OnClose != nil {
				p.OnClose()
			}
		})

		ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.75))
	})
}

// RenderSearchBar builds the unified search field shared between Cards and Graph archive views.
func RenderSearchBar(c *ui.Context, pal SurfacePalette, keyPrefix, placeholder, label string, query *string) {
	q := strings.TrimSpace(*query)
	ui.Row(c).Key(keyPrefix + "_bar").FillWidth().Padding(8, 12).Gap(8).AlignItems(ui.Center).
		Radius(RadiusControl).
		Background(pal.Card).
		Border(1, pal.Border.Alpha(0.75)).
		Shadow(0, 2, 8, 0, ui.RGBA(18, 22, 33, 0.03)).
		Role(ui.RoleSearch).
		Children(func() {
			ui.Icon(c, IconSearch).Size(14, 14).TextColor(pal.InkMuted).Shrink(0)
			ui.TextInputBase(c, query).
				Placeholder(placeholder).
				PlaceholderColor(pal.InkMuted).
				TextColor(pal.Ink).
				FontSize(FontBody).
				Label(label).
				Search().
				Grow(1).MinWidth(0)
			if q != "" {
				if GhostIconAction(c, keyPrefix+"_clear", IconClose, "清空搜索", false).
					Size(24, 24).Clicked() {
					*query = ""
				}
			}
		})
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
