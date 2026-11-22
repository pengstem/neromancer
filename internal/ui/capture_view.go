package ui

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/imagepicker"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

const (
	captureOpenDur        = 380 * time.Millisecond
	captureCloseOriginDur = 290 * time.Millisecond
	captureClosePalaceDur = 760 * time.Millisecond
	capturePortalPulseDur = 520 * time.Millisecond
)

// CaptureAnimPhase tracks the lifecycle of the full-screen capture surface:
// local-orb expansion -> full-screen editing -> either return-to-origin (cancel)
// or two-beat un-fullscreen + upward tuck into the Mind Palace (submit).
type CaptureAnimPhase uint8

const (
	CaptureAnimIdle CaptureAnimPhase = iota
	CaptureAnimOpening
	CaptureAnimOpen
	CaptureAnimClosingOrigin
	CaptureAnimClosingPalace
)

// CaptureTransitionState stores frame-persistent geometry and snapshots for
// silky container-transform and Mind Palace absorption animations.
type CaptureTransitionState struct {
	Phase            CaptureAnimPhase
	LastOpenFlag     bool
	PhaseStart       time.Time
	OriginRect       ui.Rect
	OpenedFromSpace  Space
	SubmittedTitle   string
	SubmittedSnippet string
	SubmittedTheme   domain.OrbTheme
	SubmittedIconKey string
	PortalPulseAt    time.Time
}

func clamp01f(v float32) float32 {
	if v <= 0 {
		return 0
	}
	if v >= 1 {
		return 1
	}
	return v
}

func lerp32(a, b, t float32) float32 {
	return a + (b-a)*t
}

func lerpRect(a, b ui.Rect, t float32) ui.Rect {
	return ui.Rect{
		X: lerp32(a.X, b.X, t),
		Y: lerp32(a.Y, b.Y, t),
		W: lerp32(a.W, b.W, t),
		H: lerp32(a.H, b.H, t),
	}
}

func smoothstep32(edge0, edge1, x float32) float32 {
	if edge1 <= edge0 {
		if x >= edge1 {
			return 1
		}
		return 0
	}
	t := clamp01f((x - edge0) / (edge1 - edge0))
	return t * t * (3.0 - 2.0*t)
}

func easeInOutCubic32(t float32) float32 {
	t = clamp01f(t)
	if t < 0.5 {
		return 4.0 * t * t * t
	}
	f := -2.0*t + 2.0
	return 1.0 - (f*f*f)*0.5
}

func cubicBezier32(p0, p1, p2, p3, u float32) float32 {
	u = clamp01f(u)
	inv := 1.0 - u
	return inv*inv*inv*p0 +
		3.0*inv*inv*u*p1 +
		3.0*inv*u*u*p2 +
		u*u*u*p3
}

type captureModeSpec struct {
	title    string
	subtitle string
	icon     *ui.SVG
	theme    domain.OrbTheme
}

func specForCaptureModality(idx int) captureModeSpec {
	switch idx {
	case 0:
		return captureModeSpec{
			title:    "语音记录",
			subtitle: "说完自动转写，AI 帮你归入思维宫殿",
			icon:     IconMic,
			theme:    domain.ThemeMint,
		}
	case 2:
		return captureModeSpec{
			title:    "图文记录",
			subtitle: "留住画面与当下的闪念，自动整理归档",
			icon:     IconImage,
			theme:    domain.ThemeAmber,
		}
	default:
		return captureModeSpec{
			title:    "文字记录",
			subtitle: "想到什么直接写，AI 自动帮你归类串联",
			icon:     IconQuill,
			theme:    domain.ThemeSky,
		}
	}
}

// openCaptureModal launches the dedicated full-screen editor for modalityIdx,
// expanding smoothly from originRect in window coordinates.
func (a *App) openCaptureModal(modalityIdx int, originRect ui.Rect) {
	a.CaptureModalityIdx = modalityIdx
	a.CaptureModalOpen = true
	a.CaptureAnim.OriginRect = originRect
	a.CaptureAnim.OpenedFromSpace = a.ActiveSpace
	a.CaptureAnim.LastOpenFlag = true
	a.CaptureAnim.Phase = CaptureAnimOpening
	a.CaptureAnim.PhaseStart = time.Time{}
}

// dismissCaptureModal cancels capture and smoothly shrinks the full-screen card back to its local origin.
func (a *App) dismissCaptureModal() {
	if a.Deps.Media != nil && a.Deps.Media.IsRecording() {
		_, _, _ = a.Deps.Media.StopRecording()
	}
	a.CaptureModalOpen = false
	a.CaptureAnim.LastOpenFlag = false
	a.CaptureAnim.Phase = CaptureAnimClosingOrigin
	a.CaptureAnim.PhaseStart = time.Time{}
}

