package ui

import (
	"math"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

const (
	orbDialHoldThreshold = 150 * time.Millisecond
	orbDialMainSize      = float32(54)
	orbDialItemSize      = float32(48)
	orbDialGap           = float32(12)
	orbDialStackHeight   = float32(268)
)

var (
	orbMonoBlack       = ui.Hex("#141519")
	orbMonoBlackHover  = ui.Hex("#262932")
	orbMonoBlackActive = ui.Hex("#0B0C0E")
	orbMonoWhite       = ui.Hex("#FFFFFF")
	orbMonoWhiteHover  = ui.Hex("#F5F6F8")
	orbMonoBorderLight = ui.RGBA(20, 21, 25, 0.12)
	orbMonoBorderDark  = ui.RGBA(255, 255, 255, 0.14)
)

// OrbDialState stores pure frame-persistent press, hold, drag-scrub, and close-latch state
// shared by the bottom-right circular button across Dashboard (3-way capture) and Mind Palace (binary view toggle).
type OrbDialState struct {
	DragDummy      ui.Point
	Pressing       bool
	WasOpen        bool
	Scrubbing      bool
	EnteredOption  bool
	HoverIdx       int
	DownX          float32
	DownY          float32
	PointerX       float32
	PointerY       float32
	DownAt         time.Time
	HasLatchedAlt  bool
	LatchedArchAlt ArchiveViewMode
}

type orbDialOption struct {
	key      string
	animKey  string
	label    string
	icon     *ui.SVG
	bindIdx  int
	onSelect func()
}

func lerpColor(a, b ui.Color, t float32) ui.Color {
	if t <= 0 {
		return a
	}
	if t >= 1 {
		return b
	}
	inv := 1.0 - t
	return ui.Color{
		R: uint8(float32(a.R)*inv + float32(b.R)*t + 0.5),
		G: uint8(float32(a.G)*inv + float32(b.G)*t + 0.5),
		B: uint8(float32(a.B)*inv + float32(b.B)*t + 0.5),
		A: uint8(float32(a.A)*inv + float32(b.A)*t + 0.5),
	}
}

func oppositeArchiveMode(m ArchiveViewMode) ArchiveViewMode {
	if m == ArchiveGraph {
		return ArchiveCards
	}
	return ArchiveGraph
}

func archiveOptionSpec(altMode ArchiveViewMode, a *App) orbDialOption {
	if altMode == ArchiveGraph {
		return orbDialOption{
			key:     "arch_orb_alt",
			animKey: "arch_float_0",
			label:   "脉络",
			icon:    IconConstellation,
			bindIdx: -1,
			onSelect: func() {
				a.ArchiveMode = ArchiveGraph
			},
		}
	}
	return orbDialOption{
		key:     "arch_orb_alt",
		animKey: "arch_float_0",
		label:   "卡片",
		icon:    IconCards,
		bindIdx: -1,
		onSelect: func() {
			a.ArchiveMode = ArchiveCards
		},
	}
}

func hitOrbDialOption(localX, localY float32, count int) int {
	if count <= 0 {
		return -1
	}
	const centerX = orbDialMainSize * 0.5
	if math.Abs(float64(localX-centerX)) > 52 {
		return -1
	}
	if count == 1 {
		if localY <= -6 && localY >= -104 {
			return 0
		}
		return -1
	}
	step := orbDialItemSize + orbDialGap
	topCenterY := -(orbDialGap + orbDialItemSize*0.5) - float32(count-1)*step
	if localY < topCenterY && localY >= topCenterY-step*0.65 {
		return 0
	}
	bestIdx := -1
	bestDist := float32(1e9)
	for i := 0; i < count; i++ {
		stepsUp := float32(count - 1 - i)
		centerY := -(orbDialGap + orbDialItemSize*0.5) - stepsUp*step
		dy := float32(math.Abs(float64(localY - centerY)))
		if dy <= step*0.52 && dy < bestDist {
			bestDist = dy
			bestIdx = i
		}
	}
	return bestIdx
}

func (a *App) dialOptionScreenRect(c *ui.Context, optionIdx int, rightPad, bottomPad float32) ui.Rect {
	if optionIdx >= 0 && optionIdx < len(a.DashDialHandles) {
		if b := a.DashDialHandles[optionIdx].Bounds(c); b.W > 0 && b.H > 0 {
			return b
		}
	}
	winW, winH := c.Size()
	if winW <= 0 || winH <= 0 {
		winW, winH = 412, 892
	}
	fabX := winW - rightPad - orbDialMainSize
	fabY := winH - bottomPad - orbDialMainSize
	stepsUp := float32(2 - optionIdx)
	step := orbDialItemSize + orbDialGap
	return ui.Rect{
		X: fabX + (orbDialMainSize-orbDialItemSize)*0.5,
		Y: fabY - orbDialGap - orbDialItemSize - stepsUp*step,
		W: orbDialItemSize,
		H: orbDialItemSize,
	}
}

func (a *App) renderFloatingOrbCircle(
	c *ui.Context,
	item orbDialOption,
	stepsUp int,
	prog float32,
	interactive bool,
	isHovered bool,
	onPick func(),
) {
	startBottom := (orbDialMainSize - orbDialItemSize) * 0.5
	step := orbDialItemSize + orbDialGap
	targetBottom := orbDialMainSize + orbDialGap + float32(stepsUp)*step
	floatBottom := startBottom + (targetBottom-startBottom)*prog
	insetX := (orbDialMainSize - orbDialItemSize) * 0.5

	alpha := clamp01f(prog * 1.35)
	emergeScale := 0.44 + 0.56*max(float32(0), min(float32(1.12), prog))

	if !interactive {
		ui.Box(c).Key(item.key + "_retreat").
			Absolute().Left(insetX).Bottom(floatBottom).
			Size(orbDialItemSize, orbDialItemSize).Radius(orbDialItemSize * 0.5).Center().
			Background(orbMonoWhite).
			Border(1, orbMonoBorderLight).
			Shadow(0, 5, 14, 0, ui.RGBA(0, 0, 0, 0.08*alpha)).
			Scale(emergeScale, emergeScale).
			Opacity(alpha).
			PassThrough().
			Children(func() {
				ui.Icon(c, item.icon).Size(20, 20).TextColor(orbMonoBlack)
			})
		return
	}

	orbBtn := ui.ButtonBase(c).Key(item.key)
	if item.bindIdx >= 0 && item.bindIdx < len(a.DashDialHandles) {
		orbBtn.Bind(&a.DashDialHandles[item.bindIdx])
	}

	hoverScaleTarget := float32(1.0)
	if isHovered {
		hoverScaleTarget = 1.15
	} else if orbBtn.Pressed() {
		hoverScaleTarget = 0.92
	} else if orbBtn.Hovered() {
		hoverScaleTarget = 1.06
	}
	hoverScale := orbBtn.Spring("hover_scale", hoverScaleTarget, ui.SpringOptions{Duration: 220 * time.Millisecond, Bounce: 0.16})
	finalScale := emergeScale * hoverScale

	bgTarget := orbMonoWhite
	fgTarget := orbMonoBlack
	borderTarget := orbMonoBorderLight
	shadowBlur := float32(16)
	shadowAlpha := float32(0.11)
	if isHovered || orbBtn.Pressed() {
		bgTarget = orbMonoBlack
		fgTarget = orbMonoWhite
		borderTarget = orbMonoBlack
		shadowBlur = 22
		shadowAlpha = 0.24
	} else if orbBtn.Hovered() {
		bgTarget = orbMonoWhiteHover
		shadowAlpha = 0.15
	}

	colorAnim := ui.Animation{Duration: 150 * time.Millisecond, Ease: ui.EaseOut, Oklab: true}
	bg := orbBtn.AnimateColor("orb_bg", bgTarget, colorAnim)
	fg := orbBtn.AnimateColor("orb_fg", fgTarget, colorAnim)
	borderCol := orbBtn.AnimateColor("orb_bd", borderTarget, colorAnim)

	var pullX, pullY float32
	if isHovered && a.OrbDial.Pressing {
		slotCenterY := -(orbDialGap + orbDialItemSize*0.5) - float32(stepsUp)*step
		dx := a.OrbDial.PointerX - orbDialMainSize*0.5
		dy := a.OrbDial.PointerY - slotCenterY
		pullX = max(float32(-4), min(float32(4), dx*0.12))
		pullY = max(float32(-4), min(float32(4), dy*0.12))
	}
	magX := orbBtn.Spring("mag_x", pullX, ui.SpringOptions{Duration: 160 * time.Millisecond})
	magY := orbBtn.Spring("mag_y", pullY, ui.SpringOptions{Duration: 160 * time.Millisecond})

	orbBtn.Absolute().Left(insetX).Bottom(floatBottom).
		Size(orbDialItemSize, orbDialItemSize).Radius(orbDialItemSize * 0.5).Center().
		Background(bg).
		Border(1, borderCol).
		Shadow(0, 6, shadowBlur, 0, ui.RGBA(0, 0, 0, shadowAlpha*alpha)).
		Translate(magX, magY).
		Scale(finalScale, finalScale).
		Opacity(alpha).
		FocusRing(false).
		Cursor(ui.CursorPointer).
		Label(item.label).
		OnClick(onPick).
		Children(func() {
			ui.Icon(c, item.icon).Size(20, 20).TextColor(fg)
		})
}

func (a *App) renderFloatingOrbDial(c *ui.Context, pal SurfacePalette, archiveProgress float32) {
	if a.DetailSheetOpen || a.EvidenceModalOpen || a.SettingsModalOpen || a.ShareModalOpen {
		return
	}
	if a.CaptureModalOpen && a.CaptureAnim.Phase == CaptureAnimOpen {
		return
	}

	phone := a.isPhoneLayout(c)
	_, _, bottomSafe, _ := c.SafeArea()
	t := max(float32(0), min(float32(1), archiveProgress))
	inArchive := t >= 0.5

	dialFade := float32(1.0)
	if a.CaptureModalOpen && a.CaptureAnim.Phase == CaptureAnimOpening && !a.CaptureAnim.PhaseStart.IsZero() {
		dt := float32(c.Now().Sub(a.CaptureAnim.PhaseStart)) / float32(captureOpenDur)
		dialFade = max(0, 1.0-smoothstep32(0.0, 0.55, dt))
	}

	if t < 0.99 && dialFade > 0.01 {
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(68 + bottomSafe).
			Opacity((1.0 - t) * dialFade).PassThrough().
			LinearGradient(ui.LinearGradient{
				From:  pal.Canvas.Alpha(0.0),
				To:    pal.Canvas,
				Angle: 180,
				Oklab: true,
			})
	}

	d := &a.OrbDial
	dialOpen := a.CaptureDialOpen
	backdropLabel := "收起速记菜单"
	if inArchive {
		dialOpen = a.ArchiveDialOpen
		backdropLabel = "收起视窗选择"
	}

	setDialOpen := func(open bool) {
		if inArchive {
			a.ArchiveDialOpen = open
			a.CaptureDialOpen = false
			if open {
				d.LatchedArchAlt = oppositeArchiveMode(a.ArchiveMode)
				d.HasLatchedAlt = true
			}
		} else {
			a.CaptureDialOpen = open
			a.ArchiveDialOpen = false
		}
		if !open {
			d.HoverIdx = -1
		}
	}

	dashRight := float32(18)
	archRight := float32(18)
	dashBottom := float32(20)
	archBottom := float32(46)
	if !phone {
		dashRight = 28
		archRight = 28
		dashBottom = 26
		archBottom = 48
	}
	rightPad := dashRight + (archRight-dashRight)*t
	bottomPad := dashBottom + (archBottom-dashBottom)*t + bottomSafe

	dashOptions := [3]orbDialOption{
		{
			key:     "dash_dial_image",
			animKey: "dash_float_2",
			label:   "图文",
			icon:    IconImage,
			bindIdx: 0,
			onSelect: func() {
				a.openCaptureModal(2, a.dialOptionScreenRect(c, 0, rightPad, bottomPad))
			},
		},
		{
			key:     "dash_dial_text",
			animKey: "dash_float_1",
			label:   "文字",
			icon:    IconQuill,
			bindIdx: 1,
			onSelect: func() {
				a.openCaptureModal(1, a.dialOptionScreenRect(c, 1, rightPad, bottomPad))
			},
		},
		{
			key:     "dash_dial_voice",
			animKey: "dash_float_0",
			label:   "语音",
			icon:    IconMic,
			bindIdx: 2,
			onSelect: func() {
				a.openCaptureModal(0, a.dialOptionScreenRect(c, 2, rightPad, bottomPad))
			},
		},
	}

	altMode := oppositeArchiveMode(a.ArchiveMode)
	if a.ArchiveDialOpen {
		d.LatchedArchAlt = altMode
		d.HasLatchedAlt = true
	} else if d.HasLatchedAlt {
		altMode = d.LatchedArchAlt
	}
	archOpt := archiveOptionSpec(altMode, a)

	var activeOptions []orbDialOption
	if inArchive {
		activeOptions = []orbDialOption{archOpt}
	} else {
		activeOptions = dashOptions[:]
	}

	// Hold-still wakeup transitions into scrubbing mode seamlessly if finger stays pressed.
	if d.Pressing && !d.Scrubbing {
		now := c.Now()
		if !d.DownAt.IsZero() && now.Sub(d.DownAt) >= orbDialHoldThreshold {
			d.Scrubbing = true
			if !dialOpen {
				mygo.Haptics.Medium()
				setDialOpen(true)
				dialOpen = true
			}
		} else {
			c.AnimationFrame()
		}
	}

	if dialOpen && !a.CaptureModalOpen {
		backdrop := ui.ButtonBase(c).Key("capture_dial_backdrop").
			Absolute().Fill().
			Background(ui.Transparent).
			FocusRing(false).
			Cursor(ui.CursorDefault).
			Label(backdropLabel)
		backdrop.OnClick(func() {
			mygo.Haptics.Selection()
			a.closeOrbDials()
		})
	}

	dialAnchor := ui.Box(c).Key("floating_orb_dial_anchor").
		Absolute().Right(rightPad).Bottom(bottomPad).
		Width(orbDialMainSize).Height(orbDialStackHeight).
		Opacity(dialFade).
		PassThrough()

	// Drive all float channels on the persistent anchor so first-time opens genuinely float up from 0 -> 1.
	var dashProgs [3]float32
	for i := range dashOptions {
		stepsUp := 2 - i
		target := float32(0)
		var anim ui.Animation
		if !inArchive && a.CaptureDialOpen {
			target = 1
			staggerStep := 45 * time.Millisecond
			dur := time.Duration(370+stepsUp*40) * time.Millisecond
			if d.Scrubbing {
				staggerStep = 22 * time.Millisecond
				dur = time.Duration(300+stepsUp*30) * time.Millisecond
			}
			anim = ui.Animation{
				Delay:  time.Duration(stepsUp) * staggerStep,
				Spring: ui.SpringOptions{Duration: dur, Bounce: 0.14},
			}
		} else {
			anim = ui.Animation{
				Delay:  time.Duration(i) * 24 * time.Millisecond,
				Spring: ui.SpringOptions{Duration: 260 * time.Millisecond, Bounce: 0},
			}
		}
		dashProgs[i] = dialAnchor.AnimateTo(dashOptions[i].animKey, target, anim)
	}

	archTarget := float32(0)
	archAnim := ui.Animation{
		Spring: ui.SpringOptions{Duration: 250 * time.Millisecond, Bounce: 0},
	}
	if inArchive && a.ArchiveDialOpen {
		archTarget = 1
		dur := 360 * time.Millisecond
		if d.Scrubbing {
			dur = 290 * time.Millisecond
		}
		archAnim = ui.Animation{
			Spring: ui.SpringOptions{Duration: dur, Bounce: 0.15},
		}
	}
	archProg := dialAnchor.AnimateTo(archOpt.animKey, archTarget, archAnim)
	if !a.ArchiveDialOpen && archProg <= 0.005 {
		d.HasLatchedAlt = false
	}

	interactiveOpts := !a.CaptureModalOpen

	dialAnchor.Children(func() {
		// 1. Floating monochrome icon-only option circles (rendered before sealBtn so they emerge from and tuck beneath it).
		if inArchive {
			if a.ArchiveDialOpen || archProg > 0.005 {
				isHovered := a.ArchiveDialOpen && d.Pressing && d.HoverIdx == 0
				item := archOpt
				a.renderFloatingOrbCircle(c, item, 0, archProg, a.ArchiveDialOpen && interactiveOpts, isHovered, func() {
					mygo.Haptics.Selection()
					a.closeOrbDials()
					item.onSelect()
				})
			}
		} else {
			for i, opt := range dashOptions {
				prog := dashProgs[i]
				if !a.CaptureDialOpen && prog <= 0.005 {
					continue
				}
				stepsUp := 2 - i
				isHovered := a.CaptureDialOpen && d.Pressing && d.HoverIdx == i
				item := opt
				a.renderFloatingOrbCircle(c, item, stepsUp, prog, a.CaptureDialOpen && interactiveOpts, isHovered, func() {
					mygo.Haptics.Selection()
					a.closeOrbDials()
					item.onSelect()
				})
			}
		}

		// 2. Main bottom-right monochrome circular button (zero mobile.Ink rectangular box shadow!).
		sealBtn := ui.ButtonBase(c).Key("dash_capture_fab").Bind(&a.DashFabHandle)

		transferHover := dialOpen && d.Pressing && d.HoverIdx >= 0
		sealBgTarget := orbMonoBlack
		sealFgTarget := orbMonoWhite
		sealBorderTarget := orbMonoBorderDark
		if transferHover {
			// Smooth black/white polarity transfer while sliding onto a target orb!
			sealBgTarget = orbMonoWhite
			sealFgTarget = orbMonoBlack
			sealBorderTarget = orbMonoBorderLight
		} else if d.Pressing || sealBtn.Pressed() {
			sealBgTarget = orbMonoBlackActive
		} else if sealBtn.Hovered() {
			sealBgTarget = orbMonoBlackHover
		}

		sealColorAnim := ui.Animation{Duration: 150 * time.Millisecond, Ease: ui.EaseOut, Oklab: true}
		sealBg := sealBtn.AnimateColor("seal_bg", sealBgTarget, sealColorAnim)
		sealFg := sealBtn.AnimateColor("seal_fg", sealFgTarget, sealColorAnim)
		sealBorder := sealBtn.AnimateColor("seal_bd", sealBorderTarget, sealColorAnim)

		pressTarget := float32(1.0)
		if transferHover {
			pressTarget = 0.93
		} else if d.Pressing || sealBtn.Pressed() {
			pressTarget = 0.94
		} else if sealBtn.Hovered() {
			pressTarget = 1.03
		}
		pressScale := sealBtn.Spring("press", pressTarget, ui.SpringOptions{Duration: 200 * time.Millisecond, Bounce: 0.12})

		rotTarget := float32(0)
		if !inArchive && a.CaptureDialOpen {
			rotTarget = 45
		}
		plusAngle := sealBtn.Spring("dial_rot", rotTarget, ui.SpringOptions{Duration: 280 * time.Millisecond, Bounce: 0.12})

		graphMixTarget := float32(0)
		if a.ArchiveMode == ArchiveGraph {
			graphMixTarget = 1
		}
		graphMix := sealBtn.Spring("arch_mode_mix", graphMixTarget, ui.SpringOptions{Duration: 300 * time.Millisecond, Bounce: 0.12})

		var sealLabel string
		if dialOpen {
			sealLabel = "收起"
		} else if inArchive {
			if a.ArchiveMode == ArchiveGraph {
				sealLabel = "切换视窗（当前脉络）"
			} else {
				sealLabel = "切换视窗（当前卡片）"
			}
		} else {
			sealLabel = "记一笔"
		}

		sealBtn.Absolute().Left(0).Bottom(0).
			Size(orbDialMainSize, orbDialMainSize).Radius(orbDialMainSize * 0.5).Center().
			Background(sealBg).
			Border(1, sealBorder).
			Shadow(0, 7, 20, 0, ui.RGBA(0, 0, 0, 0.18)).
			Scale(pressScale, pressScale).
			FocusRing(false).
			Cursor(ui.CursorPointer).
			Label(sealLabel)
		if a.CaptureModalOpen {
			sealBtn.PassThrough()
		} else {
			sealBtn.Draggable(&d.DragDummy, ui.DraggableOptions{
				Axis:   ui.Horizontal | ui.Vertical,
				Bounds: &ui.Rect{},
				Rigid:  true,
			}).HandleInput(func(ev ui.InputEvent) bool {
				switch ev.Kind {
				case ui.InputPointerDown:
					if ev.Button != 0 {
						return false
					}
					d.Pressing = true
					d.WasOpen = dialOpen
					d.Scrubbing = false
					d.EnteredOption = false
					d.HoverIdx = -1
					d.DownX = ev.X
					d.DownY = ev.Y
					d.PointerX = ev.X
					d.PointerY = ev.Y
					if !ev.Time.IsZero() {
						d.DownAt = ev.Time
					} else {
						d.DownAt = c.Now()
					}
					if !dialOpen {
						mygo.Haptics.Light()
						setDialOpen(true)
					}
					return true

				case ui.InputPointerMove:
					if !d.Pressing {
						return false
					}
					d.PointerX = ev.X
					d.PointerY = ev.Y
					moved := float32(math.Hypot(float64(ev.X-d.DownX), float64(ev.Y-d.DownY)))
					now := ev.Time
					if now.IsZero() {
						now = c.Now()
					}
					held := !d.DownAt.IsZero() && now.Sub(d.DownAt) >= orbDialHoldThreshold
					hit := hitOrbDialOption(ev.X, ev.Y, len(activeOptions))
					if !d.Scrubbing && (moved >= 6 || held || hit >= 0) {
						d.Scrubbing = true
						if !dialOpen {
							mygo.Haptics.Medium()
							setDialOpen(true)
							dialOpen = true
						}
					}
					if dialOpen {
						if hit >= 0 {
							d.EnteredOption = true
						}
						if hit != d.HoverIdx {
							d.HoverIdx = hit
							if hit >= 0 {
								mygo.Haptics.Selection()
							}
						}
					}
					return true

				case ui.InputPointerUp:
					if !d.Pressing {
						return false
					}
					wasScrubbing := d.Scrubbing
					wasOpen := d.WasOpen
					enteredOpt := d.EnteredOption
					hoverIdx := d.HoverIdx
					if dialOpen && hoverIdx < 0 && wasScrubbing {
						hoverIdx = hitOrbDialOption(ev.X, ev.Y, len(activeOptions))
					}
					d.Pressing = false
					d.Scrubbing = false
					d.EnteredOption = false
					d.HoverIdx = -1

					if ev.Canceled {
						if !wasOpen {
							setDialOpen(false)
						}
						return true
					}
					if dialOpen && hoverIdx >= 0 && hoverIdx < len(activeOptions) {
						mygo.Haptics.Selection()
						a.closeOrbDials()
						activeOptions[hoverIdx].onSelect()
						return true
					}
					if !wasScrubbing {
						if wasOpen {
							mygo.Haptics.Light()
						}
						setDialOpen(!wasOpen)
						return true
					}
					if enteredOpt {
						mygo.Haptics.Selection()
						setDialOpen(false)
					}
					return true

				case ui.InputKeyDown:
					if ev.Key == ui.KeyEnter || ev.Key == ui.KeySpace {
						mygo.Haptics.Light()
						setDialOpen(!dialOpen)
						return true
					}
				}
				return false
			})
		}

		if !dialOpen && !phone && !d.Pressing {
			if inArchive {
				sealBtn.Tooltip("按住上滑切换视窗")
			} else {
				sealBtn.Tooltip("记一笔")
			}
		}

		sealBtn.Children(func() {
			if t < 0.99 {
				dashScale := 1.0 - 0.28*t
				ui.Box(c).Absolute().Fill().Center().Opacity(1.0 - t).Scale(dashScale, dashScale).PassThrough().Children(func() {
					ui.Icon(c, IconPlus).Size(22, 22).TextColor(sealFg).Rotate(plusAngle)
				})
			}
			if t > 0.01 {
				archScale := 0.72 + 0.28*t
				ui.Box(c).Absolute().Fill().Center().Opacity(t).Scale(archScale, archScale).PassThrough().Children(func() {
					cardsAlpha := clamp01f(1.0 - graphMix)
					graphAlpha := clamp01f(graphMix)
					if cardsAlpha > 0.01 {
						s := 0.72 + 0.28*cardsAlpha
						ui.Box(c).Absolute().Fill().Center().Opacity(cardsAlpha).Scale(s, s).PassThrough().Children(func() {
							ui.Icon(c, IconCards).Size(20, 20).TextColor(sealFg).Rotate(-22 * graphMix)
						})
					}
					if graphAlpha > 0.01 {
						s := 0.72 + 0.28*graphAlpha
						ui.Box(c).Absolute().Fill().Center().Opacity(graphAlpha).Scale(s, s).PassThrough().Children(func() {
							ui.Icon(c, IconConstellation).Size(20, 20).TextColor(sealFg).Rotate(22 * (1.0 - graphMix))
						})
					}
				})
			}
		})
	})
}
