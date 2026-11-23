package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/service"
)

// GlobeState stores pure frame-persistent 3D orientation, physics, cached layout metrics,
// and interaction state for the Dashboard Inspiration Globe without retaining any ui.Element or *ui.Context handles.
type GlobeState struct {
	Initialized bool
	Rot         [9]float64
	VelYaw      float64
	VelPitch    float64
	LastTick    time.Time

	DragDummy     ui.Point
	Dragging      bool
	DragMoved     bool
	DragStartX    float32
	DragStartY    float32
	LastPtrX      float32
	LastPtrY      float32
	VelSampleX    float32
	VelSampleY    float32
	VelSampleTime time.Time
	DownTerm      string

	FocusAnimating bool
	FocusStart     time.Time
	FocusDuration  time.Duration
	FocusFrom      [9]float64
	FocusTo        [9]float64

	LastBlankTap   time.Time
	LastHapticWord string
	LastHapticAt   time.Time

	RipplePending bool
	RippleTerm    string

	// Cached static sphere geometry & base text metrics (rebuilt only when WordCloud or TextScale changes).
	CacheVaultGen  int
	CacheTextScale float32
	CachedPts      [][3]float64
	CachedMetrics  []globeTermMetric
	ProjectedBuf   []globeProjectedTerm
}

type globeTermMetric struct {
	Word     string
	BaseFont float32
	Weight   int
	PadY     float32
	BoxW     float32
	BoxH     float32
	OrbPal   OrbColorSet
}

type globeProjectedTerm struct {
	Term       service.WordCloudTerm
	BaseVec    [3]float64
	RotVec     [3]float64
	StageX     float32
	StageY     float32
	Width      float32
	Height     float32
	DepthScale float32
	Opacity    float32
	Backface   bool
	Occluded   bool
	ZOrder     int
}

const (
	globeGoldenAngle   = math.Pi * (3.0 - 2.2360679774997896) // pi * (3 - sqrt(5))
	globeAmbientYaw    = 0.0087                               // ~0.5 deg/s
	globeAmbientPitch  = 0.0032
	globeFrictionRate  = 3.4
	globeDragSens      = 0.0085
	globeFocusDuration = 360 * time.Millisecond
)

func (a *App) ensureGlobeCache(c *ui.Context, terms []service.WordCloudTerm) {
	st := &a.Globe
	n := len(terms)
	textScale := c.Preferences().TextScale
	if n == 0 {
		st.CachedPts = nil
		st.CachedMetrics = nil
		st.ProjectedBuf = nil
		st.CacheVaultGen = a.VaultGen
		st.CacheTextScale = textScale
		return
	}
	if st.CacheVaultGen == a.VaultGen &&
		st.CacheTextScale == textScale &&
		len(st.CachedMetrics) == n &&
		len(st.CachedPts) == n &&
		st.CachedMetrics[0].Word == terms[0].Word &&
		st.CachedMetrics[n-1].Word == terms[n-1].Word {
		return
	}

	st.CachedPts = fibonacciSpherePoints(n)
	if cap(st.CachedMetrics) < n {
		st.CachedMetrics = make([]globeTermMetric, n)
	} else {
		st.CachedMetrics = st.CachedMetrics[:n]
	}
	if cap(st.ProjectedBuf) < n {
		st.ProjectedBuf = make([]globeProjectedTerm, n)
	} else {
		st.ProjectedBuf = st.ProjectedBuf[:n]
	}

	for i, term := range terms {
		baseFont := float32(12.0) + float32(7.5)*term.NormWeight
		weight := 400
		if term.Tier == 0 {
			weight = 700
		} else if term.Tier == 1 {
			weight = 600
		}

		tw, th := c.MeasureText(0, ui.Span{Text: term.Word, Size: baseFont, Weight: weight})
		if tw <= 0 {
			tw = float32(len([]rune(term.Word)))*baseFont + 6
			th = baseFont * 1.25
		}
		padW := float32(14)
		padH := float32(8)
		if term.Tier == 2 {
			padW = 8
			padH = 4
		}
		boxW := tw + padW
		boxH := max(th, baseFont*1.15) + padH

		st.CachedMetrics[i] = globeTermMetric{
			Word:     term.Word,
			BaseFont: baseFont,
			Weight:   weight,
			PadY:     max(float32(0), (boxH-th)*0.5),
			BoxW:     boxW,
			BoxH:     boxH,
			OrbPal:   ColorsForTheme(term.Theme),
		}
	}
	st.CacheVaultGen = a.VaultGen
	st.CacheTextScale = textScale
}

