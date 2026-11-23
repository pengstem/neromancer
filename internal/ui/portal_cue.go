package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
)

const (
	portalCueCycleSec       = 10.8
	portalCueWaveDurSec     = 3.65
	portalCueWavePeriodSec  = 1.825
	portalCueArrowOutEndSec = 4.25
	portalCueTextInStartSec = 3.80
	portalCueTextInEndSec   = 4.35
	portalCueTextOutStart   = 6.85
	portalCueTextOutEnd     = 7.45
	portalCueIconInStart    = 7.00
	portalCueIconInEnd      = 7.55
	portalCueIconOutStart   = 10.20
	portalCueArrowInStart   = 10.35
)

type portalCueFrame struct {
	CyclePos float32

	ArrowAlpha     float32
	ArrowY         float32
	ArrowScale     float32
	UpperChevAlpha float32
	UpperChevY     float32
	LowerChevAlpha float32
	LowerChevY     float32

	TextAlpha float32
	TextY     float32
	TextScale float32

	IconAlpha float32
	IconY     float32
	IconScale float32

	Animating   bool
	NextWakeSec float32
}

func evalChevronWave(phase, startOffset, span, baseAlpha, peakAlpha, maxDrift float32) (alpha, dy float32) {
	w := clamp01f((phase - startOffset) / span)
	// Smooth bell for opacity (zero slope at endpoints)
	s := float32(math.Sin(float64(w) * math.Pi))
	pulse := float32(math.Pow(float64(s), 1.6))
	alpha = baseAlpha + (peakAlpha-baseAlpha)*pulse

	// Drift monotonically downward while visible, then softly reset while dim.
	const dropPortion = float32(0.76)
	if w <= dropPortion {
		dy = maxDrift * smoothstep32(0, dropPortion, w)
	} else {
		dy = maxDrift * (1.0 - smoothstep32(dropPortion, 1.0, w))
	}
	return alpha, dy
}