// mindPalacePortalPulse returns a smooth [0..1] reception glow factor for the top Mind Palace bar
// right as a newly captured idea tucks into the ceiling.
func (a *App) mindPalacePortalPulse(c *ui.Context) float32 {
	if c == nil || c.Preferences().ReduceMotion || a.CaptureAnim.PortalPulseAt.IsZero() {
		return 0
	}
	dt := c.Now().Sub(a.CaptureAnim.PortalPulseAt)
	if dt < 0 || dt >= capturePortalPulseDur {
		if dt >= capturePortalPulseDur {
			a.CaptureAnim.PortalPulseAt = time.Time{}
		}
		return 0
	}
	c.AnimationFrame()
	u := clamp01f(float32(dt) / float32(capturePortalPulseDur))
	return float32(math.Sin(float64(u) * math.Pi))
}

func (a *App) resolveCaptureOriginRect(c *ui.Context, winW, winH float32, preferRestingAnchor bool) ui.Rect {
	if preferRestingAnchor {
		if a.CaptureAnim.OpenedFromSpace == SpaceArchive {
			if b := a.ArchiveAddHandle.Bounds(c); b.W > 0 && b.H > 0 {
				return b
			}
		} else if b := a.DashFabHandle.Bounds(c); b.W > 0 && b.H > 0 {
			return b
		}
	}
	if a.CaptureAnim.OriginRect.W > 0 && a.CaptureAnim.OriginRect.H > 0 {
		return a.CaptureAnim.OriginRect
	}
	if b := a.DashFabHandle.Bounds(c); b.W > 0 && b.H > 0 {
		return b
	}
	phone := a.isPhoneLayout(c)
	_, _, bottomSafe, _ := c.SafeArea()
	rightPad := float32(18)
	bottomPad := float32(20) + bottomSafe
	if !phone {
		rightPad = 28
		bottomPad = 26 + bottomSafe
	}
	return ui.Rect{
		X: winW - rightPad - orbDialMainSize,
		Y: winH - bottomPad - orbDialMainSize,
		W: orbDialMainSize,
		H: orbDialMainSize,
	}
}

func (a *App) syncCaptureAnimState(c *ui.Context, now time.Time, reduceMotion bool) {
	if a.CaptureModalOpen && !a.CaptureAnim.LastOpenFlag {
		a.CaptureAnim.LastOpenFlag = true
		a.CaptureAnim.OpenedFromSpace = a.ActiveSpace
		if reduceMotion {
			a.CaptureAnim.Phase = CaptureAnimOpen
		} else {
			a.CaptureAnim.Phase = CaptureAnimOpening
			a.CaptureAnim.PhaseStart = now
		}
	} else if !a.CaptureModalOpen && a.CaptureAnim.LastOpenFlag {
		a.CaptureAnim.LastOpenFlag = false
		if reduceMotion {
			a.CaptureAnim.Phase = CaptureAnimIdle
		} else if a.CaptureAnim.Phase != CaptureAnimClosingPalace && a.CaptureAnim.Phase != CaptureAnimClosingOrigin {
			a.CaptureAnim.Phase = CaptureAnimClosingOrigin
			a.CaptureAnim.PhaseStart = now
		}
	}

	if reduceMotion {
		if a.CaptureModalOpen {
			a.CaptureAnim.Phase = CaptureAnimOpen
		} else {
			a.CaptureAnim.Phase = CaptureAnimIdle
		}
		return
	}

	if a.CaptureAnim.Phase != CaptureAnimIdle && a.CaptureAnim.Phase != CaptureAnimOpen && a.CaptureAnim.PhaseStart.IsZero() {
		a.CaptureAnim.PhaseStart = now
	}
}