func (a *App) renderDashboardWordCloudGlobe(c *ui.Context, pal SurfacePalette) {
	terms := a.DashboardDigest.WordCloud
	phone := a.isPhoneLayout(c)
	winW, _ := c.Size()
	if winW <= 0 {
		winW = 412
	}

	var stageW float32
	var stageH float32
	if phone {
		stageW = max(float32(240), winW-60)
		stageH = 264
	} else {
		stageW = max(float32(360), min(winW, 760)-84)
		stageH = 300
	}

	a.ensureGlobeCache(c, terms)
	pts := a.Globe.CachedPts
	ambientActive := a.stepGlobePhysics(c, terms, pts)

	StyleSurfaceCard(
		ui.Column(c).Key("dash_globe_card").FillWidth().Padding(14, 16).Gap(8),
		pal,
	).Children(func() {
		ui.Text(c, "灵感星云球").FontSize(FontBody).Bold().TextColor(pal.Ink).SingleLine()

		if len(terms) == 0 {
			ui.Text(c, "暂无灵感词条").
				FontSize(FontMeta).TextColor(pal.InkSecondary).Padding(8, 0)
			return
		}

			cx := stageW * 0.5
			cy := stageH * 0.5
			sphereR := min(stageW*0.37, stageH*0.40)
			hitR := min(stageW*0.44, stageH*0.47)
			hzLeft := cx - hitR
			hzTop := cy - hitR

			metrics := a.Globe.CachedMetrics
			projected := a.Globe.ProjectedBuf[:len(terms)]
			rot := a.Globe.Rot

			for i, term := range terms {
				cm := &metrics[i]
				rv := mat3Vec(rot, pts[i])
				z := rv[2]
				pScale := 3.4 / (3.4 - z)
				sx := cx + float32(rv[0]*pScale)*sphereR*1.15
				sy := cy + float32(rv[1]*pScale)*sphereR*0.92

				depth := float32((z + 1.0) * 0.5)
				rawScale := float32(0.62) + float32(0.50)*depth
				depthScale := globeScaleBucket(rawScale)

				var opacity float32
				switch {
				case depth < 0.42:
					opacity = 0.17 + 0.18*(depth/0.42)
				case depth < 0.72:
					opacity = 0.35 + 0.40*((depth-0.42)/0.30)
				default:
					opacity = 0.75 + 0.25*((depth-0.72)/0.28)
				}
				if opacity > 1 {
					opacity = 1
				} else if opacity < 0.14 {
					opacity = 0.14
				}

				w := cm.BoxW * depthScale
				h := cm.BoxH * depthScale
				backface := z < 0.05 || opacity < 0.30

				projected[i] = globeProjectedTerm{
					Term:       term,
					BaseVec:    pts[i],
					RotVec:     rv,
					StageX:     sx,
					StageY:     sy,
					Width:      w,
					Height:     h,
					DepthScale: depthScale,
					Opacity:    opacity,
					Backface:   backface,
					ZOrder:     int(math.Round((z + 1.0) * 1000)),
				}
			}

			// Zero-alloc 2D soft collision separation among foreground words + backface ghost-bleed suppression.
			for pass := 0; pass < 2; pass++ {
				for i := range projected {
					pt := &projected[i]
					if pt.Backface {
						continue
					}
					for j := range projected {
						if j == i || projected[j].Backface || projected[j].RotVec[2] < pt.RotVec[2] {
							continue
						}
						if projected[j].RotVec[2] == pt.RotVec[2] && j > i {
							continue
						}
						o := &projected[j]
						dx := pt.StageX - o.StageX
						dy := pt.StageY - o.StageY
						minW := (pt.Width + o.Width) * 0.44
						minH := (pt.Height + o.Height) * 0.46
						ox := minW - float32(math.Abs(float64(dx)))
						oy := minH - float32(math.Abs(float64(dy)))
						if ox > 0 && oy > 0 {
							dirY := float32(1.0)
							if dy < 0 || (dy == 0 && i%2 == 0) {
								dirY = -1.0
							}
							pushY := oy * 0.58 * dirY
							pt.StageY = max(cy-hitR*0.86, min(cy+hitR*0.86, pt.StageY+pushY))
							dirX := float32(1.0)
							if dx < 0 {
								dirX = -1.0
							}
							pt.StageX = max(cx-hitR*0.88, min(cx+hitR*0.88, pt.StageX+ox*0.16*dirX))
						}
					}
				}
			}

			// Mark foreground words whose center is still occluded by a closer foreground word,
			// and dim backface words that sit directly behind a foreground word so they never smudge text.
			for i := range projected {
				pt := &projected[i]
				if pt.Backface {
					for j := range projected {
						if projected[j].Backface {
							continue
						}
						o := &projected[j]
						if float32(math.Abs(float64(pt.StageX-o.StageX))) < (pt.Width+o.Width)*0.45 &&
							float32(math.Abs(float64(pt.StageY-o.StageY))) < (pt.Height+o.Height)*0.48 {
							pt.Opacity *= 0.24
							break
						}
					}
					continue
				}
				for j := range projected {
					if j == i || projected[j].Backface || projected[j].RotVec[2] <= pt.RotVec[2] {
						continue
					}
					o := &projected[j]
					if pt.StageX >= o.StageX-o.Width*0.5 && pt.StageX <= o.StageX+o.Width*0.5 &&
						pt.StageY >= o.StageY-o.Height*0.5 && pt.StageY <= o.StageY+o.Height*0.5 {
						pt.Occluded = true
						break
					}
				}
			}

			// Stage wrapper: clipped to isolate repaint damage from outer card shadows, with zero background rings/circles.
			ui.Box(c).Key("dash_globe_stage").FillWidth().Height(stageH).Clip().
				Draw(func(p *ui.Painter, r ui.Rect) {
					if ambientActive && p.Accelerated() {
						p.AnimationFrameRate(30)
					}
				}).
				Children(func() {
					// Single stationary circular hit-zone owns all pointer/touch dragging so words crossing
					// z = 0 never drop active touch capture, while left/right margins remain scrollable.
					hz := ui.Box(c).Key("dash_globe_hitzone").
						Absolute().Left(hzLeft).Top(hzTop).Size(hitR*2, hitR*2).Radius(hitR).
						Draggable(&a.Globe.DragDummy, ui.DraggableOptions{
							Axis:   ui.Horizontal | ui.Vertical,
							Bounds: &ui.Rect{},
							Rigid:  true,
						}).
						HandleInput(func(ev ui.InputEvent) bool {
							return a.handleGlobePointer(ev, ev.X+hzLeft, ev.Y+hzTop, cx, cy, hitR, terms, pts, projected)
						})

					// Emit all words in fixed index order with constant layout geometry and font size:
					// Scale + Translate + ZIndex + TextColor achieve 100% layout-cache & shaped-text reuse.
					hz.Children(func() {
						for i := range projected {
							pt := &projected[i]
							cm := &metrics[i]
							fg := resolveGlobeWordColor(pal, cm.OrbPal, pt)

							txt := ui.Text(c, pt.Term.Word).Key("globe_w_" + pt.Term.Word).
								Absolute().
								Left(-cm.BoxW * 0.5).Top(-cm.BoxH * 0.5).
								Width(cm.BoxW).Height(cm.BoxH).
								Padding(cm.PadY, 0).
								FontSize(cm.BaseFont).
								FontWeight(cm.Weight).
								TextAlign(ui.Center).
								SingleLine().
								Scale(pt.DepthScale, pt.DepthScale).
								Translate(pt.StageX-hzLeft, pt.StageY-hzTop).
								ZIndex(pt.ZOrder).
								TextColor(fg)
							if pt.Backface || pt.Occluded {
								txt.IgnoreInput(true).PassThrough()
							} else {
								txt.Cursor(ui.CursorPointer)
							}
						}
					})
				})
		})
}