// evalPortalCueFrame evaluates the deterministic 3-stage Apple-style morphing state
// (downward swipe arrow -> "思维宫殿" text -> Mind Palace icon) at elapsedSec.
func evalPortalCueFrame(elapsedSec float32) portalCueFrame {
	pos := float32(math.Mod(float64(elapsedSec), portalCueCycleSec))
	if pos < 0 {
		pos += portalCueCycleSec
	}

	f := portalCueFrame{
		CyclePos:       pos,
		ArrowScale:     1.0,
		UpperChevAlpha: 0.16,
		LowerChevAlpha: 0.28,
		TextScale:      1.0,
		IconScale:      1.0,
	}

	// Stage 0: Downward Swipe Arrow (0.0 .. 4.25 and fading back in 10.35 .. 10.80)
	if pos < portalCueWaveDurSec {
		f.ArrowAlpha = 1.0
		waveU := pos / portalCueWavePeriodSec
		phase := waveU - float32(math.Floor(float64(waveU)))
		f.UpperChevAlpha, f.UpperChevY = evalChevronWave(phase, 0.02, 0.78, 0.16, 0.48, 2.8)
		f.LowerChevAlpha, f.LowerChevY = evalChevronWave(phase, 0.16, 0.78, 0.28, 0.72, 3.2)
	} else if pos < portalCueArrowOutEndSec {
		outU := smoothstep32(portalCueWaveDurSec, portalCueArrowOutEndSec, pos)
		f.ArrowAlpha = 1.0 - outU
		f.ArrowY = lerp32(0, 3.5, outU)
		f.ArrowScale = lerp32(1.0, 0.90, outU)
	} else if pos >= portalCueArrowInStart {
		inU := smoothstep32(portalCueArrowInStart, portalCueCycleSec, pos)
		f.ArrowAlpha = inU
		f.ArrowY = lerp32(-3.5, 0, inU)
		f.ArrowScale = lerp32(0.90, 1.0, inU)
	}

	// Stage 1: "思维宫殿" Text (3.80 .. 7.45)
	if pos >= portalCueTextInStartSec && pos < portalCueTextInEndSec {
		inU := smoothstep32(portalCueTextInStartSec, portalCueTextInEndSec, pos)
		f.TextAlpha = inU
		f.TextY = lerp32(-3.5, 0, inU)
		f.TextScale = lerp32(0.95, 1.0, inU)
	} else if pos >= portalCueTextInEndSec && pos < portalCueTextOutStart {
		f.TextAlpha = 1.0
	} else if pos >= portalCueTextOutStart && pos < portalCueTextOutEnd {
		outU := smoothstep32(portalCueTextOutStart, portalCueTextOutEnd, pos)
		f.TextAlpha = 1.0 - outU
		f.TextY = lerp32(0, 3.5, outU)
		f.TextScale = lerp32(1.0, 0.95, outU)
	}

	// Stage 2: Mind Palace Icon (7.00 .. 10.80)
	if pos >= portalCueIconInStart && pos < portalCueIconInEnd {
		inU := smoothstep32(portalCueIconInStart, portalCueIconInEnd, pos)
		f.IconAlpha = inU
		f.IconY = lerp32(-3.5, 0, inU)
		f.IconScale = lerp32(0.90, 1.0, inU)
	} else if pos >= portalCueIconInEnd && pos < portalCueIconOutStart {
		f.IconAlpha = 1.0
	} else if pos >= portalCueIconOutStart {
		outU := smoothstep32(portalCueIconOutStart, portalCueCycleSec, pos)
		f.IconAlpha = 1.0 - outU
		f.IconY = lerp32(0, 3.5, outU)
		f.IconScale = lerp32(1.0, 0.90, outU)
	}

	// Determine whether continuous frame updates are needed or if we can sleep until next transition.
	switch {
	case pos >= portalCueTextInEndSec && pos < portalCueTextOutStart:
		f.Animating = false
		f.NextWakeSec = portalCueTextOutStart - pos
	case pos >= portalCueIconInEnd && pos < portalCueIconOutStart:
		f.Animating = false
		f.NextWakeSec = portalCueIconOutStart - pos
	default:
		f.Animating = true
	}

	return f
}

func drawHairlineChevron(p *ui.Painter, cx, cy, halfW, halfH, strokeW float32, col ui.Color) {
	if p == nil || col.A == 0 {
		return
	}
	var chev ui.Path
	chev.MoveTo(cx-halfW, cy-halfH)
	chev.LineTo(cx, cy+halfH)
	chev.LineTo(cx+halfW, cy-halfH)
	p.StrokePath(&chev, strokeW, col)
}