func (a *App) renderCaptureModal(c *ui.Context) {
	now := c.Now()
	reduceMotion := c.Preferences().ReduceMotion
	a.syncCaptureAnimState(c, now, reduceMotion)

	if !a.CaptureModalOpen && a.CaptureAnim.Phase == CaptureAnimIdle {
		return
	}

	pal := a.Palette()
	winW, winH := c.Size()
	if winW <= 0 || winH <= 0 {
		winW, winH = 412, 892
	}
	topSafe, _, bottomSafe, _ := c.SafeArea()
	spec := specForCaptureModality(a.CaptureModalityIdx)
	modeColors := ColorsForTheme(spec.theme)

	fullRect := ui.Rect{X: 0, Y: 0, W: winW, H: winH}
	vRect := fullRect
	vRad := float32(0)
	scrimAlpha := float32(0.36)
	editorAlpha := float32(1.0)
	orbSeedAlpha := float32(0.0)
	capsuleAlpha := float32(0.0)
	morphing := false

	switch a.CaptureAnim.Phase {
	case CaptureAnimOpening:
		dt := now.Sub(a.CaptureAnim.PhaseStart)
		if dt >= captureOpenDur {
			a.CaptureAnim.Phase = CaptureAnimOpen
		} else {
			c.AnimationFrame()
			morphing = true
			sp := ui.SpringOptions{Duration: captureOpenDur, Bounce: 0}
			p := clamp01f(float32(sp.Progress(dt)))
			orig := a.resolveCaptureOriginRect(c, winW, winH, false)
			orad := min(orig.W, orig.H) * 0.5
			vRect = lerpRect(orig, fullRect, p)
			vRad = orad*(1.0-p) + 28.0*4.0*p*(1.0-p)*(1.0-p)
			scrimAlpha = 0.36 * p
			editorAlpha = smoothstep32(0.14, 0.72, p)
			orbSeedAlpha = 1.0 - smoothstep32(0.0, 0.42, p)
		}

	case CaptureAnimClosingOrigin:
		dt := now.Sub(a.CaptureAnim.PhaseStart)
		if dt >= captureCloseOriginDur {
			a.CaptureAnim.Phase = CaptureAnimIdle
			return
		}
		c.AnimationFrame()
		morphing = true
		u := clamp01f(float32(dt) / float32(captureCloseOriginDur))
		p := 1.0 - easeInOutCubic32(u)
		dest := a.resolveCaptureOriginRect(c, winW, winH, true)
		drad := min(dest.W, dest.H) * 0.5
		vRect = lerpRect(dest, fullRect, p)
		vRad = drad*(1.0-p) + 24.0*4.0*p*(1.0-p)*(1.0-p)
		scrimAlpha = 0.36 * p
		editorAlpha = smoothstep32(0.26, 0.86, p)
		orbSeedAlpha = (1.0 - smoothstep32(0.0, 0.44, p)) * smoothstep32(0.0, 0.22, 1.0-u)

	case CaptureAnimClosingPalace:
		dt := now.Sub(a.CaptureAnim.PhaseStart)
		if dt >= captureClosePalaceDur {
			a.CaptureAnim.Phase = CaptureAnimIdle
			return
		}
		c.AnimationFrame()
		morphing = true
		u := clamp01f(float32(dt) / float32(captureClosePalaceDur))
		if u >= 0.80 && a.CaptureAnim.PortalPulseAt.IsZero() {
			a.CaptureAnim.PortalPulseAt = now
		}

		// Beat 1 (0..0.44): "慢慢全屏切回去" — gracefully peel back from full-screen into a floating Idea Capsule.
		// Beat 2 (0.44..1.0): "然后缩进上方" — glide all the way up to the Mind Palace header and tuck into the ceiling.
		capW := min(float32(248), winW-40)
		capH := float32(56)
		endW := float32(84)
		endH := float32(20)
		midY := winH * 0.425
		topTargetY := topSafe - 4

		var vw, vh, vcy float32
		if u <= 0.44 {
			t1 := easeInOutCubic32(u / 0.44)
			vw = lerp32(winW, capW, t1)
			vh = lerp32(winH, capH, t1)
			vRad = lerp32(0, capH*0.5, smoothstep32(0.04, 0.88, t1))
			vcy = lerp32(winH*0.50, midY, t1)
		} else {
			s2 := (u - 0.44) / 0.56
			t2 := easeInOutCubic32(s2)
			shrinkT := easeInOutCubic32(clamp01f((s2 - 0.12) / 0.88))
			vw = lerp32(capW, endW, shrinkT)
			vh = lerp32(capH, endH, shrinkT)
			vRad = vh * 0.5
			vcy = lerp32(midY, topTargetY, t2)
		}

		vcx := winW * 0.50
		vRect = ui.Rect{X: vcx - vw*0.5, Y: vcy - vh*0.5, W: vw, H: vh}

		scrimAlpha = 0.34 * (1.0 - smoothstep32(0.0, 0.40, u))
		editorAlpha = 1.0 - smoothstep32(0.0, 0.28, u)
		tuckFade := 1.0 - smoothstep32(0.84, 1.00, u)
		capsuleAlpha = smoothstep32(0.16, 0.42, u) * tuckFade
	}

	interactive := a.CaptureModalOpen
	root := ui.Box(c).Key("capture_fullscreen_overlay").Absolute().Fill().ZIndex(85)
	if !interactive {
		root.PassThrough()
	}

	root.Children(func() {
		// 1. Scrim + outer elevation shadow painted in window coordinates
		if scrimAlpha > 0.004 || morphing {
			scrim := ui.Box(c).Key("capture_fullscreen_scrim").Absolute().Fill().
				Background(ui.RGBA(18, 22, 33, scrimAlpha))
			if !interactive {
				scrim.PassThrough()
			}
			if morphing {
				shadowRect := vRect
				shadowRad := vRad
				shadowOp := float32(1.0)
				if a.CaptureAnim.Phase == CaptureAnimClosingPalace {
					u := clamp01f(float32(now.Sub(a.CaptureAnim.PhaseStart)) / float32(captureClosePalaceDur))
					shadowOp = 1.0 - smoothstep32(0.78, 1.00, u)
				}
				scrim.Draw(func(p *ui.Painter, _ ui.Rect) {
					if shadowOp > 0.01 {
						p.Shadow(shadowRect, shadowRad, 0, 12, 32, 0, ui.RGBA(18, 22, 33, 0.16*shadowOp))
					}
				})
			}
		}

		// 2. Full-screen editor surface (zero text reflow via uniform Scale + ClipInset during morph)
		showEditorSurface := !morphing || editorAlpha > 0.01 || orbSeedAlpha > 0.01
		if showEditorSurface {
			surfaceBg := pal.Canvas
			if orbSeedAlpha > 0.05 {
				surfaceBg = lerpColor(pal.Canvas, pal.Card, orbSeedAlpha)
			}
			panel := ui.Column(c).Key("capture_fullscreen_panel").
				Absolute().Left(0).Top(0).Size(winW, winH).
				Background(surfaceBg)

			if interactive {
				panel.Role(ui.RoleDialog).Label(spec.title).
					HandleInput(func(ev ui.InputEvent) bool {
						if ev.Kind == ui.InputKeyDown && ev.Key == ui.KeyEscape {
							mygo.Haptics.Selection()
							a.dismissCaptureModal()
							return true
						}
						return false
					})
			} else {
				panel.PassThrough()
			}

			if morphing {
				vw := max(float32(16), vRect.W)
				vh := max(float32(16), vRect.H)
				s := max(float32(0.04), min(float32(1.0), max(vw/winW, vh/winH)))
				localW := min(winW, vw/s)
				localH := min(winH, vh/s)
				insX := max(float32(0), (winW-localW)*0.5)
				insY := max(float32(0), (winH-localH)*0.5)
				localRad := min(min(localW, localH)*0.5, vRad/s)
				tx := (vRect.X + vRect.W*0.5) - winW*0.5
				ty := (vRect.Y + vRect.H*0.5) - winH*0.5

				borderCol := pal.Border.Alpha(0.85)
				panel.ClipInset(insY, insX, insY, insX, localRad).
					TransformOrigin(0.5, 0.5).
					Scale(s, s).
					Translate(tx, ty).
					DrawOver(func(p *ui.Painter, r ui.Rect) {
						inner := ui.Rect{X: r.X + insX, Y: r.Y + insY, W: localW, H: localH}
						p.Stroke(inner, borderCol, localRad, 1.0/s)
						if orbSeedAlpha > 0.02 {
							icSz := float32(18) / s
							icRect := ui.Rect{
								X: inner.X + (inner.W-icSz)*0.5,
								Y: inner.Y + (inner.H-icSz)*0.5,
								W: icSz,
								H: icSz,
							}
							p.Icon(spec.icon, icRect, pal.Ink.Alpha(orbSeedAlpha))
						}
					})

				if a.CaptureAnim.Phase == CaptureAnimClosingPalace {
					u := clamp01f(float32(now.Sub(a.CaptureAnim.PhaseStart)) / float32(captureClosePalaceDur))
					panel.Opacity(1.0 - smoothstep32(0.20, 0.46, u))
				}
			}

			panel.Children(func() {
				if editorAlpha > 0.01 {
					a.renderDedicatedCaptureWorkspace(c, pal, spec, modeColors, topSafe, bottomSafe, editorAlpha, interactive)
				}
			})
		}

		// 3. Condensed Idea Capsule for Beat 1 -> Beat 2 ("慢慢全屏切回去，然后缩进上方思维宫殿")
		if capsuleAlpha > 0.01 {
			a.renderFlyingPalaceCapsule(c, pal, vRect, vRad, capsuleAlpha)
		}
	})
}