// globeScaleBucket snaps depth perspective scale onto mygo/ui's octave raster-scale buckets (2^(n/8)),
// ensuring 100% glyph-atlas reuse across frames, pixel-aligned gs.screen glyph blits, and zero settle-frame re-rasterizations.
func globeScaleBucket(k float32) float32 {
	n := math.Round(math.Log2(float64(k)) * 8.0)
	return float32(math.Exp2(n / 8.0))
}

func resolveGlobeWordColor(pal SurfacePalette, orbPal OrbColorSet, pt *globeProjectedTerm) ui.Color {
	var fg ui.Color
	switch pt.Term.Tier {
	case 0:
		fg = orbPal.Rim
	case 1:
		fg = orbPal.BadgeText.Mix(pal.InkSecondary, 0.28)
	default:
		fg = pal.InkSecondary
	}
	if z := pt.RotVec[2]; z < -0.08 {
		mist := float32((-0.08 - z) / 0.92)
		if mist > 1 {
			mist = 1
		}
		fg = fg.Mix(pal.InkMuted, mist*0.48)
	}
	return fg.Alpha(pt.Opacity)
}

func (a *App) stepGlobePhysics(c *ui.Context, terms []service.WordCloudTerm, pts [][3]float64) (ambientActive bool) {
	st := &a.Globe
	now := c.Now()
	var dt float64
	if !st.LastTick.IsZero() {
		dt = now.Sub(st.LastTick).Seconds()
		if dt < 0 {
			dt = 0
		} else if dt > 0.05 {
			dt = 0.05
		}
	}
	st.LastTick = now

	if !st.Initialized {
		st.Rot = mat3Identity()
		st.Initialized = true
	}

	if st.RipplePending {
		st.RipplePending = false
		if st.RippleTerm != "" {
			for i, t := range terms {
				if t.Word == st.RippleTerm || (t.FilterTag != "" && t.FilterTag == st.RippleTerm) {
					st.FocusFrom = st.Rot
					st.FocusTo = mat3FocusTarget(pts[i], st.Rot)
					st.FocusStart = now
					st.FocusDuration = 420 * time.Millisecond
					st.FocusAnimating = true
					st.VelYaw = 0
					st.VelPitch = 0
					break
				}
			}
		}
	}

	if st.FocusAnimating {
		dur := st.FocusDuration
		if dur <= 0 {
			dur = globeFocusDuration
		}
		u := float64(now.Sub(st.FocusStart)) / float64(dur)
		if u >= 1.0 {
			st.Rot = st.FocusTo
			st.FocusAnimating = false
		} else {
			if u < 0 {
				u = 0
			}
			ease := 1.0 - math.Pow(1.0-u, 3.0)
			st.Rot = mat3Slerp(st.FocusFrom, st.FocusTo, ease)
			c.AnimationFrame()
		}
		return false
	}

	if st.Dragging {
		return false
	}

	speed := math.Hypot(st.VelYaw, st.VelPitch)
	if speed > 0.025 {
		if dt > 0 {
			delta := mat3Mul(mat3RotX(st.VelPitch*dt), mat3RotY(st.VelYaw*dt))
			st.Rot = mat3Orthonormalize(mat3Mul(delta, st.Rot))
			decay := math.Exp(-globeFrictionRate * dt)
			st.VelYaw *= decay
			st.VelPitch *= decay
			if speed < 1.4 {
				a.checkGlobeCenterHaptic(now, terms, pts)
			}
		}
		c.AnimationFrame()
		return false
	}

	st.VelYaw = 0
	st.VelPitch = 0

	if len(terms) > 0 && !c.Preferences().ReduceMotion {
		if dt > 0 {
			delta := mat3Mul(mat3RotX(globeAmbientPitch*dt), mat3RotY(globeAmbientYaw*dt))
			st.Rot = mat3Orthonormalize(mat3Mul(delta, st.Rot))
		}
		return true
	}

	return false
}