func (a *App) renderDashboardPortalCue(c *ui.Context, pal SurfacePalette, pullDist, pullThreshold float32) {
	ready := pullDist >= pullThreshold && pullThreshold > 0
	pulling := pullDist > 6
	portalGlow := a.mindPalacePortalPulse(c)

	portal := ui.ButtonBase(c).Key("dash_enter_archive").Bind(&a.DashPortalHandle)
	hovered := portal.Hovered()
	pressed := portal.Pressed()

	bg := ui.Transparent
	if ready {
		bg = ColorVermilion.Alpha(0.09)
	} else if pressed {
		bg = pal.Border.Alpha(0.46)
	} else if hovered {
		bg = pal.Border.Alpha(0.30)
	}

	portal.Height(32).Radius(RadiusControl).Background(bg).
		Cursor(ui.CursorPointer).Shrink(1).MinWidth(0).
		Label("进入思维宫殿")
	mobile.Ink(portal)
	portal.OnClick(func() {
		mygo.Haptics.Selection()
		a.enterArchiveWithTag("")
	})

	if pulling {
		pullLabel := "继续下拉"
		fg := pal.InkSecondary
		if ready {
			pullLabel = "松手打开思维宫殿"
			fg = ColorVermilion
		}
		portal.Padding(0, 10).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Justify(ui.Center).Gap(5).Children(func() {
				ui.Icon(c, IconChevronDown).Size(13.5, 13.5).TextColor(fg).Shrink(0)
				ui.Text(c, pullLabel).
					FontSize(FontMeta).FontWeight(500).TextColor(fg).SingleLine().MinWidth(0)
			})
		})
		return
	}

	portal.Width(96).Draw(func(p *ui.Painter, r ui.Rect) {
		baseInk := pal.InkSecondary
		if pressed || hovered {
			baseInk = pal.Ink
		}
		if portalGlow > 0.005 {
			baseInk = lerpColor(baseInk, ColorVermilion, portalGlow*0.85)
		}

		anchorCX := r.X + r.W*0.5
		cy := r.Y + r.H*0.5

		if p.Preferences().ReduceMotion {
			span := ui.Span{
				Text:          "思维宫殿",
				Size:          12.5,
				Weight:        500,
				LetterSpacing: 0.3,
				Color:         baseInk.Alpha(0.76),
			}
			tw, th := p.MeasureText(0, span)
			totalW := float32(10) + 5 + tw
			startX := r.X + (r.W-totalW)*0.5
			drawHairlineChevron(p, startX+5, cy+0.5, 4.2, 2.1, 1.35, baseInk.Alpha(0.62))
			p.RichText(startX+15, r.Y+(r.H-th)*0.5, 0, span)
			return
		}

		now := p.Now()
		if a.PortalCueOrigin.IsZero() || now.Before(a.PortalCueOrigin) {
			a.PortalCueOrigin = now
		}
		elapsed := float32(now.Sub(a.PortalCueOrigin).Seconds())
		f := evalPortalCueFrame(elapsed)

		glowScale := 1.0 + 0.06*portalGlow
		pullY := min(float32(3), pullDist*0.35)

		if f.ArrowAlpha > 0.01 {
			xf := ui.Scaling(f.ArrowScale*glowScale, f.ArrowScale*glowScale).
				Then(ui.Translation(0, f.ArrowY+pullY))
			p.Transform(xf, anchorCX, cy, func() {
				drawHairlineChevron(p, anchorCX, cy-2.6+f.UpperChevY, 5.0, 2.4, 1.4, baseInk.Alpha(f.ArrowAlpha*f.UpperChevAlpha))
				drawHairlineChevron(p, anchorCX, cy+1.8+f.LowerChevY, 5.0, 2.4, 1.4, baseInk.Alpha(f.ArrowAlpha*f.LowerChevAlpha))
			})
		}

		if f.TextAlpha > 0.01 {
			span := ui.Span{
				Text:          "思维宫殿",
				Size:          12.5,
				Weight:        500,
				LetterSpacing: 0.3,
				Color:         baseInk.Alpha(0.80 * f.TextAlpha),
			}
			tw, th := p.MeasureText(0, span)
			tx := r.X + (r.W-tw)*0.5
			ty := r.Y + (r.H-th)*0.5
			xf := ui.Scaling(f.TextScale*glowScale, f.TextScale*glowScale).
				Then(ui.Translation(0, f.TextY+pullY))
			p.Transform(xf, anchorCX, cy, func() {
				p.RichText(tx, ty, 0, span)
			})
		}

		if f.IconAlpha > 0.01 {
			const icSize = float32(15.5)
			icRect := ui.Rect{
				X: anchorCX - icSize*0.5,
				Y: cy - icSize*0.5,
				W: icSize,
				H: icSize,
			}
			xf := ui.Scaling(f.IconScale*glowScale, f.IconScale*glowScale).
				Then(ui.Translation(0, f.IconY+pullY))
			p.Transform(xf, anchorCX, cy, func() {
				p.Icon(IconConstellation, icRect, baseInk.Alpha(0.78*f.IconAlpha))
			})
		}

		if p.Accelerated() {
			if f.Animating {
				p.AnimationFrameRate(60)
			} else {
				waitDur := max(16*time.Millisecond, time.Duration(f.NextWakeSec*float32(time.Second)))
				p.After(waitDur)
			}
		}
	})
}