func (a *App) renderFlyingPalaceCapsule(c *ui.Context, pal SurfacePalette, vRect ui.Rect, vRad, alpha float32) {
	subColors := ColorsForTheme(a.CaptureAnim.SubmittedTheme)
	iconSVG := SVGForIconKey(a.CaptureAnim.SubmittedIconKey)
	title := strings.TrimSpace(a.CaptureAnim.SubmittedTitle)
	if title == "" {
		title = "新灵感"
	}

	// Scale content smoothly inside the capsule once Beat 2 tapers it toward the top ceiling
	baseW := float32(248)
	baseH := float32(56)
	contentScale := min(float32(1.0), min(vRect.W/baseW, vRect.H/baseH))

	ui.Box(c).Key("capture_palace_capsule").
		Absolute().Left(vRect.X).Top(vRect.Y).Size(vRect.W, vRect.H).
		Radius(vRad).Clip().PassThrough().
		Opacity(alpha).
		Background(pal.Card).
		Border(1, subColors.Mid.Alpha(0.42)).
		Center().
		Children(func() {
			ui.Row(c).Width(baseW).Height(baseH).Padding(0, 16).Gap(10).
				AlignItems(ui.Center).Justify(ui.Center).
				Scale(contentScale, contentScale).
				Children(func() {
					ui.Box(c).Size(28, 28).Radius(14).Center().Shrink(0).
						Background(subColors.BadgeBg).
						Children(func() {
							ui.Icon(c, iconSVG).Size(15, 15).TextColor(subColors.BadgeText)
						})
					ui.Column(c).Grow(1).MinWidth(0).Gap(1.5).Children(func() {
						ui.Textf(c, "《%s》", title).
							FontSize(FontBody).Bold().TextColor(pal.Ink).SingleLine()
						ui.Text(c, "正在收入上方思维宫殿…").
							FontSize(FontCaption).TextColor(subColors.BadgeText).SingleLine()
					})
				})
		})
}