func (a *App) handleGlobePointer(
	ev ui.InputEvent,
	stageX, stageY float32,
	cx, cy, hitR float32,
	terms []service.WordCloudTerm,
	pts [][3]float64,
	projected []globeProjectedTerm,
) bool {
	st := &a.Globe
	switch ev.Kind {
	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		hit := hitTestProjectedWord(projected, stageX, stageY)
		if hit == nil && math.Hypot(float64(stageX-cx), float64(stageY-cy)) > float64(hitR) {
			return false
		}
		st.Dragging = true
		st.DragMoved = false
		st.DragStartX = stageX
		st.DragStartY = stageY
		st.LastPtrX = stageX
		st.LastPtrY = stageY
		st.VelSampleX = stageX
		st.VelSampleY = stageY
		st.VelSampleTime = ev.Time
		if hit != nil {
			st.DownTerm = hit.Term.Word
		} else {
			st.DownTerm = ""
		}
		st.VelYaw = 0
		st.VelPitch = 0
		st.FocusAnimating = false
		return true

	case ui.InputPointerMove:
		if !st.Dragging {
			return false
		}
		dx := stageX - st.LastPtrX
		dy := stageY - st.LastPtrY
		st.LastPtrX = stageX
		st.LastPtrY = stageY

		if math.Hypot(float64(stageX-st.DragStartX), float64(stageY-st.DragStartY)) > 5.0 {
			st.DragMoved = true
		}

		dYaw := float64(dx) * globeDragSens
		dPitch := float64(dy) * globeDragSens
		if dYaw != 0 || dPitch != 0 {
			delta := mat3Mul(mat3RotX(dPitch), mat3RotY(dYaw))
			st.Rot = mat3Orthonormalize(mat3Mul(delta, st.Rot))
		}

		if !ev.Time.IsZero() {
			if st.VelSampleTime.IsZero() {
				st.VelSampleX = stageX
				st.VelSampleY = stageY
				st.VelSampleTime = ev.Time
			} else if vdt := ev.Time.Sub(st.VelSampleTime).Seconds(); vdt >= 0.004 {
				if vdt < 0.25 {
					vYaw := clampFloat64(float64(stageX-st.VelSampleX)*globeDragSens/vdt, -12.0, 12.0)
					vPitch := clampFloat64(float64(stageY-st.VelSampleY)*globeDragSens/vdt, -12.0, 12.0)
					st.VelYaw = 0.50*st.VelYaw + 0.50*vYaw
					st.VelPitch = 0.50*st.VelPitch + 0.50*vPitch
				} else {
					st.VelYaw = 0
					st.VelPitch = 0
				}
				st.VelSampleX = stageX
				st.VelSampleY = stageY
				st.VelSampleTime = ev.Time
			}
		}
		return true

	case ui.InputPointerUp:
		if !st.Dragging {
			return false
		}
		st.Dragging = false
		if !ev.Time.IsZero() && !st.VelSampleTime.IsZero() && ev.Time.Sub(st.VelSampleTime) > 100*time.Millisecond {
			st.VelYaw = 0
			st.VelPitch = 0
		}
		if ev.Canceled {
			st.DownTerm = ""
			return true
		}
		if st.DragMoved {
			st.DownTerm = ""
			return true
		}

		targetWord := st.DownTerm
		st.DownTerm = ""
		if targetWord == "" {
			if hit := hitTestProjectedWord(projected, stageX, stageY); hit != nil {
				targetWord = hit.Term.Word
			}
		}

		if targetWord != "" {
			for _, t := range terms {
				if t.Word == targetWord {
					a.activateGlobeWord(t)
					return true
				}
			}
		}

		// Blank globe tap: double-tap resets orientation to #1 highest-weight term facing front.
		isDouble := ev.Clicks >= 2 || (!st.LastBlankTap.IsZero() && ev.Time.Sub(st.LastBlankTap) <= 360*time.Millisecond)
		st.LastBlankTap = ev.Time
		if isDouble {
			mygo.Haptics.Light()
			a.resetGlobeOrientation(ev.Time)
		}
		return true
	}
	return false
}

