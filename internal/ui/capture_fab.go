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
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/plugins/imagepicker"
	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/mobile"
	"github.com/pengstem/neromancer/internal/domain"
	"github.com/pengstem/neromancer/internal/service"
)

const (
	orbDialHoldThreshold = 150 * time.Millisecond
	orbDialMainSize      = float32(54)
	orbDialItemSize      = float32(50)
	orbDialGap           = float32(10)
)

// OrbDialState stores pure frame-persistent press, hold, and drag-scrub state
// shared by the bottom-right circular button across Dashboard (New) and Mind Palace (View Switcher).
type OrbDialState struct {
	DragDummy     ui.Point
	Pressing      bool
	WasOpen       bool
	Scrubbing     bool
	EnteredOption bool
	HoverIdx      int
	DownX         float32
	DownY         float32
	DownAt        time.Time
}

type orbDialOption struct {
	key      string
	label    string
	icon     *ui.SVG
	iconTint ui.Color
	active   bool
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

func hitOrbDialOption(localX, localY float32, count int) int {
	if count <= 0 {
		return -1
	}
	const centerX = orbDialMainSize * 0.5
	if math.Abs(float64(localX-centerX)) > 48 {
		return -1
	}
	step := orbDialItemSize + orbDialGap
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

func (a *App) renderFloatingOrbDial(c *ui.Context, pal SurfacePalette, archiveProgress float32) {
	if a.DetailSheetOpen || a.CaptureModalOpen || a.EvidenceModalOpen || a.SettingsModalOpen || a.SleepModalOpen || a.ShareModalOpen {
		return
	}

	phone := a.isPhoneLayout(c)
	_, _, bottomSafe, _ := c.SafeArea()
	t := max(float32(0), min(float32(1), archiveProgress))
	inArchive := t >= 0.5
	appleBlue := ui.Hex("#007AFF")

	if t < 0.99 {
		ui.Box(c).Absolute().Left(0).Right(0).Bottom(0).Height(68 + bottomSafe).
			Opacity(1.0 - t).PassThrough().
			LinearGradient(ui.LinearGradient{
				From:  pal.Canvas.Alpha(0.0),
				To:    pal.Canvas,
				Angle: 180,
				Oklab: true,
			})
	}

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
		} else {
			a.CaptureDialOpen = open
			a.ArchiveDialOpen = false
		}
		if !open {
			a.OrbDial.HoverIdx = -1
		}
	}

	var options []orbDialOption
	if inArchive {
		options = []orbDialOption{
			{
				key:      "arch_orb_graph",
				label:    "脉络",
				icon:     IconConstellation,
				iconTint: appleBlue,
				active:   a.ArchiveMode == ArchiveGraph,
				onSelect: func() {
					a.ArchiveMode = ArchiveGraph
				},
			},
			{
				key:      "arch_orb_cards",
				label:    "卡片",
				icon:     IconCards,
				iconTint: appleBlue,
				active:   a.ArchiveMode == ArchiveCards,
				onSelect: func() {
					a.ArchiveMode = ArchiveCards
				},
			},
		}
	} else {
		options = []orbDialOption{
			{
				key:      "dash_dial_image",
				label:    "图文",
				icon:     IconImage,
				iconTint: ColorsForTheme(domain.ThemeAmber).BadgeText,
				onSelect: func() {
					a.CaptureModalityIdx = 2
					a.CaptureModalOpen = true
				},
			},
			{
				key:      "dash_dial_text",
				label:    "文字",
				icon:     IconQuill,
				iconTint: ColorsForTheme(domain.ThemeSky).BadgeText,
				onSelect: func() {
					a.CaptureModalityIdx = 1
					a.CaptureModalOpen = true
				},
			},
			{
				key:      "dash_dial_voice",
				label:    "语音",
				icon:     IconMic,
				iconTint: ColorsForTheme(domain.ThemeMint).BadgeText,
				onSelect: func() {
					a.CaptureModalityIdx = 0
					a.CaptureModalOpen = true
				},
			},
		}
	}

	// Hold-still wakeup: if finger stays pressed >= 150ms without moving yet, pop out the circles automatically!
	d := &a.OrbDial
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

	if dialOpen {
		backdrop := ui.ButtonBase(c).Key("capture_dial_backdrop").
			Absolute().Fill().
			Background(ui.RGBA(242, 243, 246, 0.50)).
			Cursor(ui.CursorDefault).
			Label(backdropLabel)
		backdrop.OnClick(func() {
			mygo.Haptics.Selection()
			a.closeOrbDials()
		})
	}

	winW, _ := c.Size()
	dashRight := float32(18)
	archRight := float32(18)
	dashBottom := float32(20)
	archBottom := float32(46)
	if !phone {
		dashRight = 28
		leftW := float32(420)
		if a.ArchiveMode == ArchiveGraph {
			leftW = 440
		}
		if winW > leftW {
			archRight = winW - leftW + 18
		} else {
			archRight = 28
		}
		dashBottom = 26
		archBottom = 48
	}
	rightPad := dashRight + (archRight-dashRight)*t
	bottomPad := dashBottom + (archBottom-dashBottom)*t + bottomSafe

	ui.Column(c).Key("floating_orb_dial_col").
		Absolute().Right(rightPad).Bottom(bottomPad).
		AlignItems(ui.Center).Gap(orbDialGap).
		Children(func() {
			if dialOpen {
				for i, opt := range options {
					idx := i
					item := opt
					isHovered := d.Pressing && d.HoverIdx == idx

					orbBtn := ui.ButtonBase(c).Key(item.key)
					scaleTarget := float32(1.0)
					if isHovered {
						scaleTarget = 1.14
					} else if orbBtn.Pressed() {
						scaleTarget = 0.94
					}
					scale := orbBtn.Spring("orb_scale", scaleTarget)

					bg := ui.RGBA(255, 255, 255, 0.94)
					borderCol := ui.Hex("#E1E4EC")
					icCol := item.iconTint
					txtCol := pal.Ink
					shadowCol := ui.RGBA(18, 22, 33, 0.11)
					shadowBlur := float32(16)

					if isHovered {
						bg = appleBlue
						borderCol = appleBlue
						icCol = ui.Hex("#FFFFFF")
						txtCol = ui.Hex("#FFFFFF")
						shadowCol = ui.RGBA(0, 122, 255, 0.30)
						shadowBlur = 20
					} else if item.active && d.HoverIdx < 0 {
						bg = ui.Hex("#EDF4FF")
						borderCol = appleBlue.Alpha(0.45)
						icCol = appleBlue
						txtCol = appleBlue
					}

					orbBtn.Size(orbDialItemSize, orbDialItemSize).Radius(orbDialItemSize * 0.5).Center().
						Background(bg).
						Border(1, borderCol).
						Shadow(0, 5, shadowBlur, 0, shadowCol).
						Scale(scale, scale).
						Cursor(ui.CursorPointer).
						Label(item.label)
					if !isHovered {
						orbBtn.Material(glass.Glass{Tint: bg})
					}
					mobile.Ink(orbBtn)
					orbBtn.OnClick(func() {
						mygo.Haptics.Selection()
						a.closeOrbDials()
						item.onSelect()
					}).Children(func() {
						ui.Column(c).Center().Gap(1.5).Children(func() {
							ui.Icon(c, item.icon).Size(17, 17).TextColor(icCol)
							ui.Text(c, item.label).FontSize(9.5).Bold().TextColor(txtCol).SingleLine()
						})
					})
				}
			}

			sealBtn := ui.ButtonBase(c).Key("dash_capture_fab")

			dashBg := ColorVermilion
			if d.Pressing || sealBtn.Pressed() {
				dashBg = ui.Hex("#A63922")
			} else if sealBtn.Hovered() {
				dashBg = ui.Hex("#B8422A")
			}

			archBg := ui.RGBA(255, 255, 255, 0.95)
			if d.Pressing || sealBtn.Pressed() {
				archBg = ui.Hex("#EEF2F8")
			} else if sealBtn.Hovered() {
				archBg = ui.Hex("#F7F9FC")
			}

			bg := lerpColor(dashBg, archBg, t)
			borderCol := lerpColor(ui.RGBA(255, 255, 255, 0.20), ui.Hex("#DCE0EA"), t)
			shadowCol := lerpColor(ui.RGBA(200, 75, 49, 0.34), ui.RGBA(18, 22, 33, 0.13), t)

			pressTarget := float32(1.0)
			if d.Pressing || sealBtn.Pressed() {
				pressTarget = 0.94
			}
			pressScale := sealBtn.Spring("press", pressTarget)

			rotTarget := float32(0)
			if dialOpen {
				rotTarget = 45
			}
			plusAngle := sealBtn.Spring("dial_rot", rotTarget)

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

			sealBtn.Size(orbDialMainSize, orbDialMainSize).Radius(orbDialMainSize * 0.5).Center().
				Background(bg).
				Border(1, borderCol).
				Shadow(0, 8, 22, 0, shadowCol).
				Scale(pressScale, pressScale).
				Cursor(ui.CursorPointer).
				Label(sealLabel).
				Draggable(&d.DragDummy, ui.DraggableOptions{
					Axis:   ui.Horizontal | ui.Vertical,
					Bounds: &ui.Rect{},
					Rigid:  true,
				}).
				HandleInput(func(ev ui.InputEvent) bool {
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
						if !ev.Time.IsZero() {
							d.DownAt = ev.Time
						} else {
							d.DownAt = c.Now()
						}
						return true

					case ui.InputPointerMove:
						if !d.Pressing {
							return false
						}
						moved := float32(math.Hypot(float64(ev.X-d.DownX), float64(ev.Y-d.DownY)))
						now := ev.Time
						if now.IsZero() {
							now = c.Now()
						}
						held := !d.DownAt.IsZero() && now.Sub(d.DownAt) >= orbDialHoldThreshold
						if !d.Scrubbing && (moved >= 8 || held) {
							d.Scrubbing = true
							if !dialOpen {
								mygo.Haptics.Medium()
								setDialOpen(true)
								dialOpen = true
							}
						}
						if dialOpen {
							hit := hitOrbDialOption(ev.X, ev.Y, len(options))
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
							hoverIdx = hitOrbDialOption(ev.X, ev.Y, len(options))
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
						if dialOpen && hoverIdx >= 0 && hoverIdx < len(options) {
							mygo.Haptics.Selection()
							a.closeOrbDials()
							options[hoverIdx].onSelect()
							return true
						}
						if !wasScrubbing {
							mygo.Haptics.Light()
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

			if t >= 0.85 {
				sealBtn.Material(glass.Glass{Tint: archBg})
			}
			if !dialOpen {
				if inArchive {
					sealBtn.Tooltip("切换卡片 / 脉络视窗")
				} else {
					sealBtn.Tooltip("记一笔")
				}
			}
			mobile.Ink(sealBtn)

			archIcon := IconCards
			if a.ArchiveMode == ArchiveGraph {
				archIcon = IconConstellation
			}

			sealBtn.Children(func() {
				if t < 0.99 {
					dashScale := 1.0 - 0.28*t
					ui.Box(c).Absolute().Fill().Center().Opacity(1.0 - t).Scale(dashScale, dashScale).PassThrough().Children(func() {
						ui.Icon(c, IconPlus).Size(22, 22).TextColor(ui.Hex("#FFFFFF")).Rotate(plusAngle)
					})
				}
				if t > 0.01 {
					archScale := 0.72 + 0.28*t
					ui.Box(c).Absolute().Fill().Center().Opacity(t).Scale(archScale, archScale).PassThrough().Children(func() {
						if dialOpen && inArchive {
							ui.Icon(c, IconPlus).Size(21, 21).TextColor(pal.InkSecondary).Rotate(45)
						} else {
							ui.Icon(c, archIcon).Size(21, 21).TextColor(appleBlue)
						}
					})
				}
			})
		})
}

func (a *App) renderCaptureModal(c *ui.Context) {
	pal := a.Palette()

	mobile.Sheet(c, &a.CaptureModalOpen, mobile.SheetOptions{
		Detents:    []float32{0.78, 0.94},
		Background: &pal.Card,
		Label:      "记一条灵感",
	}, func() {
		ui.Scroll(c).Fill().Children(func() {
			ui.Column(c).FillWidth().Padding(14, 18, 28, 18).Gap(14).Children(func() {
				ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Text(c, "记一条灵感").FontSize(18).Bold().TextColor(pal.Ink).Grow(1)
					if GhostIconAction(c, "capture_close", IconClose, "收起", false).Clicked() {
						a.CaptureModalOpen = false
					}
				})

				a.renderCaptureModalityTabs(c, pal)

				if a.CaptureModalityIdx == 0 {
					a.renderVoiceCaptureBar(c)
				}

				if a.CaptureModalityIdx == 2 {
					a.renderImageCaptureBar(c, pal)
				}

				ui.Box(c).FillWidth().Padding(12, 14).Radius(14).
					Background(pal.SoftPanel).
					Children(func() {
						ta := ui.TextAreaBase(c, &a.CaptureText).
							Placeholder("想到什么直接记下来…").
							PlaceholderColor(pal.InkMuted).
							TextColor(pal.Ink).
							Label("灵感内容").
							Height(96).FillWidth()
						if a.CaptureModalityIdx == 1 {
							ta.AutoFocus()
						}
						if ta.Changed() {
							if hits, err := a.Deps.Organize.SearchRecall(a.CaptureText, 2); err == nil {
								a.CaptureRecallHits = hits
							}
						}
					})

				if len(a.CaptureRecallHits) > 0 && strings.TrimSpace(a.CaptureText) != "" {
					hit := a.CaptureRecallHits[0]
					hitPal := ColorsForTheme(hit.Node.Theme)
					ui.Row(c).FillWidth().Padding(10, 12).Gap(9).AlignItems(ui.Center).Radius(12).
						Background(pal.SoftPanel).Children(func() {
						ui.Icon(c, SVGForIconKey(domain.ResolveIdeaIconKey(hit.Node))).
							Size(16, 16).TextColor(hitPal.BadgeText).Shrink(0)
						ui.Textf(c, "之前记过：《%s》", hit.Node.Title).
							FontSize(12.5).Bold().TextColor(pal.Ink).Grow(1).MinWidth(0).MaxLines(1)
						if PillAction(c, "capture_merge_hit", IconCheck, "合并进去", false).Clicked() {
							a.submitQuickCapture(hit.Node.ID, false)
						}
					})
				}

				ui.Row(c).FillWidth().Wrap().Gap(10).AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
					if PillAction(c, "capture_scene_chip", IconMapPin, "记下当时场景", a.CaptureShowScene).Clicked() {
						a.CaptureShowScene = !a.CaptureShowScene
					}
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconSpark).Size(14, 14).TextColor(pal.InkSecondary)
						ui.Text(c, "AI 自动整理").FontSize(12.5).TextColor(pal.InkSecondary)
						ui.Switch(c, &a.CaptureAutoOrg).Label("AI 自动整理")
					})
				})

				if a.CaptureShowScene {
					ui.Column(c).FillWidth().Gap(8).Children(func() {
						ui.TextInput(c, &a.CaptureSceneLoc).Placeholder("地点，如：花店门口").FillWidth()
						ui.Row(c).FillWidth().Gap(8).Children(func() {
							ui.TextInput(c, &a.CaptureSceneMood).Placeholder("心情或天气").Grow(1).MinWidth(0)
							ui.TextInput(c, &a.CaptureSceneMusic).Placeholder("在听的歌").Grow(1).MinWidth(0)
						})
					})
				}

				submitBtn := ui.ButtonBase(c).Key("capture_submit").
					FillWidth().Padding(12, 16).Radius(12).Center().
					Background(pal.Ink).TextColor(pal.Card).
					Cursor(ui.CursorPointer).Label("记下来")
				mobile.Ink(submitBtn)
				submitBtn.OnClick(func() {
					a.submitQuickCapture("", a.CaptureAutoOrg)
				}).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, IconCheck).Size(15, 15).TextColor(pal.Card)
						ui.Text(c, "记下来").FontSize(14).Bold().TextColor(pal.Card).SingleLine()
					})
				})
			})
		})
	})
}