func (a *App) renderDedicatedCaptureWorkspace(
	c *ui.Context,
	pal SurfacePalette,
	spec captureModeSpec,
	modeColors OrbColorSet,
	topSafe, bottomSafe, alpha float32,
	interactive bool,
) {
	phone := a.isPhoneLayout(c)
	padX := float32(18)
	if !phone {
		padX = 28
	}

	buildColumn := func() {
		ui.Column(c).Fill().Padding(14+topSafe, padX, 16+bottomSafe, padX).Gap(14).
			Opacity(alpha).
			Children(func() {
				// Top Full-Screen Dedicated Modality Header (no modality switcher tabs!)
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
					ui.Box(c).Size(34, 34).Radius(11).Center().Shrink(0).
						Background(modeColors.BadgeBg).
						Border(1, modeColors.Mid.Alpha(0.22)).
						Children(func() {
							ui.Icon(c, spec.icon).Size(17, 17).TextColor(modeColors.BadgeText)
						})

					ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
						ui.Text(c, spec.title).FontSize(FontTitle).Bold().TextColor(pal.Ink).SingleLine()
						ui.Text(c, spec.subtitle).FontSize(FontCaption).TextColor(pal.InkMuted).SingleLine()
					})

					closeBtn := GhostIconAction(c, "capture_close", IconClose, "收起", false)
					if interactive && closeBtn.Clicked() {
						mygo.Haptics.Selection()
						a.dismissCaptureModal()
					}
				})

				ui.Box(c).FillWidth().Height(1).Background(pal.Border.Alpha(0.75))

				// Dedicated Center Workspace per Entry Modality
				ui.Scroll(c).Key("capture_editor_scroll").Grow(1).FillWidth().MinHeight(0).Children(func() {
					ui.Column(c).FillWidth().Gap(14).Padding(2, 0, 8, 0).Children(func() {
						switch a.CaptureModalityIdx {
						case 0:
							a.renderVoiceDedicatedEditor(c, pal, interactive)
						case 2:
							a.renderImageDedicatedEditor(c, pal, modeColors, interactive)
						default:
							a.renderTextDedicatedEditor(c, pal, interactive)
						}

						a.renderCaptureRecallAndScene(c, pal, interactive)
					})
				})

				// Bottom Primary Submit Button (AI auto-organize runs automatically by default)
				submitBtn := ui.ButtonBase(c).Key("capture_submit")
				pressScale := submitBtn.Spring("press", ternaryFloat(submitBtn.Pressed(), 0.98, 1.0))
				submitBtn.FillWidth().Padding(13, 18).Radius(RadiusControl).Center().
					Background(pal.Ink).TextColor(pal.Card).
					Shadow(0, 6, 18, 0, ui.RGBA(20, 21, 25, 0.14)).
					Scale(pressScale, pressScale).
					Cursor(ui.CursorPointer).Label("记下来")
				mobile.Ink(submitBtn)
				submitBtn.OnClick(func() {
					if interactive {
						a.submitQuickCapture("")
					}
				}).Children(func() {
					ui.Row(c).Gap(7).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconCheck).Size(16, 16).TextColor(pal.Card)
						ui.Text(c, "记下来").FontSize(FontCardTitle).Bold().TextColor(pal.Card).SingleLine()
					})
				})
			})
	}

	if phone {
		buildColumn()
		return
	}
	ui.Column(c).Fill().AlignItems(ui.Center).Children(func() {
		ui.Box(c).FillHeight().FillWidth().MaxWidth(600).Children(buildColumn)
	})
}

func ternaryFloat(cond bool, a, b float32) float32 {
	if cond {
		return a
	}
	return b
}

func (a *App) renderTextDedicatedEditor(c *ui.Context, pal SurfacePalette, interactive bool) {
	StyleSurfaceCard(
		ui.Column(c).FillWidth().Padding(16, 18).MinHeight(340),
		pal,
	).Children(func() {
		if !interactive {
			snap := a.CaptureAnim.SubmittedSnippet
			if snap == "" {
				snap = a.CaptureText
			}
			ui.Text(c, snap).FontSize(FontCardTitle).LineHeight(1.55).TextColor(pal.Ink)
			return
		}

		ta := ui.TextAreaBase(c, &a.CaptureText).
			Placeholder("想到什么直接记下来…").
			PlaceholderColor(pal.InkMuted).
			TextColor(pal.Ink).
			FontSize(FontCardTitle).
			Label("灵感内容").
			Height(310).FillWidth()
		if a.CaptureAnim.Phase == CaptureAnimOpen {
			ta.AutoFocus()
		}
		if ta.Changed() {
			if hits, err := a.Deps.Organize.SearchRecall(a.CaptureText, 2); err == nil {
				a.CaptureRecallHits = hits
			}
		}
	})
}