func hitTestProjectedWord(projected []globeProjectedTerm, sx, sy float32) *globeProjectedTerm {
	var best *globeProjectedTerm
	for i := range projected {
		pt := &projected[i]
		if pt.Backface {
			continue
		}
		left := pt.StageX - pt.Width*0.5
		top := pt.StageY - pt.Height*0.5
		if sx >= left && sx <= left+pt.Width && sy >= top && sy <= top+pt.Height {
			if best == nil || pt.RotVec[2] > best.RotVec[2] {
				best = pt
			}
		}
	}
	return best
}

func (a *App) activateGlobeWord(term service.WordCloudTerm) {
	mygo.Haptics.Selection()
	a.enterArchiveWithTerm(term)
}

func (a *App) resetGlobeOrientation(now time.Time) {
	st := &a.Globe
	if !st.Initialized {
		st.Rot = mat3Identity()
		st.Initialized = true
	}
	if now.IsZero() {
		now = time.Now()
	}
	st.VelYaw = 0
	st.VelPitch = 0
	st.FocusFrom = st.Rot
	st.FocusTo = mat3Identity()
	st.FocusStart = now
	st.FocusDuration = globeFocusDuration
	st.FocusAnimating = true
}

func (a *App) checkGlobeCenterHaptic(now time.Time, terms []service.WordCloudTerm, pts [][3]float64) {
	if !now.IsZero() && !a.Globe.LastHapticAt.IsZero() && now.Sub(a.Globe.LastHapticAt) < 220*time.Millisecond {
		return
	}
	for i, t := range terms {
		if t.Tier != 0 || i >= len(pts) {
			continue
		}
		rv := mat3Vec(a.Globe.Rot, pts[i])
		if rv[2] > 0.94 {
			if a.Globe.LastHapticWord != t.Word {
				a.Globe.LastHapticWord = t.Word
				a.Globe.LastHapticAt = now
				mygo.Haptics.Light()
			}
			return
		}
	}
}