func (a *App) renderCaptureModalityTabs(c *ui.Context, pal SurfacePalette) {
	modes := []struct {
		icon  *ui.SVG
		label string
	}{
		{IconMic, "语音"},
		{IconQuill, "文字"},
		{IconImage, "图片"},
	}

	seg := ui.SegmentedBase(c, &a.CaptureModalityIdx, len(modes))
	seg.Track.FillWidth().Padding(4).Gap(4).Radius(999).Background(pal.SoftPanel).
		Label("记录方式").Children(func() {
		for i, m := range modes {
			active := i == a.CaptureModalityIdx
			s := seg.Segment(i).Padding(7, 12).Radius(999).Gap(6).
				AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).Label(m.label)
			if active {
				s.Background(pal.Card).Shadow(0, 1, 4, 0, ui.RGBA(0, 0, 0, 0.08))
			}
			mobile.Ink(s)
			fg := pal.InkSecondary
			if active {
				fg = pal.Ink
			}
			s.Children(func() {
				ui.Icon(c, m.icon).Size(14, 14).TextColor(fg)
				ui.Text(c, m.label).FontSize(13).Bold().TextColor(fg).SingleLine()
			})
		}
	}).FillEqually()
}

func (a *App) renderVoiceCaptureBar(c *ui.Context) {
	pal := a.Palette()
	phase := a.Phase(c)
	recording := a.Deps.Media != nil && a.Deps.Media.IsRecording()

	energy := float32(0.42)
	btnLabel := "开始录音"
	caption := "点一下开始说话"
	pulseColor := ColorSuccessInk
	btnBg := pal.Ink
	centerIcon := IconMic
	if recording {
		energy = 1.0
		btnLabel = "结束录音"
		caption = "正在听你说，点一下结束"
		pulseColor = ColorDangerInk
		btnBg = ColorDangerInk
		centerIcon = IconStop
	}

	ui.Column(c).FillWidth().Padding(14, 12).Gap(8).Center().Radius(18).
		Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
		ui.Box(c).Size(156, 142).Center().
			Draw(func(p *ui.Painter, r ui.Rect) {
				DrawVoicePulse(p, r, phase, energy, pulseColor)
			}).
			Children(func() {
				micBtn := ui.ButtonBase(c).Key("voice_centerpiece_btn").Size(84, 84).Radius(42).
					Center().Background(btnBg).
					Shadow(0, 8, 20, 0, ui.RGBA(20, 21, 25, 0.18)).
					Cursor(ui.CursorPointer).Label(btnLabel)
				mobile.Ink(micBtn)
				micBtn.Children(func() {
					ui.Icon(c, centerIcon).Size(24, 24).TextColor(pal.Card)
				})

				if micBtn.Clicked() {
					a.toggleVoiceRecording()
				}
			})

		ui.Text(c, caption).FontSize(12.5).Bold().TextColor(pal.InkSecondary)

		ui.Row(c).Gap(8).AlignItems(ui.Center).Center().Children(func() {
			if PillAction(c, "pick_audio_btn", IconArchive, "导入录音", false).Clicked() {
				a.pickFileAsync(true)
			}
			if a.CaptureAudioPath != "" {
				ui.Row(c).Gap(4).Padding(4, 8).Radius(999).Background(ColorSuccessSoft).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, IconCheck).Size(12, 12).TextColor(ColorSuccessInk)
					ui.Text(c, "已添加录音").FontSize(11.5).Bold().TextColor(ColorSuccessInk)
					if GhostIconAction(c, "clear_audio_btn", IconClose, "清除录音", false).
						Size(20, 20).Clicked() {
						a.CaptureAudioPath = ""
					}
				})
			}
		})
	})
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