func (a *App) renderVoiceDedicatedEditor(c *ui.Context, pal SurfacePalette, interactive bool) {
	phase := a.Phase(c)
	recording := a.Deps.Media != nil && a.Deps.Media.IsRecording()

	energy := float32(0.32)
	btnLabel := "开始录音"
	caption := "点一下开始说话"
	pulseColor := ColorVermilion.Alpha(0.55)
	btnBg := pal.Ink
	centerIcon := IconMic
	if recording {
		energy = 1.0
		btnLabel = "结束录音"
		caption = "正在听你说，点一下结束"
		pulseColor = ColorVermilion
		btnBg = ColorVermilion
		centerIcon = IconStop
	} else if a.CaptureAudioPath != "" {
		caption = "录音已就绪，点下方「记下来」自动转成文字"
	}

	StyleSurfaceCard(
		ui.Column(c).FillWidth().Padding(22, 16).Gap(12).Center(),
		pal,
	).Children(func() {
		ui.Box(c).Size(176, 160).Center().
			Draw(func(p *ui.Painter, r ui.Rect) {
				DrawVoicePulse(p, r, phase, energy, pulseColor)
			}).
			Children(func() {
				micBtn := ui.ButtonBase(c).Key("voice_centerpiece_btn").Size(90, 90).Radius(45).
					Center().Background(btnBg).
					Shadow(0, 8, 22, 0, ui.RGBA(20, 21, 25, 0.16)).
					Cursor(ui.CursorPointer).Label(btnLabel)
				mobile.Ink(micBtn)
				micBtn.Children(func() {
					ui.Icon(c, centerIcon).Size(26, 26).TextColor(pal.Card)
				})

				if interactive && micBtn.Clicked() {
					a.toggleVoiceRecording()
				}
			})

		ui.Text(c, caption).FontSize(FontBody).Bold().TextColor(pal.InkSecondary)

		ui.Row(c).Gap(8).AlignItems(ui.Center).Center().Wrap().Children(func() {
			if PillAction(c, "pick_audio_btn", IconArchive, "导入录音", false).Clicked() && interactive {
				a.pickFileAsync(true)
			}
			if a.CaptureAudioPath != "" {
				ui.Row(c).Gap(4).Padding(4, 10).Radius(RadiusPill).Background(ColorSuccessSoft).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconCheck).Size(12, 12).TextColor(ColorSuccessInk)
					ui.Text(c, "已添加录音").FontSize(FontCaption).Bold().TextColor(ColorSuccessInk)
					if GhostIconAction(c, "clear_audio_btn", IconClose, "清除录音", false).
						Size(20, 20).Clicked() && interactive {
						a.CaptureAudioPath = ""
					}
				})
			}
		})
	})

	// Optional lightweight note companion for voice memos
	StyleSurfaceCard(
		ui.Box(c).FillWidth().Padding(12, 14),
		pal,
	).Children(func() {
		if !interactive {
			if s := strings.TrimSpace(a.CaptureAnim.SubmittedSnippet); s != "" {
				ui.Text(c, s).FontSize(FontBody).TextColor(pal.Ink)
			}
			return
		}
		ta := ui.TextAreaBase(c, &a.CaptureText).
			Placeholder("顺手补一句备注（可选）…").
			PlaceholderColor(pal.InkMuted).
			TextColor(pal.Ink).
			FontSize(FontBody).
			Label("补充备注").
			Height(76).FillWidth()
		if ta.Changed() {
			if hits, err := a.Deps.Organize.SearchRecall(a.CaptureText, 2); err == nil {
				a.CaptureRecallHits = hits
			}
		}
	})
}

func (a *App) renderImageDedicatedEditor(c *ui.Context, pal SurfacePalette, modeColors OrbColorSet, interactive bool) {
	if a.CaptureImagePath == "" {
		StyleSurfaceCard(
			ui.Column(c).FillWidth().Padding(26, 18).Gap(14).Center(),
			pal,
		).Children(func() {
			ui.Box(c).Size(52, 52).Radius(16).Center().
				Background(modeColors.BadgeBg).
				Border(1, modeColors.Mid.Alpha(0.22)).
				Children(func() {
					ui.Icon(c, IconImage).Size(24, 24).TextColor(modeColors.BadgeText)
				})

			ui.Column(c).Center().Gap(4).Children(func() {
				ui.Text(c, "拍下眼前的画面，或从相册选一张图").
					FontSize(FontBody).Bold().TextColor(pal.Ink)
				ui.Text(c, "图片原件会完整留存，随时可回溯当时的现场").
					FontSize(FontCaption).TextColor(pal.InkMuted)
			})

			ui.Row(c).Gap(10).AlignItems(ui.Center).Center().Wrap().Children(func() {
				if PillAction(c, "capture_camera", IconCamera, "拍照", true).Clicked() && interactive {
					a.captureCameraAsync()
				}
				if PillAction(c, "capture_gallery", IconImage, "相册选图", false).Clicked() && interactive {
					a.pickFileAsync(false)
				}
			})
		})
	} else {
		StyleSurfaceCard(
			ui.Column(c).FillWidth().Padding(12).Gap(10),
			pal,
		).Children(func() {
			if src := a.loadCachedImage(a.CaptureImagePath); src != nil {
				ui.Image(c, src).FillWidth().Height(196).Radius(12).Fit(ui.Cover)
			} else {
				ui.Box(c).FillWidth().Height(140).Radius(12).Center().
					Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
					ui.Icon(c, IconImage).Size(26, 26).TextColor(pal.InkSecondary)
				})
			}

			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, "已选好图片").FontSize(FontMeta).Bold().TextColor(ColorSuccessInk)
					ui.Text(c, filepath.Base(a.CaptureImagePath)).FontSize(FontCaption).TextColor(pal.InkMuted).SingleLine()
				})
				if PillAction(c, "capture_gallery", IconImage, "换一张", false).Clicked() && interactive {
					a.pickFileAsync(false)
				}
				if GhostIconAction(c, "clear_image_btn", IconClose, "移除图片", false).Size(28, 28).Clicked() && interactive {
					a.CaptureImagePath = ""
				}
			})
		})
	}

	StyleSurfaceCard(
		ui.Box(c).FillWidth().Padding(14, 16),
		pal,
	).Children(func() {
		if !interactive {
			if s := strings.TrimSpace(a.CaptureAnim.SubmittedSnippet); s != "" {
				ui.Text(c, s).FontSize(FontBody).TextColor(pal.Ink)
			}
			return
		}
		ta := ui.TextAreaBase(c, &a.CaptureText).
			Placeholder("这张图让你想到了什么？写两句吧…").
			PlaceholderColor(pal.InkMuted).
			TextColor(pal.Ink).
			FontSize(FontBody).
			Label("灵感内容").
			Height(108).FillWidth()
		if ta.Changed() {
			if hits, err := a.Deps.Organize.SearchRecall(a.CaptureText, 2); err == nil {
				a.CaptureRecallHits = hits
			}
		}
	})
}