// =============================================================================
// Pure-Go 3D Sphere & Quaternion Math
// =============================================================================

func fibonacciSpherePoints(n int) [][3]float64 {
	if n <= 0 {
		return nil
	}
	pts := make([][3]float64, n)
	pts[0] = [3]float64{0, 0, 1}
	for i := 1; i < n; i++ {
		z := 1.0 - 2.0*float64(i)/float64(n)
		theta := float64(i) * globeGoldenAngle
		r := math.Sqrt(max(0.0, 1.0-z*z))
		pts[i] = [3]float64{r * math.Cos(theta), r * math.Sin(theta), z}
	}
	return pts
}

func mat3Identity() [9]float64 {
	return [9]float64{
		1, 0, 0,
		0, 1, 0,
		0, 0, 1,
	}
}

func mat3ApproxIdentity(m [9]float64) bool {
	return m[0] > 0.995 && m[4] > 0.995 && m[8] > 0.995
}

func mat3Mul(a, b [9]float64) [9]float64 {
	var out [9]float64
	for r := 0; r < 3; r++ {
		for c := 0; c < 3; c++ {
			out[r*3+c] = a[r*3+0]*b[0*3+c] + a[r*3+1]*b[1*3+c] + a[r*3+2]*b[2*3+c]
		}
	}
	return out
}