func (a *App) renderImageCaptureBar(c *ui.Context, pal SurfacePalette) {
	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Row(c).FillWidth().Wrap().Gap(8).AlignItems(ui.Center).Children(func() {
			if PillAction(c, "capture_camera", IconCamera, "拍照", true).Clicked() {
				a.captureCameraAsync()
			}
			if PillAction(c, "capture_gallery", IconImage, "相册选图", false).Clicked() {
				a.pickFileAsync(false)
			}
		})

		if a.CaptureImagePath != "" {
			ui.Row(c).FillWidth().Padding(8, 10).Gap(10).Radius(12).AlignItems(ui.Center).
				Background(pal.SoftPanel).Border(1, pal.Border).Children(func() {
				if src := a.loadCachedImage(a.CaptureImagePath); src != nil {
					ui.Image(c, src).Size(44, 44).Radius(8).Fit(ui.Cover)
				} else {
					ui.Box(c).Size(44, 44).Radius(8).Center().Background(pal.Card).Border(1, pal.Border).Children(func() {
						ui.Icon(c, IconImage).Size(18, 18).TextColor(pal.InkSecondary)
					})
				}
				ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
					ui.Text(c, "已选好图片").FontSize(12).Bold().TextColor(ColorSuccessInk)
					ui.Text(c, filepath.Base(a.CaptureImagePath)).FontSize(11).TextColor(pal.InkMuted).SingleLine()
				})
				if GhostIconAction(c, "clear_image_btn", IconClose, "移除图片", false).Size(26, 26).Clicked() {
					a.CaptureImagePath = ""
				}
			})
		}
	})
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
		if !isAudio && (runtime.GOOS == "android" || runtime.GOOS == "ios") {
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

func (a *App) submitQuickCapture(targetNodeID string, autoOrganize bool) {
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
		AutoOrganize: autoOrganize,
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

	a.CaptureModalOpen = false
	a.CaptureText = ""
	a.CaptureAudioPath = ""
	a.CaptureImagePath = ""
	a.CaptureRecallHits = nil

	localIn := in
	localIn.AutoOrganize = false
	out, err := a.Deps.Capture.CaptureQuick(context.Background(), localIn)
	if err != nil {
		a.setErrorBanner(err.Error())
		return
	}
	mygo.Haptics.Success()
	_ = a.ReloadVault()
	a.SelectNode(out.Node.ID)
	a.TriggerGlobeCaptureRipple(out.Node)

	if !autoOrganize {
		if out.AIWarning != "" {
			a.setBanner(fmt.Sprintf("已记下：《%s》（%s）", out.Node.Title, out.AIWarning))
		} else {
			a.setBanner(fmt.Sprintf("已记下：《%s》", out.Node.Title))
		}
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