func (a *App) renderCaptureRecallAndScene(c *ui.Context, pal SurfacePalette, interactive bool) {
	if len(a.CaptureRecallHits) > 0 && strings.TrimSpace(a.CaptureText) != "" {
		hit := a.CaptureRecallHits[0]
		hitPal := ColorsForTheme(hit.Node.Theme)
		ui.Row(c).FillWidth().Padding(10, 12).Gap(9).AlignItems(ui.Center).Radius(RadiusControl).
			Background(pal.Card).Border(1, pal.Border.Alpha(0.75)).Children(func() {
			ui.Icon(c, SVGForIconKey(domain.ResolveIdeaIconKey(hit.Node))).
				Size(16, 16).TextColor(hitPal.BadgeText).Shrink(0)
			ui.Textf(c, "之前记过：《%s》", hit.Node.Title).
				FontSize(FontMeta).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0).MaxLines(1)
			if PillAction(c, "capture_merge_hit", IconCheck, "合并进去", false).Clicked() && interactive {
				a.submitQuickCapture(hit.Node.ID)
			}
		})
	}

	ui.Row(c).FillWidth().AlignItems(ui.Center).Children(func() {
		if PillAction(c, "capture_scene_chip", IconMapPin, "记下当时场景", a.CaptureShowScene).Clicked() && interactive {
			a.CaptureShowScene = !a.CaptureShowScene
		}
	})

	if a.CaptureShowScene && interactive {
		ui.Column(c).FillWidth().Gap(8).Children(func() {
			ui.TextInput(c, &a.CaptureSceneLoc).Placeholder("地点，如：花店门口").Background(pal.Card).FillWidth()
			ui.Row(c).FillWidth().Gap(8).Children(func() {
				ui.TextInput(c, &a.CaptureSceneMood).Placeholder("心情或天气").Background(pal.Card).Grow(1).MinWidth(0)
				ui.TextInput(c, &a.CaptureSceneMusic).Placeholder("在听的歌").Background(pal.Card).Grow(1).MinWidth(0)
			})
		})
	}
}

func (a *App) toggleVoiceRecording() {
	if a.Deps.Media == nil {
		a.setErrorBanner("录音功能暂不可用")
		return
	}
	if a.Deps.Media.IsRecording() {
		path, durMs, err := a.Deps.Media.StopRecording()
		if err != nil {
			a.setErrorBanner(err.Error())
			return
		}
		mygo.Haptics.Success()
		a.CaptureAudioPath = path
		a.setBanner(fmt.Sprintf("已录 %.1f 秒，保存后会自动转成文字", float64(durMs)/1000.0))
		return
	}

	mygo.Haptics.Medium()
	path, err := a.Deps.Media.StartRecording()
	if err != nil {
		a.setErrorBanner(err.Error())
		return
	}
	a.CaptureAudioPath = path
	a.setBanner("正在录音，说完点一下即可结束")
}

func (a *App) captureCameraAsync() {
	go func() {
		if runtime.GOOS != "android" && runtime.GOOS != "ios" {
			a.Dispatch(func() {
				a.setBanner("电脑上请点「相册选图」选择本地图片，手机上可直接拍照")
			})
			return
		}
		path, err := imagepicker.Capture(context.Background())
		a.Dispatch(func() {
			if err != nil {
				a.setErrorBanner("打开相机失败：" + err.Error())
				return
			}
			if path != "" {
				a.CaptureImagePath = path
			}
		})
	}()
}