func mat3Vec(m [9]float64, v [3]float64) [3]float64 {
	return [3]float64{
		m[0]*v[0] + m[1]*v[1] + m[2]*v[2],
		m[3]*v[0] + m[4]*v[1] + m[5]*v[2],
		m[6]*v[0] + m[7]*v[1] + m[8]*v[2],
	}
}

func mat3RotX(rad float64) [9]float64 {
	c, s := math.Cos(rad), math.Sin(rad)
	return [9]float64{
		1, 0, 0,
		0, c, s,
		0, -s, c,
	}
}

func mat3RotY(rad float64) [9]float64 {
	c, s := math.Cos(rad), math.Sin(rad)
	return [9]float64{
		c, 0, s,
		0, 1, 0,
		-s, 0, c,
	}
}

func mat3Orthonormalize(m [9]float64) [9]float64 {
	c0 := [3]float64{m[0], m[3], m[6]}
	c1 := [3]float64{m[1], m[4], m[7]}

	c0 = vec3Norm(c0)
	dot01 := vec3Dot(c0, c1)
	c1 = vec3Sub(c1, vec3Scale(c0, dot01))
	c1 = vec3Norm(c1)
	c2 := vec3Cross(c0, c1)

	return [9]float64{
		c0[0], c1[0], c2[0],
		c0[1], c1[1], c2[1],
		c0[2], c1[2], c2[2],
	}
}

func mat3FocusTarget(baseVec [3]float64, currentRot [9]float64) [9]float64 {
	v := vec3Norm(mat3Vec(currentRot, baseVec))
	vz := v[2]
	if vz > 0.999999 {
		return mat3Orthonormalize(currentRot)
	}
	if vz < -0.999999 {
		flipY := [9]float64{
			-1, 0, 0,
			0, 1, 0,
			0, 0, -1,
		}
		return mat3Orthonormalize(mat3Mul(flipY, currentRot))
	}

	// Shortest-arc Rodrigues matrix rotating v onto (0, 0, +1):
	// axis = v x (0,0,1) = (v[1], -v[0], 0).
	ax, ay := v[1], -v[0]
	k := [9]float64{
		0, 0, ay,
		0, 0, -ax,
		-ay, ax, 0,
	}
	k2 := mat3Mul(k, k)
	invOnePlusD := 1.0 / (1.0 + vz)

	var delta [9]float64
	id := mat3Identity()
	for i := 0; i < 9; i++ {
		delta[i] = id[i] + k[i] + k2[i]*invOnePlusD
	}
	return mat3Orthonormalize(mat3Mul(delta, currentRot))
}

type quat struct {
	w, x, y, z float64
}

func mat3ToQuat(m [9]float64) quat {
	tr := m[0] + m[4] + m[8]
	var q quat
	switch {
	case tr > 0:
		s := math.Sqrt(tr+1.0) * 2.0
		q = quat{0.25 * s, (m[7] - m[5]) / s, (m[2] - m[6]) / s, (m[3] - m[1]) / s}
	case m[0] > m[4] && m[0] > m[8]:
		s := math.Sqrt(1.0+m[0]-m[4]-m[8]) * 2.0
		q = quat{(m[7] - m[5]) / s, 0.25 * s, (m[1] + m[3]) / s, (m[2] + m[6]) / s}
	case m[4] > m[8]:
		s := math.Sqrt(1.0+m[4]-m[0]-m[8]) * 2.0
		q = quat{(m[2] - m[6]) / s, (m[1] + m[3]) / s, 0.25 * s, (m[5] + m[7]) / s}
	default:
		s := math.Sqrt(1.0+m[8]-m[0]-m[4]) * 2.0
		q = quat{(m[3] - m[1]) / s, (m[2] + m[6]) / s, (m[5] + m[7]) / s, 0.25 * s}
	}
	n := math.Sqrt(q.w*q.w + q.x*q.x + q.y*q.y + q.z*q.z)
	if n > 0 {
		q.w /= n
		q.x /= n
		q.y /= n
		q.z /= n
	} else {
		q = quat{w: 1}
	}
	return q
}

