package ui

import (
	"math"
	"sort"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

// GlobeState stores pure frame-persistent 3D orientation, physics, and interaction state
// for the Dashboard Inspiration Globe without retaining any ui.Element or *ui.Context handles.
type GlobeState struct {
	Initialized bool
	Rot         [9]float64
	VelYaw      float64
	VelPitch    float64
	LastTick    time.Time

	DragDummy   ui.Point
	Dragging    bool
	DragMoved   bool
	DragStartX  float32
	DragStartY  float32
	LastPtrX    float32
	LastPtrY    float32
	LastPtrTime time.Time
	DownTerm    string

	FocusAnimating bool
	FocusStart     time.Time
	FocusDuration  time.Duration
	FocusFrom      [9]float64
	FocusTo        [9]float64

	LastBlankTap   time.Time
	LastHapticWord string

	RipplePending bool
	RippleStart   time.Time
	RippleTerm    string
	RippleTheme   domain.OrbTheme
}

type globeProjectedTerm struct {
	Term      service.WordCloudTerm
	BaseVec   [3]float64
	RotVec    [3]float64
	StageX    float32
	StageY    float32
	Width     float32
	Height    float32
	FontSize  float32
	Opacity   float32
	BlurSigma float32
	Backface  bool
	ZOrder    int
}

const (
	globeGoldenAngle   = math.Pi * (3.0 - 2.2360679774997896) // pi * (3 - sqrt(5))
	globeAmbientYaw    = 0.0087                               // ~0.5 deg/s
	globeAmbientPitch  = 0.0032
	globeFrictionRate  = 3.4
	globeDragSens      = 0.0085
	globeFocusDuration = 360 * time.Millisecond
	globeRippleDur     = 1100 * time.Millisecond
)

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

	pts := fibonacciSpherePoints(len(terms))
	ambientActive := a.stepGlobePhysics(c, terms, pts)

	ui.Column(c).Key("dash_globe_card").FillWidth().Padding(14).Gap(10).Radius(16).
		Background(pal.Card).
		Shadow(0, 2, 10, 0, ui.RGBA(18, 22, 33, 0.04)).
		Children(func() {
		ui.Text(c, "灵感星云球").FontSize(13.5).Bold().TextColor(pal.Ink).SingleLine()

		if len(terms) == 0 {
			ui.Text(c, "记下几条想法后，这里会自动浮现你的灵感星云球。").
				FontSize(12).TextColor(pal.InkSecondary).Padding(8, 0)
			return
		}

		cx := stageW * 0.5
		cy := stageH * 0.5
		sphereR := min(stageW*0.37, stageH*0.40)
		hitR := min(stageW*0.44, stageH*0.47)
		hzLeft := cx - hitR
		hzTop := cy - hitR

		projected := make([]globeProjectedTerm, len(terms))
		for i, term := range terms {
			rv := mat3Vec(a.Globe.Rot, pts[i])
			z := rv[2]
			pScale := 3.4 / (3.4 - z)
			sx := cx + float32(rv[0]*pScale)*sphereR*1.15
			sy := cy + float32(rv[1]*pScale)*sphereR*0.92

			depth := float32((z + 1.0) * 0.5)
			depthScale := float32(0.62) + float32(0.50)*depth

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

			var blurSigma float32
			if z < -0.08 {
				blurFactor := float32((-0.08 - z) / 0.92)
				if blurFactor > 1 {
					blurFactor = 1
				}
				blurSigma = 0.45 + 1.20*blurFactor
			}

			baseFont := float32(12.0) + float32(7.5)*term.NormWeight
			fontSize := baseFont * depthScale

			weight := 400
			if term.Tier == 0 {
				weight = 700
			} else if term.Tier == 1 {
				weight = 600
			}

			tw, th := c.MeasureText(0, ui.Span{Text: term.Word, Size: fontSize, Weight: weight})
			if tw <= 0 {
				tw = float32(len([]rune(term.Word)))*fontSize + 6
				th = fontSize * 1.25
			}
			padW := float32(14)
			padH := float32(8)
			if term.Tier == 2 {
				padW = 8
				padH = 4
			}
			w := tw + padW
			h := max(th, fontSize*1.15) + padH

			backface := z < 0.05 || opacity < 0.30
			projected[i] = globeProjectedTerm{
				Term:      term,
				BaseVec:   pts[i],
				RotVec:    rv,
				StageX:    sx,
				StageY:    sy,
				Width:     w,
				Height:    h,
				FontSize:  fontSize,
				Opacity:   opacity,
				BlurSigma: blurSigma,
				Backface:  backface,
				ZOrder:    int(math.Round((z + 1.0) * 1000)),
			}
		}

		// Paint back-to-front so painter order and ZIndex stay consistent.
		sortedDraw := make([]globeProjectedTerm, len(projected))
		copy(sortedDraw, projected)
		sort.SliceStable(sortedDraw, func(i, j int) bool {
			return sortedDraw[i].RotVec[2] < sortedDraw[j].RotVec[2]
		})

		leadTheme := terms[0].Theme
		leadPal := ColorsForTheme(leadTheme)
		rippleStart := a.Globe.RippleStart
		rippleTheme := a.Globe.RippleTheme
		reduceMotion := c.Preferences().ReduceMotion

		// Stage wrapper: NO wireframe or circle border, soft light atmospheric mist backdrop.
		ui.Box(c).Key("dash_globe_stage").FillWidth().Height(stageH).
			Draw(func(p *ui.Painter, r ui.Rect) {
				p.FillRadialGradient(r, ui.RadialGradient{
					Center: ui.Point{X: 0.5, Y: 0.5},
					Radius: min(r.W, r.H) * 0.56,
					Oklab:  true,
					Stops: []ui.GradientStop{
						{Color: leadPal.Mid.Alpha(0.085), At: 0},
						{Color: leadPal.Core.Alpha(0.055), At: 0.45},
						{Color: ui.Hex("#CBD5E1").Alpha(0.045), At: 0.78},
						{Color: leadPal.Core.Alpha(0.0), At: 1},
					},
				}, 0)

				if !rippleStart.IsZero() && !reduceMotion {
					elapsed := p.Now().Sub(rippleStart)
					if elapsed >= 0 && elapsed < globeRippleDur {
						prog := float32(elapsed.Seconds() / globeRippleDur.Seconds())
						ease := 1.0 - (1.0-prog)*(1.0-prog)
						alpha := (1.0 - prog) * 0.42
						rCol := ColorsForTheme(rippleTheme).Mid
						rcx, rcy := r.X+r.W*0.5, r.Y+r.H*0.5

						var ring ui.Path
						ring.Circle(rcx, rcy, 14+ease*(min(r.W, r.H)*0.44))
						p.StrokePath(&ring, 2.2*(1.0-0.55*prog), rCol.Alpha(alpha))

						if prog < 0.45 {
							sp := prog / 0.45
							sx := r.X + r.W*0.78 - sp*(r.W*0.28)
							sy := r.Y + r.H*0.18 + sp*(r.H*0.32)
							var comet ui.Path
							comet.Circle(sx, sy, 3.6*(1.0-sp*0.6))
							p.FillPath(&comet, rCol.Alpha((1.0-sp)*0.75))
						}
						p.AnimationFrame()
					}
				}

				if ambientActive && p.Accelerated() {
					p.AnimationFrameRate(30)
				}
			}).
			Children(func() {
				// Backface terms sit directly in stage behind the hit-zone so PassThrough
				// excludes them from both hitZone pointer activation and Tester.Click.
				for _, item := range sortedDraw {
					if !item.Backface {
						continue
					}
					pt := item
					orbPal := ColorsForTheme(pt.Term.Theme)
					box := ui.Box(c).Key("globe_w_"+pt.Term.Word).
						Absolute().Left(pt.StageX-pt.Width*0.5).Top(pt.StageY-pt.Height*0.5).
						Width(pt.Width).Height(pt.Height).
						Center().ZIndex(pt.ZOrder).
						Opacity(pt.Opacity).
						PassThrough()
					if pt.BlurSigma > 0.05 {
						box.Blur(pt.BlurSigma).Cache()
					}
					box.Children(func() {
						renderGlobeWordText(c, pal, orbPal, pt)
					})
				}

				// Circular hot-zone inside stage: Draggable (Rigid=true) prevents DashboardScroll
				// from stealing touch drags inside the globe sphere while leaving left/right margins scrollable.
				hz := ui.Box(c).Key("dash_globe_hitzone").
					Absolute().Left(hzLeft).Top(hzTop).Size(hitR*2, hitR*2).Radius(hitR).
					ZIndex(1200).
					Draggable(&a.Globe.DragDummy, ui.DraggableOptions{
						Axis:   ui.Horizontal | ui.Vertical,
						Bounds: &ui.Rect{},
						Rigid:  true,
					}).
					HandleInput(func(ev ui.InputEvent) bool {
						return a.handleGlobePointer(ev, "", ev.X+hzLeft, ev.Y+hzTop, cx, cy, hitR, terms, pts, projected)
					})

				hz.Children(func() {
					for _, item := range sortedDraw {
						if item.Backface {
							continue
						}
						pt := item
						localLeft := pt.StageX - pt.Width*0.5 - hzLeft
						localTop := pt.StageY - pt.Height*0.5 - hzTop
						orbPal := ColorsForTheme(pt.Term.Theme)

						btn := ui.ButtonBase(c).Key("globe_w_"+pt.Term.Word).
							Absolute().Left(localLeft).Top(localTop).
							Width(pt.Width).Height(pt.Height).
							Radius(999).Center().ZIndex(pt.ZOrder).
							Opacity(pt.Opacity).
							Cursor(ui.CursorPointer).
							Label(pt.Term.Word).
							Draggable(&a.Globe.DragDummy, ui.DraggableOptions{
								Axis:   ui.Horizontal | ui.Vertical,
								Bounds: &ui.Rect{},
								Rigid:  true,
							})
						btn.HandleInput(func(ev ui.InputEvent) bool {
							stageX := ev.X + localLeft + hzLeft
							stageY := ev.Y + localTop + hzTop
							return a.handleGlobePointer(ev, pt.Term.Word, stageX, stageY, cx, cy, hitR, terms, pts, projected)
						}).Children(func() {
							renderGlobeWordText(c, pal, orbPal, pt)
						})
					}
				})
			})
	})
}