func (a *App) pickFileAsync(isAudio bool) {
	go func() {
		if !isAudio && (runtime.GOOS == "android" || runtime.GOOS != "darwin" && runtime.GOOS == "ios") {
			paths, err := imagepicker.Pick(context.Background(), imagepicker.Options{
				Kind:  imagepicker.Images,
				Limit: 1,
			})
			if err != nil {
				a.Dispatch(func() { a.setErrorBanner("打开相册失败：" + err.Error()) })
				return
			}
			if len(paths) > 0 {
				a.Dispatch(func() { a.CaptureImagePath = paths[0] })
			}
			return
		}

		title := "选择图片"
		filters := []mygo.FileFilter{{Name: "Images", Extensions: []string{"png", "jpg", "jpeg", "webp"}}}
		if isAudio {
			title = "选择录音文件"
			filters = []mygo.FileFilter{{Name: "Audio", Extensions: []string{"wav", "ogg", "opus"}}}
		}
		paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
			Parent:  a.MainWin,
			Title:   title,
			Filters: filters,
		})
		if err != nil || len(paths) == 0 {
			return
		}
		a.Dispatch(func() {
			if isAudio {
				a.CaptureAudioPath = paths[0]
			} else {
				a.CaptureImagePath = paths[0]
			}
		})
	}()
}

// submitQuickCapture commits raw evidence locally first (<5ms), triggers the two-beat
// full-screen -> floating capsule -> Mind Palace upward tuck animation, and always
// runs background AI auto-organization by default when not manually merged.
func (a *App) submitQuickCapture(targetNodeID string) {
	if a.Deps.Media != nil && a.Deps.Media.IsRecording() {
		if path, _, err := a.Deps.Media.StopRecording(); err == nil && path != "" {
			a.CaptureAudioPath = path
		}
	}

	modality := "text"
	switch a.CaptureModalityIdx {
	case 0:
		modality = "voice"
	case 2:
		modality = "image"
	}

	in := service.QuickCaptureInput{
		RawText:      a.CaptureText,
		Modality:     modality,
		AudioSrcPath: a.CaptureAudioPath,
		ImageSrcPath: a.CaptureImagePath,
		TargetNodeID: targetNodeID,
		AutoOrganize: false,
		Scene: domain.SceneContext{
			Location:    a.CaptureSceneLoc,
			MoodWeather: a.CaptureSceneMood,
			NowPlaying:  a.CaptureSceneMusic,
		},
	}

	if strings.TrimSpace(in.RawText) == "" && in.AudioSrcPath == "" && in.ImageSrcPath == "" {
		a.setErrorBanner("先说句话、写几个字，或者选张图吧")
		return
	}

	out, err := a.Deps.Capture.CaptureQuick(context.Background(), in)
	if err != nil {
		a.setErrorBanner(err.Error())
		return
	}

	// Snapshot for the un-fullscreen -> Mind Palace upward tuck animation
	a.CaptureAnim.SubmittedSnippet = strings.TrimSpace(in.RawText)
	a.CaptureAnim.SubmittedTitle = out.Node.Title
	a.CaptureAnim.SubmittedTheme = out.Node.Theme
	a.CaptureAnim.SubmittedIconKey = domain.ResolveIdeaIconKey(out.Node)
	a.CaptureAnim.PortalPulseAt = time.Time{}
	a.CaptureAnim.LastOpenFlag = false
	a.CaptureAnim.Phase = CaptureAnimClosingPalace
	a.CaptureAnim.PhaseStart = time.Time{}

	a.CaptureModalOpen = false
	a.CaptureText = ""
	a.CaptureAudioPath = ""
	a.CaptureImagePath = ""
	a.CaptureRecallHits = nil

	mygo.Haptics.Success()
	_ = a.ReloadVault()
	a.SelectNode(out.Node.ID)
	a.TriggerGlobeCaptureRipple(out.Node)

	if strings.TrimSpace(targetNodeID) != "" {
		a.setBanner(fmt.Sprintf("已合并进《%s》，当时的原记录也留好了", out.Node.Title))
		return
	}

	if a.Deps.AI == nil || !a.Deps.AI.Configured() {
		a.setBanner(fmt.Sprintf("已记下：《%s》（填好 DeepSeek Key 后可自动整理）", out.Node.Title))
		return
	}

	ev := out.Evidence
	nodeID := out.Node.ID
	a.RunAsync("AI 正在帮你整理这条想法…", func(ctx context.Context) error {
		organized, mergedInto, err := a.Deps.Capture.OrganizeEvidenceIntoCatalog(ctx, ev, nodeID)
		if err != nil {
			return err
		}
		a.Dispatch(func() {
			a.SelectNode(organized.ID)
			a.TriggerGlobeCaptureRipple(organized)
			if mergedInto != "" {
				a.setBanner(fmt.Sprintf("已合并到《%s》，当时的原记录也留好了", organized.Title))
			} else {
				a.setBanner(fmt.Sprintf("已整理好：《%s》", organized.Title))
			}
		})
		return nil
	})
}