func quatToMat3(q quat) [9]float64 {
	xx, yy, zz := q.x*q.x, q.y*q.y, q.z*q.z
	xy, xz, yz := q.x*q.y, q.x*q.z, q.y*q.z
	wx, wy, wz := q.w*q.x, q.w*q.y, q.w*q.z
	return [9]float64{
		1 - 2*(yy+zz), 2 * (xy - wz), 2 * (xz + wy),
		2 * (xy + wz), 1 - 2*(xx+zz), 2 * (yz - wx),
		2 * (xz - wy), 2 * (yz + wx), 1 - 2*(xx+yy),
	}
}

func quatSlerp(a, b quat, t float64) quat {
	dot := a.w*b.w + a.x*b.x + a.y*b.y + a.z*b.z
	if dot < 0 {
		dot = -dot
		b = quat{-b.w, -b.x, -b.y, -b.z}
	}
	if dot > 0.9995 {
		res := quat{
			w: a.w + t*(b.w-a.w),
			x: a.x + t*(b.x-a.x),
			y: a.y + t*(b.y-a.y),
			z: a.z + t*(b.z-a.z),
		}
		n := math.Sqrt(res.w*res.w + res.x*res.x + res.y*res.y + res.z*res.z)
		if n > 0 {
			res.w /= n
			res.x /= n
			res.y /= n
			res.z /= n
		}
		return res
	}
	if dot > 1 {
		dot = 1
	}
	theta0 := math.Acos(dot)
	theta := theta0 * t
	sinTheta := math.Sin(theta)
	sinTheta0 := math.Sin(theta0)
	s0 := math.Cos(theta) - dot*sinTheta/sinTheta0
	s1 := sinTheta / sinTheta0
	return quat{
		w: s0*a.w + s1*b.w,
		x: s0*a.x + s1*b.x,
		y: s0*a.y + s1*b.y,
		z: s0*a.z + s1*b.z,
	}
}

func mat3Slerp(a, b [9]float64, t float64) [9]float64 {
	return mat3Orthonormalize(quatToMat3(quatSlerp(mat3ToQuat(a), mat3ToQuat(b), t)))
}

func vec3Dot(a, b [3]float64) float64 {
	return a[0]*b[0] + a[1]*b[1] + a[2]*b[2]
}

func vec3Sub(a, b [3]float64) [3]float64 {
	return [3]float64{a[0] - b[0], a[1] - b[1], a[2] - b[2]}
}

func vec3Scale(a [3]float64, s float64) [3]float64 {
	return [3]float64{a[0] * s, a[1] * s, a[2] * s}
}

func vec3Cross(a, b [3]float64) [3]float64 {
	return [3]float64{
		a[1]*b[2] - a[2]*b[1],
		a[2]*b[0] - a[0]*b[2],
		a[0]*b[1] - a[1]*b[0],
	}
}

func vec3Norm(v [3]float64) [3]float64 {
	n := math.Sqrt(v[0]*v[0] + v[1]*v[1] + v[2]*v[2])
	if n <= 1e-12 {
		return [3]float64{0, 0, 1}
	}
	return [3]float64{v[0] / n, v[1] / n, v[2] / n}
}

func clampFloat64(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