func renderGlobeWordText(c *ui.Context, pal SurfacePalette, orbPal OrbColorSet, pt globeProjectedTerm) {
	var fg ui.Color
	weight := 400
	switch pt.Term.Tier {
	case 0:
		fg = orbPal.Rim
		weight = 700
	case 1:
		fg = orbPal.BadgeText.Mix(pal.InkSecondary, 0.32)
		weight = 600
	default:
		fg = ui.Hex("#7E8594")
		weight = 400
	}

	txt := ui.Text(c, pt.Term.Word).
		FontSize(pt.FontSize).
		FontWeight(weight).
		TextColor(fg).
		SingleLine()

	if pt.Term.Tier == 0 && pt.RotVec[2] > 0.35 {
		zNorm := float32((pt.RotVec[2] - 0.35) / 0.65)
		txt.TextShadow(0, 0, 10*zNorm, orbPal.Mid.Alpha(0.26*zNorm))
	}
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
		st.RippleStart = now
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
			if speed < 1.8 {
				a.checkGlobeCenterHaptic(terms, pts)
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
	termWord string,
	stageX, stageY float32,
	cx, cy, hitR float32,
	terms []service.WordCloudTerm,
	pts [][3]float64,
	projected []globeProjectedTerm,
) bool {
	st := &a.Globe
	switch ev.Kind {
	case ui.InputKeyDown:
		if termWord != "" && (ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace) {
			for _, t := range terms {
				if t.Word == termWord {
					a.activateGlobeWord(t)
					return true
				}
			}
		}
		return false

	case ui.InputPointerDown:
		if ev.Button != 0 {
			return false
		}
		if termWord == "" && math.Hypot(float64(stageX-cx), float64(stageY-cy)) > float64(hitR) {
			return false
		}
		st.Dragging = true
		st.DragMoved = false
		st.DragStartX = stageX
		st.DragStartY = stageY
		st.LastPtrX = stageX
		st.LastPtrY = stageY
		st.LastPtrTime = ev.Time
		st.DownTerm = termWord
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
		if math.Hypot(float64(stageX-st.DragStartX), float64(stageY-st.DragStartY)) > 5.0 {
			st.DragMoved = true
		}

		dYaw := float64(dx) * globeDragSens
		dPitch := float64(dy) * globeDragSens
		if dYaw != 0 || dPitch != 0 {
			delta := mat3Mul(mat3RotX(dPitch), mat3RotY(dYaw))
			st.Rot = mat3Orthonormalize(mat3Mul(delta, st.Rot))
		}

		dt := ev.Time.Sub(st.LastPtrTime).Seconds()
		if dt > 0.002 && dt < 0.2 {
			instYaw := clampFloat64(dYaw/dt, -12.0, 12.0)
			instPitch := clampFloat64(dPitch/dt, -12.0, 12.0)
			st.VelYaw = 0.55*st.VelYaw + 0.45*instYaw
			st.VelPitch = 0.55*st.VelPitch + 0.45*instPitch
			if math.Hypot(st.VelYaw, st.VelPitch) < 1.8 {
				a.checkGlobeCenterHaptic(terms, pts)
			}
		}
		st.LastPtrX = stageX
		st.LastPtrY = stageY
		st.LastPtrTime = ev.Time
		return true

	case ui.InputPointerUp:
		if !st.Dragging {
			return false
		}
		st.Dragging = false
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

func (a *App) checkGlobeCenterHaptic(terms []service.WordCloudTerm, pts [][3]float64) {
	for i, t := range terms {
		if t.Tier != 0 || i >= len(pts) {
			continue
		}
		rv := mat3Vec(a.Globe.Rot, pts[i])
		if rv[2] > 0.94 {
			if a.Globe.LastHapticWord != t.Word {
				a.Globe.LastHapticWord = t.Word
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
